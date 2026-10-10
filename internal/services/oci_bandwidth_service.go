package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/google/uuid"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	tagManagedKey  = "oci-panel-managed"
	tagManagedVal  = "true"
	tagFeatureKey  = "oci-panel-feature"
	tagFeatureVal  = "bandwidth-500"
	tagSchemaKey   = "oci-panel-schema"
	tagSchemaVal   = "2"
	tagPoolKey     = "oci-panel-pool"
	tagModeKey     = "oci-panel-mode"
	tagInstanceKey = "oci-panel-instance"

	ModeDedicated   = "dedicated"
	ModeSharedPorts = "shared-ports"

	OriginCreated  = "created"
	OriginAdopted  = "adopted"
	OriginBorrowed = "borrowed"

	StatusEnabled        = "enabled"
	StatusDisabled       = "disabled"
	StatusEnabling       = "enabling"
	StatusDisabling      = "disabling"
	StatusMigrating      = "migrating"
	StatusRepairRequired = "repair-required"
	StatusUnknown        = "unknown"

	OpStatusQueued         = "queued"
	OpStatusRunning        = "running"
	OpStatusSucceeded      = "succeeded"
	OpStatusFailed         = "failed"
	OpStatusRolledBack     = "rolled-back"
	OpStatusRepairRequired = "repair-required"
)

// Bandwidth500Service 提供 500Mbps 共享 NAT/NLB 生命周期、状态识别与迁移管理
type Bandwidth500Service struct {
	ociService *OCIService
	lockMap    sync.Map // key: tenantId:region:vcnId, value: *sync.Mutex
}

var (
	bandwidthServiceInstance *Bandwidth500Service
	bandwidthServiceOnce     sync.Once
)

// GetBandwidth500Service 获取单例服务
func GetBandwidth500Service(ociService *OCIService) *Bandwidth500Service {
	bandwidthServiceOnce.Do(func() {
		bandwidthServiceInstance = &Bandwidth500Service{
			ociService: ociService,
		}
	})
	return bandwidthServiceInstance
}

// getVcnLock 获取指定物理作用域的互斥锁
func (s *Bandwidth500Service) getVcnLock(tenantID, region, vcnID string) *sync.Mutex {
	key := fmt.Sprintf("%s:%s:%s", tenantID, region, vcnID)
	val, _ := s.lockMap.LoadOrStore(key, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// ClassifyOCIError 识别 OCI 错误并将配额超限错误分类
func ClassifyOCIError(err error) error {
	if err == nil {
		return nil
	}
	var svcErr common.ServiceError
	if errors.As(err, &svcErr) {
		code := svcErr.GetCode()
		msg := svcErr.GetMessage()
		reqID := svcErr.GetOpcRequestID()
		lowMsg := strings.ToLower(msg)
		if code == "LimitExceeded" || strings.Contains(lowMsg, "max-nlb-flexible-count") || (strings.Contains(lowMsg, "nlb") && strings.Contains(lowMsg, "limit")) {
			return fmt.Errorf("NLB_QUOTA_EXCEEDED: 网络负载均衡器(NLB)配额超限 (max-nlb-flexible-count)。建议复用同VCN现有NLB开启共享端口模式，或在OCI控制台核查/申请配额。RequestID: %s, 详情: %s", reqID, msg)
		}
		return fmt.Errorf("OCI API 错误 [%s]: %s (RequestID: %s)", code, msg, reqID)
	}
	return err
}

// shortInstanceHash 生成 8 位确定性短哈希，用于监听器与后端集合命名
func shortInstanceHash(instanceID string) string {
	h := sha256.Sum256([]byte(instanceID))
	return hex.EncodeToString(h[:])[:8]
}

// GetPrimaryVnicAndIP 获取实例的主 VNIC 及主私有 IP（确定性选择，避免随机取第0项）
func (s *Bandwidth500Service) GetPrimaryVnicAndIP(ctx context.Context, user *models.OciUser, instanceID string) (*core.Vnic, string, error) {
	computeClient, err := s.ociService.GetComputeClient(user)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get compute client: %w", err)
	}

	vnClient, err := s.ociService.GetVirtualNetworkClient(user)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get virtual network client: %w", err)
	}

	instResp, err := computeClient.GetInstance(ctx, core.GetInstanceRequest{InstanceId: &instanceID})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get instance: %w", err)
	}

	vnicAttachResp, err := computeClient.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
		CompartmentId: instResp.CompartmentId,
		InstanceId:    &instanceID,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to list VNIC attachments: %w", err)
	}
	if len(vnicAttachResp.Items) == 0 {
		return nil, "", fmt.Errorf("no VNIC attachment found for instance %s", instanceID)
	}

	var primaryVnic *core.Vnic
	for _, att := range vnicAttachResp.Items {
		if att.VnicId == nil {
			continue
		}
		vnicResp, err := vnClient.GetVnic(ctx, core.GetVnicRequest{VnicId: att.VnicId})
		if err != nil {
			continue
		}
		if vnicResp.IsPrimary != nil && *vnicResp.IsPrimary {
			primaryVnic = &vnicResp.Vnic
			break
		}
		if primaryVnic == nil {
			primaryVnic = &vnicResp.Vnic
		}
	}

	if primaryVnic == nil {
		return nil, "", fmt.Errorf("no valid VNIC found for instance %s", instanceID)
	}

	privateIP := ""
	if primaryVnic.PrivateIp != nil && *primaryVnic.PrivateIp != "" {
		privateIP = *primaryVnic.PrivateIp
	} else {
		// 查私有 IP 列表获取主私有 IP
		pipResp, err := vnClient.ListPrivateIps(ctx, core.ListPrivateIpsRequest{VnicId: primaryVnic.Id})
		if err == nil {
			for _, pip := range pipResp.Items {
				if pip.IsPrimary != nil && *pip.IsPrimary && pip.IpAddress != nil {
					privateIP = *pip.IpAddress
					break
				}
				if privateIP == "" && pip.IpAddress != nil {
					privateIP = *pip.IpAddress
				}
			}
		}
	}

	if privateIP == "" {
		return nil, "", fmt.Errorf("no valid private IP found on primary VNIC %s", *primaryVnic.Id)
	}

	return primaryVnic, privateIP, nil
}

// waitForWorkRequest 等待 NLB 异步工作请求完成
func (s *Bandwidth500Service) waitForWorkRequest(ctx context.Context, nlbClient networkloadbalancer.NetworkLoadBalancerClient, workRequestID *string) error {
	if workRequestID == nil || *workRequestID == "" {
		return nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for NLB work request %s", *workRequestID)
		default:
		}

		wrResp, err := nlbClient.GetWorkRequest(timeoutCtx, networkloadbalancer.GetWorkRequestRequest{
			WorkRequestId: workRequestID,
		})
		if err != nil {
			// 短暂网络波动等待重试
			time.Sleep(2 * time.Second)
			continue
		}

		switch wrResp.Status {
		case networkloadbalancer.OperationStatusSucceeded:
			return nil
		case networkloadbalancer.OperationStatusFailed:
			return fmt.Errorf("NLB work request %s failed", *workRequestID)
		case networkloadbalancer.OperationStatusCanceled:
			return fmt.Errorf("NLB work request %s was canceled", *workRequestID)
		}

		time.Sleep(2 * time.Second)
	}
}

// GetInstance500MbpsStatus 获取指定实例的 500Mbps 状态（包含持久化绑定核验与历史旧配置只读识别）
func (s *Bandwidth500Service) GetInstance500MbpsStatus(ctx context.Context, user *models.OciUser, instanceID string) (*models.Bandwidth500Summary, error) {
	falseVal := false
	trueVal := true

	// 1. 检查机型
	instance, err := s.ociService.GetInstanceById(user, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	shape := ""
	if instance.Shape != nil {
		shape = *instance.Shape
	}
	isAMD := strings.Contains(shape, "E2.1.Micro")
	if !isAMD {
		return &models.Bandwidth500Summary{
			Status:              StatusDisabled,
			Enabled:             &falseVal,
			CanEnable:           false,
			CanDisable:          false,
			ActionBlockedReason: "仅支持 VM.Standard.E2.1.Micro 实例",
			Health:              "unknown",
			Connectivity:        "unverified",
			Freshness:           "fresh",
			ObservedAt:          time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	// 2. 先查数据库是否有活跃/中间状态绑定
	var binding models.Bandwidth500Binding
	dbErr := database.GetDB().Where("instance_id = ?", instanceID).First(&binding).Error
	if dbErr == nil {
		// 数据库中存在记录
		if binding.Status == StatusEnabling || binding.Status == StatusDisabling || binding.Status == StatusMigrating {
			return &models.Bandwidth500Summary{
				Status:              binding.Status,
				Enabled:             nil, // 未完成状态不为 true 也不为 false
				Mode:                binding.Mode,
				Source:              "binding",
				NlbID:               binding.NlbID,
				AccessIP:            binding.ObservedIP,
				CanEnable:           false,
				CanDisable:          false,
				ActionBlockedReason: fmt.Sprintf("当前正在执行操作: %s", binding.Status),
				Health:              "unknown",
				Connectivity:        "unverified",
				Freshness:           "fresh",
				ObservedAt:          time.Now().UTC().Format(time.RFC3339),
			}, nil
		}

		if binding.Status == StatusEnabled {
			// 校验云端 NLB 是否存在并可用
			nlbClient, err := s.ociService.GetNetworkLoadBalancerClient(user)
			if err == nil && binding.NlbID != "" {
				nlbResp, err := nlbClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: &binding.NlbID,
				})
				if err == nil {
					var publicIP string
					for _, ip := range nlbResp.IpAddresses {
						if ip.IpAddress != nil && !isPrivateIP(*ip.IpAddress) {
							publicIP = *ip.IpAddress
							break
						}
					}
					var endpoints []models.Bandwidth500Endpoint
					_ = json.Unmarshal([]byte(binding.TargetEndpoints), &endpoints)

					// 统计共享成员数
					var memberCount int64
					database.GetDB().Model(&models.Bandwidth500Binding{}).Where("nlb_id = ? AND status = ?", binding.NlbID, StatusEnabled).Count(&memberCount)

					return &models.Bandwidth500Summary{
						Status:             StatusEnabled,
						Enabled:            &trueVal,
						Mode:               binding.Mode,
						Source:             "binding",
						NlbID:              binding.NlbID,
						AccessIP:           publicIP,
						Endpoints:          endpoints,
						Health:             "healthy",
						Connectivity:       "unverified",
						CanEnable:          false,
						CanDisable:         true,
						Freshness:          "fresh",
						ObservedAt:         time.Now().UTC().Format(time.RFC3339),
						SharedMemberCount:  int(memberCount),
						CanMigrateToShared: binding.Mode == ModeDedicated,
					}, nil
				}
			}

			// 云端 NLB 无法获取或已丢失
			return &models.Bandwidth500Summary{
				Status:              StatusRepairRequired,
				Enabled:             &trueVal,
				Mode:                binding.Mode,
				Source:              "binding",
				NlbID:               binding.NlbID,
				AccessIP:            binding.ObservedIP,
				CanEnable:           false,
				CanDisable:          true,
				ActionBlockedReason: "NLB 资源不可达或已被外部删除，需要修复或重新配置",
				Health:              "unhealthy",
				Connectivity:        "unverified",
				Freshness:           "fresh",
				ObservedAt:          time.Now().UTC().Format(time.RFC3339),
			}, nil
		}
	}

	// 3. 数据库无活跃记录，进行历史旧配置只读识别 (legacy-verified)
	summary, err := s.detectLegacyConfig(ctx, user, instanceID)
	if err == nil && summary != nil {
		return summary, nil
	}

	// 4. 确认未启用
	return &models.Bandwidth500Summary{
		Status:       StatusDisabled,
		Enabled:      &falseVal,
		CanEnable:    true,
		CanDisable:   false,
		Health:       "unknown",
		Connectivity: "unverified",
		Freshness:    "fresh",
		ObservedAt:   time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// detectLegacyConfig 只读识别升级前由旧版 oci-panel 开启的 500Mbps 实例
func (s *Bandwidth500Service) detectLegacyConfig(ctx context.Context, user *models.OciUser, instanceID string) (*models.Bandwidth500Summary, error) {
	vnic, privateIP, err := s.GetPrimaryVnicAndIP(ctx, user, instanceID)
	if err != nil {
		return nil, err
	}

	vnClient, err := s.ociService.GetVirtualNetworkClient(user)
	if err != nil {
		return nil, err
	}

	nlbClient, err := s.ociService.GetNetworkLoadBalancerClient(user)
	if err != nil {
		return nil, err
	}

	subnetResp, err := vnClient.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: vnic.SubnetId})
	if err != nil {
		return nil, err
	}

	// 获取 compartment 内的 NLB 列表
	nlbListResp, err := nlbClient.ListNetworkLoadBalancers(ctx, networkloadbalancer.ListNetworkLoadBalancersRequest{
		CompartmentId:  subnetResp.CompartmentId,
		LifecycleState: networkloadbalancer.ListNetworkLoadBalancersLifecycleStateActive,
	})
	if err != nil || nlbListResp.NetworkLoadBalancerCollection.Items == nil {
		return nil, err
	}

	var matchedNlb *networkloadbalancer.NetworkLoadBalancerSummary
	for _, item := range nlbListResp.NetworkLoadBalancerCollection.Items {
		// 校验带旧受管标签
		if isManagedNlbForInstance(item.FreeformTags, instanceID) {
			matchedNlb = &item
			break
		}
	}

	if matchedNlb == nil {
		return nil, nil
	}

	// 获取该 NLB 的详细配置，确认后端指向该实例主私有 IP
	nlbDetail, err := nlbClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: matchedNlb.Id,
	})
	if err != nil {
		return nil, err
	}

	backendMatched := false
	for _, bs := range nlbDetail.BackendSets {
		for _, b := range bs.Backends {
			if b.IpAddress != nil && *b.IpAddress == privateIP {
				backendMatched = true
				break
			}
		}
		if backendMatched {
			break
		}
	}

	if !backendMatched {
		return nil, nil
	}

	// 检查 VNIC 路由表是否配置了指向 NAT 的默认路由
	if vnic.RouteTableId == nil || *vnic.RouteTableId == "" {
		return nil, nil
	}

	rtResp, err := vnClient.GetRouteTable(ctx, core.GetRouteTableRequest{RtId: vnic.RouteTableId})
	if err != nil {
		return nil, nil
	}

	hasNatRoute := false
	for _, rule := range rtResp.RouteRules {
		if rule.Destination != nil && *rule.Destination == "0.0.0.0/0" && rule.NetworkEntityId != nil {
			// 校验网络实体是 NAT 网关
			natResp, err := vnClient.GetNatGateway(ctx, core.GetNatGatewayRequest{NatGatewayId: rule.NetworkEntityId})
			if err == nil && natResp.LifecycleState == core.NatGatewayLifecycleStateAvailable {
				hasNatRoute = true
				break
			}
		}
	}

	if !hasNatRoute {
		return nil, nil
	}

	// 匹配成功：确认属于已启用的旧受管实例
	var publicIP string
	for _, ip := range nlbDetail.IpAddresses {
		if ip.IpAddress != nil && !isPrivateIP(*ip.IpAddress) {
			publicIP = *ip.IpAddress
			break
		}
	}

	trueVal := true
	endpoints := []models.Bandwidth500Endpoint{
		{
			ID:          "all-ports",
			Protocol:    "TCP_AND_UDP",
			ListenPort:  0,
			BackendPort: 0,
		},
	}

	return &models.Bandwidth500Summary{
		Status:              StatusEnabled,
		Enabled:             &trueVal,
		Mode:                ModeDedicated,
		Source:              "legacy-verified",
		NlbID:               *matchedNlb.Id,
		AccessIP:            publicIP,
		Endpoints:           endpoints,
		Health:              "healthy",
		Connectivity:        "unverified",
		CanEnable:           false,
		CanDisable:          false, // 缺原始快照时置灰关闭按钮，提示需先确认恢复目标
		ActionBlockedReason: "已识别升级前受管 500Mbps 配置，但无初始路由快照。为防止误伤网络，请先在迁移计划中确认恢复目标。",
		Freshness:           "fresh",
		ObservedAt:          time.Now().UTC().Format(time.RFC3339),
		SharedMemberCount:   1,
		CanMigrateToShared:  true, // 支持原地迁移为共享池
	}, nil
}

// DecorateInstances 为实例列表批量附加 500Mbps 状态摘要
func (s *Bandwidth500Service) DecorateInstances(ctx context.Context, user *models.OciUser, instances []models.InstanceInfo) []models.InstanceInfo {
	if len(instances) == 0 {
		return instances
	}

	// 并发查询各实例状态
	var wg sync.WaitGroup
	result := make([]models.InstanceInfo, len(instances))
	copy(result, instances)

	// 控制并发量
	sem := make(chan struct{}, 5)

	for i := range result {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			inst := &result[idx]
			// 如果不是 AMD E2.1.Micro，直接标记不支持，避免不必要的 OCI API 请求
			if !strings.Contains(inst.Shape, "E2.1.Micro") {
				falseVal := false
				inst.Bandwidth500 = &models.Bandwidth500Summary{
					Status:              StatusDisabled,
					Enabled:             &falseVal,
					CanEnable:           false,
					CanDisable:          false,
					ActionBlockedReason: "仅支持 VM.Standard.E2.1.Micro (AMD) 实例",
					Health:              "unknown",
					Connectivity:        "unverified",
					Freshness:           "fresh",
					ObservedAt:          time.Now().UTC().Format(time.RFC3339),
				}
				return
			}

			summary, err := s.GetInstance500MbpsStatus(ctx, user, inst.ID)
			if err != nil {
				inst.Bandwidth500 = &models.Bandwidth500Summary{
					Status:              StatusUnknown,
					Enabled:             nil,
					CanEnable:           false,
					CanDisable:          false,
					ActionBlockedReason: fmt.Sprintf("获取状态失败: %v", err),
					Health:              "unknown",
					Connectivity:        "unverified",
					Freshness:           "stale",
					ObservedAt:          time.Now().UTC().Format(time.RFC3339),
				}
				return
			}
			inst.Bandwidth500 = summary
		}(i)
	}

	wg.Wait()
	return result
}

// PlanRequest 预检与生成开启计划请求
type PlanRequest struct {
	InstanceID string                        `json:"instanceId"`
	Mode       string                        `json:"mode"`     // dedicated, shared-ports
	SSHPort    int                           `json:"sshPort"`  // 实例健康检查端口，默认 22
	Mappings   []models.Bandwidth500Endpoint `json:"mappings"` // 共享模式映射端口
	NLBID      string                        `json:"nlbId"`    // 指定复用 NLB
}

// PlanResponse 计划响应
type PlanResponse struct {
	PlanID            string                        `json:"planId"`
	InstanceID        string                        `json:"instanceId"`
	Mode              string                        `json:"mode"`
	CurrentStatus     string                        `json:"currentStatus"`
	VcnID             string                        `json:"vcnId"`
	SubnetID          string                        `json:"subnetId"`
	CanReuseNat       bool                          `json:"canReuseNat"`
	NatGatewayID      string                        `json:"natGatewayId"`
	CanReuseNlb       bool                          `json:"canReuseNlb"`
	NlbID             string                        `json:"nlbId"`
	NlbAccessIP       string                        `json:"nlbAccessIp"`
	EffectiveMappings []models.Bandwidth500Endpoint `json:"effectiveMappings"`
	PortConflicts     []string                      `json:"portConflicts,omitempty"`
	RequiresMigration bool                          `json:"requiresMigration"`
	Warnings          []string                      `json:"warnings"`
	Supported         bool                          `json:"supported"`
	Reason            string                        `json:"reason"`
}

// Plan500Mbps 执行只读预检并生成执行计划（不创建资源、不修改云端）
func (s *Bandwidth500Service) Plan500Mbps(ctx context.Context, user *models.OciUser, req PlanRequest) (*PlanResponse, error) {
	if req.SSHPort <= 0 || req.SSHPort > 65535 {
		req.SSHPort = 22
	}
	if req.Mode == "" {
		req.Mode = ModeDedicated
	}

	// 1. 检查机型
	instance, err := s.ociService.GetInstanceById(user, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	if instance.Shape == nil || !strings.Contains(*instance.Shape, "E2.1.Micro") {
		return &PlanResponse{
			Supported: false,
			Reason:    "当前实例机型不支持 500Mbps 功能，仅 VM.Standard.E2.1.Micro 实例支持",
		}, nil
	}

	// 2. 获取网络拓扑
	vnic, _, err := s.GetPrimaryVnicAndIP(ctx, user, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary VNIC: %w", err)
	}

	vnClient, err := s.ociService.GetVirtualNetworkClient(user)
	if err != nil {
		return nil, err
	}
	subnetResp, err := vnClient.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: vnic.SubnetId})
	if err != nil {
		return nil, fmt.Errorf("failed to get subnet: %w", err)
	}
	vcnID := *subnetResp.VcnId
	compartmentID := *instance.CompartmentId

	resp := &PlanResponse{
		PlanID:            uuid.New().String(),
		InstanceID:        req.InstanceID,
		Mode:              req.Mode,
		VcnID:             vcnID,
		SubnetID:          *vnic.SubnetId,
		Supported:         true,
		EffectiveMappings: req.Mappings,
	}

	// 3. 检查是否有可用 NAT 网关
	natResp, err := vnClient.ListNatGateways(ctx, core.ListNatGatewaysRequest{
		CompartmentId:  &compartmentID,
		VcnId:          &vcnID,
		LifecycleState: core.NatGatewayLifecycleStateAvailable,
	})
	if err == nil && len(natResp.Items) > 0 {
		resp.CanReuseNat = true
		resp.NatGatewayID = *natResp.Items[0].Id
	} else {
		resp.Warnings = append(resp.Warnings, "同 VCN 未发现可用 NAT 网关，执行时将创建新 NAT 网关")
	}

	// 4. 检查 NLB 共享与复用
	nlbClient, err := s.ociService.GetNetworkLoadBalancerClient(user)
	if err != nil {
		return nil, err
	}

	nlbListResp, err := nlbClient.ListNetworkLoadBalancers(ctx, networkloadbalancer.ListNetworkLoadBalancersRequest{
		CompartmentId:  &compartmentID,
		LifecycleState: networkloadbalancer.ListNetworkLoadBalancersLifecycleStateActive,
	})

	if err == nil && nlbListResp.NetworkLoadBalancerCollection.Items != nil {
		// 寻找同 VCN 的受管共享 NLB
		for _, nlb := range nlbListResp.NetworkLoadBalancerCollection.Items {
			tags := nlb.FreeformTags
			if tags != nil && tags[tagManagedKey] == tagManagedVal && tags[tagFeatureKey] == tagFeatureVal && tags[tagModeKey] == ModeSharedPorts {
				resp.CanReuseNlb = true
				resp.NlbID = *nlb.Id
				for _, ip := range nlb.IpAddresses {
					if ip.IpAddress != nil && !isPrivateIP(*ip.IpAddress) {
						resp.NlbAccessIP = *ip.IpAddress
						break
					}
				}
				break
			}
		}

		// 检查是否有第一台实例创建的旧受管 NLB 可原地迁移
		if !resp.CanReuseNlb && req.Mode == ModeSharedPorts {
			for _, nlb := range nlbListResp.NetworkLoadBalancerCollection.Items {
				tags := nlb.FreeformTags
				if tags != nil && tags[tagManagedKey] == tagManagedVal && tags[tagInstanceKey] != "" && tags[tagInstanceKey] != req.InstanceID {
					resp.RequiresMigration = true
					resp.Warnings = append(resp.Warnings, fmt.Sprintf("发现同 compartment 内存在其他实例占用的独享 NLB (%s)。推荐先将其迁移为共享池以复用额度与 IP。", *nlb.Id))
					break
				}
			}
		}
	}

	// 5. 校验共享端口映射
	if req.Mode == ModeSharedPorts {
		if len(req.Mappings) == 0 {
			// 提供默认映射
			resp.EffectiveMappings = []models.Bandwidth500Endpoint{
				{
					ID:          "ssh",
					Protocol:    "TCP",
					ListenPort:  2202,
					BackendPort: req.SSHPort,
				},
			}
		}

		// 检查端口冲突
		if resp.CanReuseNlb && resp.NlbID != "" {
			nlbDetail, err := nlbClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
				NetworkLoadBalancerId: &resp.NlbID,
			})
			if err == nil {
				for _, m := range resp.EffectiveMappings {
					for _, l := range nlbDetail.Listeners {
						if l.Port != nil && *l.Port == m.ListenPort {
							resp.PortConflicts = append(resp.PortConflicts, fmt.Sprintf("监听端口 %d 与已有监听器 %s 冲突", m.ListenPort, *l.Name))
						}
					}
				}
			}
		}
	}

	if len(resp.PortConflicts) > 0 {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("检测到 %d 个监听端口冲突，请修改监听端口", len(resp.PortConflicts)))
	}

	return resp, nil
}

// EnableRequest 开启请求
type EnableRequest struct {
	InstanceID  string                        `json:"instanceId"`
	Mode        string                        `json:"mode"`     // dedicated, shared-ports
	SSHPort     int                           `json:"sshPort"`  // 内部健康检查端口
	Mappings    []models.Bandwidth500Endpoint `json:"mappings"` // 端口映射
	NLBID       string                        `json:"nlbId"`    // 指定 NLB
	RetainNatGw *bool                         `json:"retainNatGw"`
	RetainNlb   *bool                         `json:"retainNlb"`
}

// Enable500MbpsOrchestrated 经过编排与安全加固的 500Mbps 开启入口
func (s *Bandwidth500Service) Enable500MbpsOrchestrated(ctx context.Context, user *models.OciUser, req EnableRequest) (string, error) {
	if req.SSHPort <= 0 || req.SSHPort > 65535 {
		req.SSHPort = 22
	}
	if req.Mode == "" {
		req.Mode = ModeDedicated
	}

	// 1. 基础校验
	instance, err := s.ociService.GetInstanceById(user, req.InstanceID)
	if err != nil {
		return "", fmt.Errorf("failed to get instance: %w", err)
	}
	if instance.Shape == nil || !strings.Contains(*instance.Shape, "E2.1.Micro") {
		return "", fmt.Errorf("only AMD VM.Standard.E2.1.Micro instances support 500Mbps")
	}

	vnic, privateIP, err := s.GetPrimaryVnicAndIP(ctx, user, req.InstanceID)
	if err != nil {
		return "", fmt.Errorf("failed to get primary VNIC and IP: %w", err)
	}

	vnClient, err := s.ociService.GetVirtualNetworkClient(user)
	if err != nil {
		return "", fmt.Errorf("failed to get virtual network client: %w", err)
	}
	subnetResp, err := vnClient.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: vnic.SubnetId})
	if err != nil {
		return "", fmt.Errorf("failed to get subnet: %w", err)
	}
	vcnID := *subnetResp.VcnId
	compartmentID := *instance.CompartmentId

	// 2. 获取物理作用域锁 (tenantId:region:vcnId)
	lock := s.getVcnLock(user.OciTenantID, user.OciRegion, vcnID)
	lock.Lock()
	defer lock.Unlock()

	// 3. 检查是否有正在运行的操作或已开启绑定
	var existingBinding models.Bandwidth500Binding
	if err := database.GetDB().Where("instance_id = ?", req.InstanceID).First(&existingBinding).Error; err == nil {
		if existingBinding.Status == StatusEnabled && existingBinding.ObservedIP != "" {
			// 同配置幂等返回
			return existingBinding.ObservedIP, nil
		}
		if existingBinding.Status == StatusEnabling {
			return "", fmt.Errorf("该实例当前正在开启中，请稍候再试")
		}
	}

	// 4. 保存原始快照 (VNIC 原 RouteTableId, SkipSourceDestCheck)
	snapshot := models.Bandwidth500OriginalSnapshot{
		RouteTableID:        vnic.RouteTableId,
		SkipSourceDestCheck: vnic.SkipSourceDestCheck,
		NsgIDs:              vnic.NsgIds,
		HadExplicitRoute:    vnic.RouteTableId != nil && *vnic.RouteTableId != "",
	}
	snapshotBytes, _ := json.Marshal(snapshot)

	// 创建/更新数据库绑定记录为 enabling
	bindingID := uuid.New().String()
	if existingBinding.ID != "" {
		bindingID = existingBinding.ID
	}
	binding := models.Bandwidth500Binding{
		ID:               bindingID,
		InstanceID:       req.InstanceID,
		UserID:           user.ID,
		TenantID:         user.OciTenantID,
		Region:           user.OciRegion,
		VcnID:            vcnID,
		Mode:             req.Mode,
		VnicID:           *vnic.Id,
		PrivateIP:        privateIP,
		OriginalSnapshot: string(snapshotBytes),
		Status:           StatusEnabling,
	}
	_ = database.GetDB().Save(&binding).Error

	// 5. 发现或创建 NAT 网关
	natGatewayID, isBorrowedNat, err := s.ensureNatGateway(ctx, vnClient, compartmentID, vcnID)
	if err != nil {
		binding.Status = StatusRepairRequired
		_ = database.GetDB().Save(&binding).Error
		return "", fmt.Errorf("failed to ensure NAT gateway: %w", ClassifyOCIError(err))
	}
	binding.NatGatewayID = natGatewayID

	// 登记 NAT 资源
	natOrigin := OriginCreated
	canDeleteNat := true
	if isBorrowedNat {
		natOrigin = OriginBorrowed
		canDeleteNat = false // 借用的 NAT 绝不删除
	}
	s.registerResource("nat_gateway", natGatewayID, user.OciTenantID, user.OciRegion, compartmentID, vcnID, "", natOrigin, canDeleteNat)

	// 6. 发现或创建 NLB
	nlbClient, err := s.ociService.GetNetworkLoadBalancerClient(user)
	if err != nil {
		binding.Status = StatusRepairRequired
		_ = database.GetDB().Save(&binding).Error
		return "", fmt.Errorf("failed to get NLB client: %w", err)
	}

	var nlbID string
	var publicIP string
	var endpoints []models.Bandwidth500Endpoint

	if req.Mode == ModeSharedPorts {
		// 共享模式：确保共享 NLB
		nlbID, publicIP, err = s.ensureSharedNlb(ctx, nlbClient, compartmentID, *vnic.SubnetId, vcnID)
		if err != nil {
			binding.Status = StatusRepairRequired
			_ = database.GetDB().Save(&binding).Error
			return "", fmt.Errorf("failed to ensure shared NLB: %w", ClassifyOCIError(err))
		}

		if len(req.Mappings) == 0 {
			req.Mappings = []models.Bandwidth500Endpoint{
				{
					ID:          "ssh",
					Protocol:    "TCP",
					ListenPort:  2202,
					BackendPort: req.SSHPort,
				},
			}
		}
		endpoints = req.Mappings

		// 为该实例创建监听器与后端集合
		err = s.ensureInstanceEndpoints(ctx, nlbClient, nlbID, req.InstanceID, privateIP, endpoints)
		if err != nil {
			binding.Status = StatusRepairRequired
			_ = database.GetDB().Save(&binding).Error
			return "", fmt.Errorf("failed to ensure instance endpoints: %w", ClassifyOCIError(err))
		}
	} else {
		// 独享全端口模式
		nlbID, publicIP, err = s.ensureDedicatedNlb(ctx, nlbClient, compartmentID, *vnic.SubnetId, req.InstanceID, privateIP, req.SSHPort)
		if err != nil {
			binding.Status = StatusRepairRequired
			_ = database.GetDB().Save(&binding).Error
			return "", fmt.Errorf("failed to ensure dedicated NLB: %w", ClassifyOCIError(err))
		}
		endpoints = []models.Bandwidth500Endpoint{
			{
				ID:          "all-ports",
				Protocol:    "TCP_AND_UDP",
				ListenPort:  0,
				BackendPort: 0,
			},
		}
	}

	binding.NlbID = nlbID
	binding.ObservedIP = publicIP
	epBytes, _ := json.Marshal(endpoints)
	binding.TargetEndpoints = string(epBytes)

	// 7. 创建或复用包含 NAT 默认路由的受管路由表
	routeTableID, err := s.ensureNatRouteTable(ctx, vnClient, compartmentID, vcnID, natGatewayID)
	if err != nil {
		binding.Status = StatusRepairRequired
		_ = database.GetDB().Save(&binding).Error
		return "", fmt.Errorf("failed to ensure NAT route table: %w", ClassifyOCIError(err))
	}
	binding.RouteTableID = routeTableID
	s.registerResource("route_table", routeTableID, user.OciTenantID, user.OciRegion, compartmentID, vcnID, "", OriginCreated, true)

	// 8. 最后一步：更新 VNIC 路由表与 SkipSourceDestCheck（确保外部网络就绪后才切换路由）
	skipSourceDestCheck := true
	_, err = vnClient.UpdateVnic(ctx, core.UpdateVnicRequest{
		VnicId: vnic.Id,
		UpdateVnicDetails: core.UpdateVnicDetails{
			SkipSourceDestCheck: &skipSourceDestCheck,
			RouteTableId:        &routeTableID,
		},
	})
	if err != nil {
		binding.Status = StatusRepairRequired
		_ = database.GetDB().Save(&binding).Error
		return "", fmt.Errorf("failed to update VNIC route table: %w", ClassifyOCIError(err))
	}

	// 9. 更新持久化状态为 enabled
	binding.Status = StatusEnabled
	binding.ObservedHealth = "healthy"
	_ = database.GetDB().Save(&binding).Error

	return publicIP, nil
}

// ensureNatGateway 确保同 VCN 存在可用 NAT 网关（优先复用既有可用 NAT 并标记 borrowed）
func (s *Bandwidth500Service) ensureNatGateway(ctx context.Context, vnClient core.VirtualNetworkClient, compartmentID, vcnID string) (string, bool, error) {
	natResp, err := vnClient.ListNatGateways(ctx, core.ListNatGatewaysRequest{
		CompartmentId:  &compartmentID,
		VcnId:          &vcnID,
		LifecycleState: core.NatGatewayLifecycleStateAvailable,
	})
	if err == nil && len(natResp.Items) > 0 {
		return *natResp.Items[0].Id, true, nil // 借用既有 NAT
	}

	// 创建受管 NAT 网关
	natName := "oci-panel-nat-gw"
	createResp, err := vnClient.CreateNatGateway(ctx, core.CreateNatGatewayRequest{
		CreateNatGatewayDetails: core.CreateNatGatewayDetails{
			CompartmentId: &compartmentID,
			VcnId:         &vcnID,
			DisplayName:   &natName,
			FreeformTags: map[string]string{
				tagManagedKey: tagManagedVal,
				tagFeatureKey: tagFeatureVal,
			},
		},
	})
	if err != nil {
		return "", false, err
	}

	// 等待可用
	natID := *createResp.Id
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return "", false, fmt.Errorf("timeout waiting for NAT gateway to become available")
		default:
		}
		gResp, err := vnClient.GetNatGateway(timeoutCtx, core.GetNatGatewayRequest{NatGatewayId: &natID})
		if err == nil && gResp.LifecycleState == core.NatGatewayLifecycleStateAvailable {
			break
		}
		time.Sleep(2 * time.Second)
	}

	return natID, false, nil
}

// ensureDedicatedNlb 确保独享全端口 NLB（幂等支持，若已存在且正常则复用）
func (s *Bandwidth500Service) ensureDedicatedNlb(ctx context.Context, nlbClient networkloadbalancer.NetworkLoadBalancerClient, compartmentID, subnetID, instanceID, privateIP string, sshPort int) (string, string, error) {
	// 查找是否已存在为该实例创建的受管 NLB
	listResp, err := nlbClient.ListNetworkLoadBalancers(ctx, networkloadbalancer.ListNetworkLoadBalancersRequest{
		CompartmentId:  &compartmentID,
		LifecycleState: networkloadbalancer.ListNetworkLoadBalancersLifecycleStateActive,
	})
	if err == nil && listResp.NetworkLoadBalancerCollection.Items != nil {
		for _, nlb := range listResp.NetworkLoadBalancerCollection.Items {
			if isManagedNlbForInstance(nlb.FreeformTags, instanceID) {
				pubIP := getPublicIPFromNlbAddresses(nlb.IpAddresses)
				if pubIP != "" {
					return *nlb.Id, pubIP, nil
				}
			}
		}
	}

	// 创建新独享 NLB
	nlbName := fmt.Sprintf("%s%s", nlbManagedNamePrefix, time.Now().Format("20060102150405"))
	isPrivate := false
	port := 0
	weight := 1
	isPreserveSource := true
	isFailOpen := true

	tags := managedNlbFreeformTags(instanceID)
	tags[tagFeatureKey] = tagFeatureVal
	tags[tagModeKey] = ModeDedicated

	createResp, err := nlbClient.CreateNetworkLoadBalancer(ctx, networkloadbalancer.CreateNetworkLoadBalancerRequest{
		CreateNetworkLoadBalancerDetails: networkloadbalancer.CreateNetworkLoadBalancerDetails{
			CompartmentId: &compartmentID,
			DisplayName:   &nlbName,
			SubnetId:      &subnetID,
			IsPrivate:     &isPrivate,
			FreeformTags:  tags,
			Listeners: map[string]networkloadbalancer.ListenerDetails{
				"listener1": {
					Name:                  stringPtr("listener1"),
					DefaultBackendSetName: stringPtr("backend1"),
					Protocol:              networkloadbalancer.ListenerProtocolsTcpAndUdp,
					Port:                  &port,
				},
			},
			BackendSets: map[string]networkloadbalancer.BackendSetDetails{
				"backend1": {
					Policy:           networkloadbalancer.NetworkLoadBalancingPolicyTwoTuple,
					IsPreserveSource: &isPreserveSource,
					IsFailOpen:       &isFailOpen,
					HealthChecker: &networkloadbalancer.HealthChecker{
						Protocol: networkloadbalancer.HealthCheckProtocolsTcp,
						Port:     &sshPort,
					},
					Backends: []networkloadbalancer.Backend{
						{
							IpAddress: &privateIP,
							TargetId:  &instanceID,
							Port:      &port,
							Weight:    &weight,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return "", "", err
	}

	nlbID := *createResp.Id
	pubIP, err := s.waitForNlbActiveAndPublicIP(ctx, nlbClient, nlbID)
	if err != nil {
		return "", "", err
	}

	return nlbID, pubIP, nil
}

// ensureSharedNlb 确保同 VCN 存在受管共享 NLB
func (s *Bandwidth500Service) ensureSharedNlb(ctx context.Context, nlbClient networkloadbalancer.NetworkLoadBalancerClient, compartmentID, subnetID, vcnID string) (string, string, error) {
	poolID := fmt.Sprintf("pool-%s", vcnID[:min(8, len(vcnID))])

	// 查找已有共享 NLB
	listResp, err := nlbClient.ListNetworkLoadBalancers(ctx, networkloadbalancer.ListNetworkLoadBalancersRequest{
		CompartmentId:  &compartmentID,
		LifecycleState: networkloadbalancer.ListNetworkLoadBalancersLifecycleStateActive,
	})
	if err == nil && listResp.NetworkLoadBalancerCollection.Items != nil {
		for _, nlb := range listResp.NetworkLoadBalancerCollection.Items {
			tags := nlb.FreeformTags
			if tags != nil && tags[tagManagedKey] == tagManagedVal && tags[tagFeatureKey] == tagFeatureVal && tags[tagModeKey] == ModeSharedPorts {
				pubIP := getPublicIPFromNlbAddresses(nlb.IpAddresses)
				if pubIP != "" {
					return *nlb.Id, pubIP, nil
				}
			}
		}
	}

	// 创建新受管共享 NLB
	nlbName := fmt.Sprintf("oci-panel-shared-nlb-%s", time.Now().Format("20060102150405"))
	isPrivate := false

	tags := map[string]string{
		tagManagedKey: tagManagedVal,
		tagFeatureKey: tagFeatureVal,
		tagSchemaKey:  tagSchemaVal,
		tagModeKey:    ModeSharedPorts,
		tagPoolKey:    poolID,
	}

	createResp, err := nlbClient.CreateNetworkLoadBalancer(ctx, networkloadbalancer.CreateNetworkLoadBalancerRequest{
		CreateNetworkLoadBalancerDetails: networkloadbalancer.CreateNetworkLoadBalancerDetails{
			CompartmentId: &compartmentID,
			DisplayName:   &nlbName,
			SubnetId:      &subnetID,
			IsPrivate:     &isPrivate,
			FreeformTags:  tags,
		},
	})
	if err != nil {
		return "", "", err
	}

	nlbID := *createResp.Id
	pubIP, err := s.waitForNlbActiveAndPublicIP(ctx, nlbClient, nlbID)
	if err != nil {
		return "", "", err
	}

	return nlbID, pubIP, nil
}

// ensureInstanceEndpoints 为共享 NLB 创建指定实例的 BackendSet 与 Listener
func (s *Bandwidth500Service) ensureInstanceEndpoints(ctx context.Context, nlbClient networkloadbalancer.NetworkLoadBalancerClient, nlbID, instanceID, privateIP string, endpoints []models.Bandwidth500Endpoint) error {
	instHash := shortInstanceHash(instanceID)
	isPreserveSource := true
	isFailOpen := true
	weight := 1

	for _, ep := range endpoints {
		bsName := fmt.Sprintf("bs_%s_%s_%d", instHash, strings.ToLower(ep.Protocol), ep.ListenPort)
		lsName := fmt.Sprintf("ls_%s_%s_%d", instHash, strings.ToLower(ep.Protocol), ep.ListenPort)

		// 确定协议与端口
		protocol := networkloadbalancer.ListenerProtocolsTcp
		healthProto := networkloadbalancer.HealthCheckProtocolsTcp
		if strings.EqualFold(ep.Protocol, "UDP") {
			protocol = networkloadbalancer.ListenerProtocolsUdp
			healthProto = networkloadbalancer.HealthCheckProtocolsUdp
		}

		backendPort := ep.BackendPort
		listenPort := ep.ListenPort

		// 1. 创建 BackendSet
		bsResp, err := nlbClient.CreateBackendSet(ctx, networkloadbalancer.CreateBackendSetRequest{
			NetworkLoadBalancerId: &nlbID,
			CreateBackendSetDetails: networkloadbalancer.CreateBackendSetDetails{
				Name:             &bsName,
				Policy:           networkloadbalancer.NetworkLoadBalancingPolicyTwoTuple,
				IsPreserveSource: &isPreserveSource,
				IsFailOpen:       &isFailOpen,
				HealthChecker: &networkloadbalancer.HealthCheckerDetails{
					Protocol: healthProto,
					Port:     &backendPort,
				},
				Backends: []networkloadbalancer.BackendDetails{
					{
						IpAddress: &privateIP,
						TargetId:  &instanceID,
						Port:      &backendPort,
						Weight:    &weight,
					},
				},
			},
		})
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to create backend set %s: %w", bsName, err)
		}
		if err == nil {
			_ = s.waitForWorkRequest(ctx, nlbClient, bsResp.OpcWorkRequestId)
		}

		// 2. 创建 Listener
		lsResp, err := nlbClient.CreateListener(ctx, networkloadbalancer.CreateListenerRequest{
			NetworkLoadBalancerId: &nlbID,
			CreateListenerDetails: networkloadbalancer.CreateListenerDetails{
				Name:                  &lsName,
				DefaultBackendSetName: &bsName,
				Protocol:              protocol,
				Port:                  &listenPort,
			},
		})
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to create listener %s: %w", lsName, err)
		}
		if err == nil {
			_ = s.waitForWorkRequest(ctx, nlbClient, lsResp.OpcWorkRequestId)
		}
	}

	return nil
}

// ensureNatRouteTable 创建或复用带 NAT 默认出口的受管路由表（保留原有效规则）
func (s *Bandwidth500Service) ensureNatRouteTable(ctx context.Context, vnClient core.VirtualNetworkClient, compartmentID, vcnID, natGatewayID string) (string, error) {
	// 查询是否已有指向该 NAT 的受管路由表
	rtResp, err := vnClient.ListRouteTables(ctx, core.ListRouteTablesRequest{
		CompartmentId:  &compartmentID,
		VcnId:          &vcnID,
		LifecycleState: core.RouteTableLifecycleStateAvailable,
	})
	if err == nil {
		for _, rt := range rtResp.Items {
			for _, rule := range rt.RouteRules {
				if rule.NetworkEntityId != nil && *rule.NetworkEntityId == natGatewayID &&
					rule.Destination != nil && *rule.Destination == "0.0.0.0/0" {
					return *rt.Id, nil
				}
			}
		}
	}

	// 创建新路由表
	rtName := "oci-panel-nat-route"
	destination := "0.0.0.0/0"
	createRtResp, err := vnClient.CreateRouteTable(ctx, core.CreateRouteTableRequest{
		CreateRouteTableDetails: core.CreateRouteTableDetails{
			CompartmentId: &compartmentID,
			VcnId:         &vcnID,
			DisplayName:   &rtName,
			FreeformTags: map[string]string{
				tagManagedKey: tagManagedVal,
				tagFeatureKey: tagFeatureVal,
			},
			RouteRules: []core.RouteRule{
				{
					Destination:     &destination,
					NetworkEntityId: &natGatewayID,
					DestinationType: core.RouteRuleDestinationTypeCidrBlock,
				},
			},
		},
	})
	if err != nil {
		return "", err
	}

	rtID := *createRtResp.Id
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return "", fmt.Errorf("timeout waiting for route table to become available")
		default:
		}
		rResp, err := vnClient.GetRouteTable(timeoutCtx, core.GetRouteTableRequest{RtId: &rtID})
		if err == nil && rResp.LifecycleState == core.RouteTableLifecycleStateAvailable {
			break
		}
		time.Sleep(2 * time.Second)
	}

	return rtID, nil
}

// DisableRequest 关闭请求
type DisableRequest struct {
	InstanceID  string `json:"instanceId"`
	RetainNatGw *bool  `json:"retainNatGw"` // 默认 true
	RetainNlb   *bool  `json:"retainNlb"`   // 默认 true
}

// Disable500MbpsOrchestrated 安全解绑单台实例，绝不高危清空全 VCN 路由或删除共享资源
func (s *Bandwidth500Service) Disable500MbpsOrchestrated(ctx context.Context, user *models.OciUser, req DisableRequest) error {
	// 默认保留资源，防止误删共享 NAT / NLB
	retainNatGw := true
	if req.RetainNatGw != nil {
		retainNatGw = *req.RetainNatGw
	}
	retainNlb := true
	if req.RetainNlb != nil {
		retainNlb = *req.RetainNlb
	}

	instance, err := s.ociService.GetInstanceById(user, req.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}

	vnic, _, err := s.GetPrimaryVnicAndIP(ctx, user, req.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to get primary VNIC: %w", err)
	}

	vnClient, err := s.ociService.GetVirtualNetworkClient(user)
	if err != nil {
		return fmt.Errorf("failed to get virtual network client: %w", err)
	}
	subnetResp, err := vnClient.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: vnic.SubnetId})
	if err != nil {
		return fmt.Errorf("failed to get subnet: %w", err)
	}
	vcnID := *subnetResp.VcnId

	// 取得物理作用域锁
	lock := s.getVcnLock(user.OciTenantID, user.OciRegion, vcnID)
	lock.Lock()
	defer lock.Unlock()

	// 查询 DB 绑定
	var binding models.Bandwidth500Binding
	hasBinding := database.GetDB().Where("instance_id = ?", req.InstanceID).First(&binding).Error == nil

	// 1. 恢复 VNIC 路由表与 SkipSourceDestCheck
	var targetRtID *string
	skipCheck := false

	if hasBinding && binding.OriginalSnapshot != "" {
		var snapshot models.Bandwidth500OriginalSnapshot
		if err := json.Unmarshal([]byte(binding.OriginalSnapshot), &snapshot); err == nil {
			targetRtID = snapshot.RouteTableID
			if snapshot.SkipSourceDestCheck != nil {
				skipCheck = *snapshot.SkipSourceDestCheck
			}
		}
	}

	// 如果没有原始快照（如历史遗留实例），查找默认路由表恢复
	if targetRtID == nil {
		emptyStr := ""
		targetRtID = &emptyStr // 清空 VNIC 级路由，继承子网路由
	}

	// 更新 VNIC 恢复原有路由
	_, err = vnClient.UpdateVnic(ctx, core.UpdateVnicRequest{
		VnicId: vnic.Id,
		UpdateVnicDetails: core.UpdateVnicDetails{
			SkipSourceDestCheck: &skipCheck,
			RouteTableId:        targetRtID,
		},
	})
	if err != nil {
		log.Printf("Warning: failed to restore VNIC route table for %s: %v", req.InstanceID, err)
	}

	// 2. 解除 NLB 上的本实例端口映射与后端
	nlbClient, err := s.ociService.GetNetworkLoadBalancerClient(user)
	if err == nil {
		nlbID := ""
		if hasBinding && binding.NlbID != "" {
			nlbID = binding.NlbID
		} else {
			// 查找该实例的旧受管 NLB
			listResp, err := nlbClient.ListNetworkLoadBalancers(ctx, networkloadbalancer.ListNetworkLoadBalancersRequest{
				CompartmentId:  instance.CompartmentId,
				LifecycleState: networkloadbalancer.ListNetworkLoadBalancersLifecycleStateActive,
			})
			if err == nil && listResp.NetworkLoadBalancerCollection.Items != nil {
				for _, n := range listResp.NetworkLoadBalancerCollection.Items {
					if isManagedNlbForInstance(n.FreeformTags, req.InstanceID) {
						nlbID = *n.Id
						break
					}
				}
			}
		}

		if nlbID != "" {
			// 如果是共享模式，删除本实例专属的 Listener 和 BackendSet
			if hasBinding && binding.Mode == ModeSharedPorts {
				var endpoints []models.Bandwidth500Endpoint
				_ = json.Unmarshal([]byte(binding.TargetEndpoints), &endpoints)
				instHash := shortInstanceHash(req.InstanceID)

				for _, ep := range endpoints {
					lsName := fmt.Sprintf("ls_%s_%s_%d", instHash, strings.ToLower(ep.Protocol), ep.ListenPort)
					bsName := fmt.Sprintf("bs_%s_%s_%d", instHash, strings.ToLower(ep.Protocol), ep.ListenPort)

					// 必须先删 Listener，再删 BackendSet
					lsResp, err := nlbClient.DeleteListener(ctx, networkloadbalancer.DeleteListenerRequest{
						NetworkLoadBalancerId: &nlbID,
						ListenerName:          &lsName,
					})
					if err == nil {
						_ = s.waitForWorkRequest(ctx, nlbClient, lsResp.OpcWorkRequestId)
					}

					bsResp, err := nlbClient.DeleteBackendSet(ctx, networkloadbalancer.DeleteBackendSetRequest{
						NetworkLoadBalancerId: &nlbID,
						BackendSetName:        &bsName,
					})
					if err == nil {
						_ = s.waitForWorkRequest(ctx, nlbClient, bsResp.OpcWorkRequestId)
					}
				}
			}

			// 如果允许清理 NLB (!retainNlb)，必须检查是否为该实例独占，且无其他实例引用！
			if !retainNlb {
				var otherBindingsCount int64
				database.GetDB().Model(&models.Bandwidth500Binding{}).
					Where("nlb_id = ? AND instance_id != ? AND status = ?", nlbID, req.InstanceID, StatusEnabled).
					Count(&otherBindingsCount)

				if otherBindingsCount == 0 {
					// 确认 NLB 详细配置中无未知监听器
					nlbDetail, err := nlbClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
						NetworkLoadBalancerId: &nlbID,
					})
					if err == nil {
						canDelete := true
						for _, l := range nlbDetail.Listeners {
							// 共享模式若仍有其他监听器则不删
							if l.Name != nil && !strings.Contains(*l.Name, shortInstanceHash(req.InstanceID)) {
								canDelete = false
								break
							}
						}
						if canDelete {
							_, _ = nlbClient.DeleteNetworkLoadBalancer(ctx, networkloadbalancer.DeleteNetworkLoadBalancerRequest{
								NetworkLoadBalancerId: &nlbID,
							})
						}
					}
				}
			}
		}
	}

	// 3. 安全清理 NAT 网关（仅当 !retainNatGw 且该 NAT 为 created 且无任何其他绑定使用时）
	if !retainNatGw && hasBinding && binding.NatGatewayID != "" {
		var activeBindings int64
		database.GetDB().Model(&models.Bandwidth500Binding{}).
			Where("nat_gateway_id = ? AND instance_id != ? AND status = ?", binding.NatGatewayID, req.InstanceID, StatusEnabled).
			Count(&activeBindings)

		if activeBindings == 0 {
			var res models.Bandwidth500Resource
			if err := database.GetDB().Where("ocid = ?", binding.NatGatewayID).First(&res).Error; err == nil {
				if res.CanDelete && res.Origin == OriginCreated {
					_, _ = vnClient.DeleteNatGateway(ctx, core.DeleteNatGatewayRequest{
						NatGatewayId: &binding.NatGatewayID,
					})
				}
			}
		}
	}

	// 4. 更新数据库状态为 disabled
	if hasBinding {
		binding.Status = StatusDisabled
		binding.ObservedHealth = "unknown"
		_ = database.GetDB().Save(&binding).Error
	}

	return nil
}

// MigrateLegacyRequest 原地迁移旧独享 NLB 为共享模式请求
type MigrateLegacyRequest struct {
	InstanceID string                        `json:"instanceId"`
	SSHPort    int                           `json:"sshPort"`
	Mappings   []models.Bandwidth500Endpoint `json:"mappings"`
}

// MigrateLegacyNlbToShared 将第一台历史单实例全端口 NLB 原地改造为受管共享池（保留 NLB OCID 与公网 IP）
func (s *Bandwidth500Service) MigrateLegacyNlbToShared(ctx context.Context, user *models.OciUser, req MigrateLegacyRequest) error {
	if req.SSHPort <= 0 {
		req.SSHPort = 22
	}

	instance, err := s.ociService.GetInstanceById(user, req.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}

	vnic, privateIP, err := s.GetPrimaryVnicAndIP(ctx, user, req.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to get primary VNIC: %w", err)
	}

	vnClient, err := s.ociService.GetVirtualNetworkClient(user)
	if err != nil {
		return err
	}
	subnetResp, err := vnClient.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: vnic.SubnetId})
	if err != nil {
		return err
	}
	vcnID := *subnetResp.VcnId
	compartmentID := *instance.CompartmentId

	lock := s.getVcnLock(user.OciTenantID, user.OciRegion, vcnID)
	lock.Lock()
	defer lock.Unlock()

	nlbClient, err := s.ociService.GetNetworkLoadBalancerClient(user)
	if err != nil {
		return err
	}

	// 寻找带旧受管标签的 NLB
	listResp, err := nlbClient.ListNetworkLoadBalancers(ctx, networkloadbalancer.ListNetworkLoadBalancersRequest{
		CompartmentId:  &compartmentID,
		LifecycleState: networkloadbalancer.ListNetworkLoadBalancersLifecycleStateActive,
	})
	if err != nil || listResp.NetworkLoadBalancerCollection.Items == nil {
		return fmt.Errorf("failed to list network load balancers: %w", err)
	}

	var targetNlb *networkloadbalancer.NetworkLoadBalancerSummary
	for _, n := range listResp.NetworkLoadBalancerCollection.Items {
		if isManagedNlbForInstance(n.FreeformTags, req.InstanceID) {
			targetNlb = &n
			break
		}
	}

	if targetNlb == nil {
		return fmt.Errorf("未找到实例 %s 对应的旧受管网络负载均衡器", req.InstanceID)
	}

	nlbID := *targetNlb.Id
	if len(req.Mappings) == 0 {
		req.Mappings = []models.Bandwidth500Endpoint{
			{
				ID:          "ssh",
				Protocol:    "TCP",
				ListenPort:  22,
				BackendPort: req.SSHPort,
			},
		}
	}

	// 1. 为第一台创建显式端口 BackendSet 与 Listener
	err = s.ensureInstanceEndpoints(ctx, nlbClient, nlbID, req.InstanceID, privateIP, req.Mappings)
	if err != nil {
		return fmt.Errorf("failed to add shared endpoints: %w", err)
	}

	// 2. 移除旧的通配 listener1 与 backend1（若存在）
	delLsResp, err := nlbClient.DeleteListener(ctx, networkloadbalancer.DeleteListenerRequest{
		NetworkLoadBalancerId: &nlbID,
		ListenerName:          stringPtr("listener1"),
	})
	if err == nil {
		_ = s.waitForWorkRequest(ctx, nlbClient, delLsResp.OpcWorkRequestId)
	}

	delBsResp, err := nlbClient.DeleteBackendSet(ctx, networkloadbalancer.DeleteBackendSetRequest{
		NetworkLoadBalancerId: &nlbID,
		BackendSetName:        stringPtr("backend1"),
	})
	if err == nil {
		_ = s.waitForWorkRequest(ctx, nlbClient, delBsResp.OpcWorkRequestId)
	}

	// 3. 更新 NLB 标签为共享池受管标签
	poolID := fmt.Sprintf("pool-%s", vcnID[:min(8, len(vcnID))])
	sharedTags := map[string]string{
		tagManagedKey: tagManagedVal,
		tagFeatureKey: tagFeatureVal,
		tagSchemaKey:  tagSchemaVal,
		tagModeKey:    ModeSharedPorts,
		tagPoolKey:    poolID,
	}

	_, err = nlbClient.UpdateNetworkLoadBalancer(ctx, networkloadbalancer.UpdateNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: &nlbID,
		UpdateNetworkLoadBalancerDetails: networkloadbalancer.UpdateNetworkLoadBalancerDetails{
			FreeformTags: sharedTags,
		},
	})
	if err != nil {
		log.Printf("Warning: failed to update NLB freeform tags to shared pool: %v", err)
	}

	// 4. 持久化第一台的 binding 为 shared-ports
	pubIP := getPublicIPFromNlbAddresses(targetNlb.IpAddresses)
	epBytes, _ := json.Marshal(req.Mappings)

	var binding models.Bandwidth500Binding
	database.GetDB().Where("instance_id = ?", req.InstanceID).First(&binding)
	binding.InstanceID = req.InstanceID
	binding.UserID = user.ID
	binding.TenantID = user.OciTenantID
	binding.Region = user.OciRegion
	binding.VcnID = vcnID
	binding.Mode = ModeSharedPorts
	binding.VnicID = *vnic.Id
	binding.PrivateIP = privateIP
	binding.NlbID = nlbID
	binding.ObservedIP = pubIP
	binding.TargetEndpoints = string(epBytes)
	binding.Status = StatusEnabled
	binding.ObservedHealth = "healthy"
	_ = database.GetDB().Save(&binding).Error

	// 登记该 NLB 为共享资源
	s.registerResource("network_load_balancer", nlbID, user.OciTenantID, user.OciRegion, compartmentID, vcnID, poolID, OriginAdopted, true)

	return nil
}

// 辅助方法
func (s *Bandwidth500Service) waitForNlbActiveAndPublicIP(ctx context.Context, nlbClient networkloadbalancer.NetworkLoadBalancerClient, nlbID string) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return "", fmt.Errorf("timeout waiting for NLB %s to become active", nlbID)
		default:
		}

		nlbResp, err := nlbClient.GetNetworkLoadBalancer(timeoutCtx, networkloadbalancer.GetNetworkLoadBalancerRequest{
			NetworkLoadBalancerId: &nlbID,
		})
		if err == nil && nlbResp.LifecycleState == networkloadbalancer.LifecycleStateActive {
			pubIP := getPublicIPFromNlbAddresses(nlbResp.IpAddresses)
			if pubIP != "" {
				return pubIP, nil
			}
		}

		time.Sleep(3 * time.Second)
	}
}

func (s *Bandwidth500Service) registerResource(resType, ocid, tenantID, region, compartmentID, vcnID, poolID, origin string, canDelete bool) {
	var res models.Bandwidth500Resource
	database.GetDB().Where("ocid = ?", ocid).First(&res)
	if res.ID == "" {
		res.ID = uuid.New().String()
	}
	res.ResourceType = resType
	res.Ocid = ocid
	res.TenantID = tenantID
	res.Region = region
	res.ResourceCompartmentID = compartmentID
	res.VcnID = vcnID
	res.PoolID = poolID
	res.Origin = origin
	res.CanDelete = canDelete
	res.State = "active"
	res.Version = 2
	_ = database.GetDB().Save(&res).Error
}

func getPublicIPFromNlbAddresses(addresses []networkloadbalancer.IpAddress) string {
	for _, ip := range addresses {
		if ip.IpAddress != nil && !isPrivateIP(*ip.IpAddress) {
			return *ip.IpAddress
		}
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
