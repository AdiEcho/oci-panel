package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/adiecho/oci-panel/internal/util"
	"github.com/google/uuid"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

// oci_bandwidth_pool.go — 「先建 ELB，后分配给机器」的受管共享 NLB 池。
//
// 500Mbps 依赖 NAT 网关 + 网络负载均衡器（OCI NLB，控制台称 ELB）。此前只能在实例上点
// 「开启500Mbps」时由编排流程顺带创建 NLB，配额紧张或想提前备料时没有入口。
// 这里把 NLB 抽成可独立存在的池资源：
//   - 创建：不绑定实例，按 VCN/子网建好共享 NLB 并打上受管标签，状态 available；
//   - 分配：把某个 available 的 NLB 交给实例开启流程复用（不再新建 NLB），状态 assigned；
//   - 解绑：实例关闭 500Mbps 且无其他成员时，自动回到 available；
//   - 删除：仅允许删除无任何成员引用的 ELB。

// 池记录状态
const (
	PoolStatusProvisioning = "provisioning" // 云端创建中，尚未拿到公网 IP
	PoolStatusAvailable    = "available"    // 已创建、未分配给任何实例
	PoolStatusAssigned     = "assigned"     // 已分配给实例
	PoolStatusMissing      = "missing"      // 库中有记录但云端 NLB 已不存在
	PoolStatusFailed       = "failed"       // 创建过程中失败
)

// CreatePoolRequest 创建独立 ELB 请求
type CreatePoolRequest struct {
	VcnID    string `json:"vcnId"`
	SubnetID string `json:"subnetId"`
	NlbName  string `json:"nlbName"`
}

// ListNlbPools 列出当前配置下由面板管理的 ELB（合并云端实时状态与本地池记录）
func (s *Bandwidth500Service) ListNlbPools(ctx context.Context, user *models.OciUser) ([]models.Bandwidth500PoolView, error) {
	nlbClient, err := s.ociService.GetNetworkLoadBalancerClient(user)
	if err != nil {
		return nil, fmt.Errorf("failed to get NLB client: %w", err)
	}

	compartmentID := user.OciTenantID
	items, err := paginate(func(page *string) ([]networkloadbalancer.NetworkLoadBalancerSummary, *string, error) {
		resp, err := nlbClient.ListNetworkLoadBalancers(ctx, networkloadbalancer.ListNetworkLoadBalancersRequest{
			CompartmentId:  &compartmentID,
			LifecycleState: networkloadbalancer.ListNetworkLoadBalancersLifecycleStateActive,
			Page:           page,
		})
		if err != nil {
			return nil, nil, err
		}
		return resp.NetworkLoadBalancerCollection.Items, resp.OpcNextPage, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list network load balancers: %w", err)
	}

	// VCN 名称映射（一次列举，避免逐条查询）
	vcnNames := map[string]string{}
	if vnClient, err := s.ociService.GetVirtualNetworkClient(user); err == nil {
		if vcns, err := paginate(func(page *string) ([]core.Vcn, *string, error) {
			resp, err := vnClient.ListVcns(ctx, core.ListVcnsRequest{CompartmentId: &compartmentID, Page: page})
			if err != nil {
				return nil, nil, err
			}
			return resp.Items, resp.OpcNextPage, nil
		}); err == nil {
			for _, v := range vcns {
				if v.Id != nil && v.DisplayName != nil {
					vcnNames[*v.Id] = *v.DisplayName
				}
			}
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	views := make([]models.Bandwidth500PoolView, 0, len(items))
	seen := map[string]bool{}

	for _, nlb := range items {
		tags := nlb.FreeformTags
		if tags == nil || tags[tagManagedKey] != tagManagedVal || tags[tagFeatureKey] != tagFeatureVal {
			continue
		}
		if nlb.Id == nil {
			continue
		}
		nlbID := *nlb.Id
		seen[nlbID] = true

		vcnID := s.lookupNlbVcnID(ctx, user, nlb)
		view := models.Bandwidth500PoolView{
			ID:            nlbID,
			NlbID:         nlbID,
			NlbName:       derefString(nlb.DisplayName),
			AccessIP:      getPublicIPFromNlbAddresses(nlb.IpAddresses),
			VcnID:         vcnID,
			VcnName:       vcnNames[vcnID],
			SubnetID:      derefString(nlb.SubnetId),
			CompartmentID: derefString(nlb.CompartmentId),
			Mode:          tags[tagModeKey],
			Status:        PoolStatusAvailable,
			ObservedAt:    now,
		}
		if view.Mode == "" {
			view.Mode = ModeSharedPorts
		}
		if view.Mode == ModeDedicated {
			// 历史独享 NLB 仍可在此展示与代管，但不参与「预建分配」流程
			view.Status = PoolStatusAssigned
			view.CanDelete = false
		}
		if nlb.TimeCreated != nil {
			view.CreatedAt = nlb.TimeCreated.Format("2006-01-02 15:04:05")
		}

		// 本地池记录（若存在）用于判断归属与来源
		var pool models.Bandwidth500Pool
		hasPoolRecord := database.GetDB().Where("nlb_id = ?", nlbID).First(&pool).Error == nil
		if hasPoolRecord {
			view.Origin = pool.Origin
			view.LastError = pool.LastError
			if !pool.CreatedAt.IsZero() && view.CreatedAt == "" {
				view.CreatedAt = pool.CreatedAt.Format("2006-01-02 15:04:05")
			}
			if pool.Status == PoolStatusProvisioning || pool.Status == PoolStatusFailed {
				if view.AccessIP == "" {
					// 失败记录保留失败状态与原因，创建中则显示创建中
					if pool.Status == PoolStatusFailed {
						view.Status = PoolStatusFailed
					} else {
						view.Status = PoolStatusProvisioning
					}
				} else {
					// 云端已可用，顺手把记录推进到可用并清除历史错误
					pool.Status = PoolStatusAvailable
					pool.AccessIP = view.AccessIP
					pool.LastError = ""
					view.LastError = ""
					_ = database.GetDB().Save(&pool).Error
				}
			}
		}
		if view.Origin == "" {
			view.Origin = OriginAdopted
		}

		// 成员统计：以真正处于 enabled 的绑定为准
		view.MemberCount = s.countEnabledMembers(nlbID)
		instanceID := tags[tagInstanceKey]
		if instanceID != "" {
			view.InstanceID = instanceID
		} else if view.MemberCount > 0 {
			view.InstanceID = s.firstEnabledMember(nlbID)
		}
		if view.InstanceID != "" {
			view.InstanceName = s.instanceDisplayName(user, view.InstanceID)
			view.Status = PoolStatusAssigned
		}
		// 云端还没有公网地址时仍视为创建中，避免被误认领
		if view.Status == PoolStatusAvailable && view.AccessIP == "" && view.Mode == ModeSharedPorts {
			view.Status = PoolStatusProvisioning
		}
		if view.Status == PoolStatusProvisioning || view.Status == PoolStatusFailed {
			view.CanAssign = false
			view.CanDelete = view.Mode == ModeSharedPorts
			views = append(views, view)
			continue
		}
		// 只有共享池且无成员时才可分配；带实例标签但无 enabled 绑定（如失败态）也允许重新分配
		view.CanAssign = view.Mode == ModeSharedPorts && view.MemberCount == 0
		view.CanDelete = view.Mode == ModeSharedPorts && view.MemberCount == 0
		views = append(views, view)
	}

	// 库中存在、但 active 列表里没有的记录：可能正在创建、刚创建完或确实已丢失。
	// 不能一律标记「云端丢失」，否则新建 ELB 后页面会先闪出误导性的丢失提示。
	var orphans []models.Bandwidth500Pool
	if dbErr := database.GetDB().Where("user_id = ?", user.ID).Find(&orphans).Error; dbErr == nil {
		for _, pool := range orphans {
			if seen[pool.NlbID] {
				continue
			}
			view := models.Bandwidth500PoolView{
				ID:            pool.ID,
				NlbID:         pool.NlbID,
				NlbName:       pool.NlbName,
				AccessIP:      pool.AccessIP,
				VcnID:         pool.VcnID,
				VcnName:       vcnNames[pool.VcnID],
				SubnetID:      pool.SubnetID,
				CompartmentID: pool.CompartmentID,
				Mode:          ModeSharedPorts,
				Origin:        pool.Origin,
				InstanceID:    pool.InstanceID,
				InstanceName:  s.instanceDisplayName(user, pool.InstanceID),
				CreatedAt:     pool.CreatedAt.Format("2006-01-02 15:04:05"),
				ObservedAt:    now,
			}
			views = append(views, s.resolvePendingPool(ctx, nlbClient, pool, view))
		}
	}

	// 未分配优先靠前，其余按创建时间倒序
	sort.SliceStable(views, func(i, j int) bool {
		ai := views[i].Status != PoolStatusAvailable
		aj := views[j].Status != PoolStatusAvailable
		if ai != aj {
			return !ai
		}
		return views[i].CreatedAt > views[j].CreatedAt
	})

	return views, nil
}

// CreateNlbPool 按 VCN 预建一个受管共享 ELB（不绑定实例、不建立后端与监听器）
func (s *Bandwidth500Service) CreateNlbPool(ctx context.Context, user *models.OciUser, req CreatePoolRequest) (*models.Bandwidth500PoolView, error) {
	if strings.TrimSpace(req.VcnID) == "" {
		return nil, fmt.Errorf("请选择 ELB 所属 VCN")
	}

	vnClient, err := s.ociService.GetVirtualNetworkClient(user)
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual network client: %w", err)
	}
	nlbClient, err := s.ociService.GetNetworkLoadBalancerClient(user)
	if err != nil {
		return nil, fmt.Errorf("failed to get NLB client: %w", err)
	}

	vcnResp, err := vnClient.GetVcn(ctx, core.GetVcnRequest{VcnId: &req.VcnID})
	if err != nil {
		return nil, fmt.Errorf("failed to get VCN: %w", err)
	}
	vcnID := req.VcnID
	compartmentID := derefString(vcnResp.CompartmentId)
	if compartmentID == "" {
		compartmentID = user.OciTenantID
	}

	subnetID := req.SubnetID
	if subnetID != "" {
		sub, err := vnClient.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: &subnetID})
		if err != nil {
			return nil, fmt.Errorf("failed to get subnet: %w", err)
		}
		if sub.VcnId != nil && *sub.VcnId != vcnID {
			return nil, fmt.Errorf("所选子网不属于该 VCN")
		}
	} else {
		subnetID, err = s.pickPublicSubnet(ctx, vnClient, compartmentID, vcnID)
		if err != nil {
			return nil, err
		}
	}

	// 物理作用域锁，避免与实例开启流程并发创建同 VCN 的池
	lock := s.getVcnLock(user.OciTenantID, user.OciRegion, vcnID)
	lock.Lock()
	defer lock.Unlock()

	nlbName := strings.TrimSpace(req.NlbName)
	if nlbName == "" {
		nlbName = fmt.Sprintf("oci-panel-shared-nlb-%s", time.Now().Format("20060102150405"))
	}
	poolID := s.poolIDForVcn(vcnID)

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
		return nil, ClassifyOCIError(err)
	}

	nlbID := *createResp.Id
	s.registerResource("network_load_balancer", nlbID, user.OciTenantID, user.OciRegion, compartmentID, vcnID, poolID, OriginCreated, true)

	pool := &models.Bandwidth500Pool{
		UserID:        user.ID,
		TenantID:      user.OciTenantID,
		Region:        user.OciRegion,
		CompartmentID: compartmentID,
		VcnID:         vcnID,
		SubnetID:      subnetID,
		PoolID:        poolID,
		NlbID:         nlbID,
		NlbName:       nlbName,
		Mode:          ModeSharedPorts,
		Origin:        OriginCreated,
		Status:        PoolStatusProvisioning,
	}
	s.savePoolRecord(pool)

	// OCI 建 NLB 通常需要 1-3 分钟，放到后台等待，避免 HTTP 请求超时
	poolRecordID := pool.ID
	util.Go("WaitNlbPoolActive", func() {
		publicIP, err := s.waitForNlbActiveAndPublicIP(context.Background(), nlbClient, nlbID)
		var record models.Bandwidth500Pool
		if dbErr := database.GetDB().Where("id = ?", poolRecordID).First(&record).Error; dbErr != nil {
			return
		}
		if err != nil {
			record.Status = PoolStatusFailed
			record.LastError = fmt.Sprintf("ELB 已创建，但等待公网地址失败: %v", err)
			_ = database.GetDB().Save(&record).Error
			return
		}
		record.Status = PoolStatusAvailable
		record.AccessIP = publicIP
		record.LastError = ""
		_ = database.GetDB().Save(&record).Error
	})

	return &models.Bandwidth500PoolView{
		ID:            pool.ID,
		NlbID:         nlbID,
		NlbName:       nlbName,
		VcnID:         vcnID,
		VcnName:       derefString(vcnResp.DisplayName),
		SubnetID:      subnetID,
		CompartmentID: compartmentID,
		Mode:          ModeSharedPorts,
		Origin:        OriginCreated,
		Status:        PoolStatusProvisioning,
		CanDelete:     true,
		ObservedAt:    time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// poolProvisionGrace 新建 ELB 在云端尚未进入 active 列表时的宽限时间。
// 期间即使 active 列表或 Get 查不到，也只显示「创建中」，不误判为云端丢失。
const poolProvisionGrace = 15 * time.Minute

func withinPoolGrace(createdAt time.Time) bool {
	if createdAt.IsZero() {
		return false
	}
	return time.Since(createdAt) < poolProvisionGrace
}

// resolvePendingPool 处理「本地有池记录、但 active 列表里没有」的 ELB：
// 正在创建 / 刚创建完 / 确实丢失 三种情况必须区分，避免新建后页面闪现「云端丢失」。
func (s *Bandwidth500Service) resolvePendingPool(ctx context.Context, nlbClient networkloadbalancer.NetworkLoadBalancerClient, pool models.Bandwidth500Pool, view models.Bandwidth500PoolView) models.Bandwidth500PoolView {
	view.CanDelete = false
	if pool.NlbID == "" {
		view.Status = PoolStatusFailed
		view.LastError = pool.LastError
		return view
	}

	detail, err := nlbClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: &pool.NlbID,
	})
	if err == nil {
		publicIP := getPublicIPFromNlbAddresses(detail.IpAddresses)
		if publicIP != "" {
			view.AccessIP = publicIP
		}
		if detail.LifecycleState == networkloadbalancer.LifecycleStateActive && publicIP != "" {
			// 后台等待可能超时或进程重启，这里顺手把记录修正为可用
			view.Status = PoolStatusAvailable
			view.CanAssign = true
			view.CanDelete = true
			view.LastError = ""
			_ = database.GetDB().Model(&models.Bandwidth500Pool{}).Where("id = ?", pool.ID).Updates(map[string]any{
				"status": PoolStatusAvailable, "access_ip": publicIP, "last_error": "",
			}).Error
			return view
		}
		// 仍在 CREATING / UPDATING，属于正常创建过程
		view.Status = PoolStatusProvisioning
		view.CanDelete = true
		view.LastError = ""
		return view
	}

	if isNotFoundError(err) {
		if withinPoolGrace(pool.CreatedAt) && (pool.Status == PoolStatusProvisioning || pool.Status == PoolStatusFailed) {
			view.Status = pool.Status
			view.LastError = pool.LastError
			view.CanDelete = true
			return view
		}
	} else if withinPoolGrace(pool.CreatedAt) && pool.Status == PoolStatusProvisioning {
		// 暂时无法核验（权限、网络波动）时也不立即判丢失
		view.Status = PoolStatusProvisioning
		view.CanDelete = true
		return view
	}

	view.Status = PoolStatusMissing
	view.LastError = "云端 NLB 已不存在，请在 OCI 控制台核查或删除此记录"
	return view
}

// DeleteNlbPool 删除一个未分配（无任何 enabled 成员）的受管 ELB
func (s *Bandwidth500Service) DeleteNlbPool(ctx context.Context, user *models.OciUser, nlbID string) error {
	if strings.TrimSpace(nlbID) == "" {
		return fmt.Errorf("缺少 ELB 标识")
	}
	if s.countEnabledMembers(nlbID) > 0 {
		return fmt.Errorf("该 ELB 仍被实例占用，请先在实例上关闭 500Mbps")
	}

	nlbClient, err := s.ociService.GetNetworkLoadBalancerClient(user)
	if err != nil {
		return fmt.Errorf("failed to get NLB client: %w", err)
	}

	detail, err := nlbClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: &nlbID,
	})
	switch {
	case err == nil:
		tags := detail.FreeformTags
		if tags == nil || tags[tagManagedKey] != tagManagedVal || tags[tagFeatureKey] != tagFeatureVal {
			return fmt.Errorf("只能删除由面板创建或纳管的 ELB")
		}
		if _, err := nlbClient.DeleteNetworkLoadBalancer(ctx, networkloadbalancer.DeleteNetworkLoadBalancerRequest{
			NetworkLoadBalancerId: &nlbID,
		}); err != nil {
			return ClassifyOCIError(err)
		}
	case isNotFoundError(err):
		log.Printf("DeleteNlbPool: 云端 NLB %s 已不存在，清理本地记录", nlbID)
	default:
		return ClassifyOCIError(err)
	}

	var pool models.Bandwidth500Pool
	if dbErr := database.GetDB().Where("nlb_id = ?", nlbID).First(&pool).Error; dbErr == nil {
		_ = database.GetDB().Delete(&pool).Error
	}
	var res models.Bandwidth500Resource
	if dbErr := database.GetDB().Where("ocid = ?", nlbID).First(&res).Error; dbErr == nil {
		res.State = "deleted"
		_ = database.GetDB().Save(&res).Error
	}
	return nil
}

// findAvailablePool 返回本配置中同 VCN 的首个待分配 ELB
func (s *Bandwidth500Service) findAvailablePool(userID, vcnID string) string {
	var pools []models.Bandwidth500Pool
	_ = database.GetDB().
		Where("user_id = ? AND status = ? AND mode = ?", userID, PoolStatusAvailable, ModeSharedPorts).
		Order("created_at asc").Find(&pools).Error
	for _, p := range pools {
		if p.NlbID == "" {
			continue
		}
		if p.VcnID != "" && p.VcnID != vcnID {
			continue
		}
		return p.NlbID
	}
	return ""
}

// acquireSharedNlb 共享模式下确定要使用的 NLB：显式指定 > 本配置的可用池 > 既有共享池/自动升级 > 新建
func (s *Bandwidth500Service) acquireSharedNlb(ctx context.Context, user *models.OciUser, nlbClient networkloadbalancer.NetworkLoadBalancerClient, compartmentID, subnetID, vcnID, preferredNlbID string) (string, string, error) {
	if preferredNlbID != "" {
		return s.resolvePoolNlb(ctx, user, nlbClient, vcnID, preferredNlbID)
	}

	// 优先复用本配置中已建好但尚未分配的 ELB（优先同 VCN）
	var pools []models.Bandwidth500Pool
	_ = database.GetDB().
		Where("user_id = ? AND status = ? AND mode = ?", user.ID, PoolStatusAvailable, ModeSharedPorts).
		Order("created_at asc").Find(&pools).Error
	for _, p := range pools {
		if p.NlbID == "" {
			continue
		}
		if p.VcnID != "" && p.VcnID != vcnID {
			continue
		}
		nlbID, publicIP, err := s.resolvePoolNlb(ctx, user, nlbClient, vcnID, p.NlbID)
		if err != nil {
			log.Printf("acquireSharedNlb: 池 NLB %s 不可用，继续尝试其它候选: %v", p.NlbID, err)
			continue
		}
		return nlbID, publicIP, nil
	}

	return s.ensureSharedNlb(ctx, user, nlbClient, compartmentID, subnetID, vcnID)
}

// resolvePoolNlb 校验并返回指定 NLB 的 ID 与公网 IP
func (s *Bandwidth500Service) resolvePoolNlb(ctx context.Context, user *models.OciUser, nlbClient networkloadbalancer.NetworkLoadBalancerClient, vcnID, nlbID string) (string, string, error) {
	detail, err := nlbClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: &nlbID,
	})
	if err != nil {
		return "", "", fmt.Errorf("获取 ELB %s 失败: %w", nlbID, ClassifyOCIError(err))
	}
	tags := detail.FreeformTags
	if tags == nil || tags[tagManagedKey] != tagManagedVal || tags[tagFeatureKey] != tagFeatureVal {
		return "", "", fmt.Errorf("ELB %s 不是由面板创建的受管负载均衡器", nlbID)
	}
	if tags[tagModeKey] == ModeDedicated {
		return "", "", fmt.Errorf("ELB %s 处于独享全端口模式，无法加入共享池", nlbID)
	}
	if detail.LifecycleState != networkloadbalancer.LifecycleStateActive {
		return "", "", fmt.Errorf("ELB %s 当前状态为 %s，请等待其可用", nlbID, detail.LifecycleState)
	}
	publicIP := getPublicIPFromNlbAddresses(detail.IpAddresses)
	if publicIP == "" {
		return "", "", fmt.Errorf("ELB %s 暂无公网 IP，无法作为共享访问入口", nlbID)
	}

	// ELB 与实例必须同属一个 VCN，否则后端私有 IP 无法被转发
	if vcnID != "" && detail.SubnetId != nil {
		if vnClient, err := s.ociService.GetVirtualNetworkClient(user); err == nil {
			if sub, err := vnClient.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: detail.SubnetId}); err == nil && sub.VcnId != nil && *sub.VcnId != vcnID {
				return "", "", fmt.Errorf("ELB %s 属于其它 VCN，无法分配给当前 VCN 内的实例", nlbID)
			}
		}
	}
	return nlbID, publicIP, nil
}

// markPoolAssigned 实例开启成功后把 ELB 池记录标记为已分配；
// 若该 NLB 原先不在池中（如历史共享池或被自动升级的独享 NLB），补建一条 adopted 记录。
func (s *Bandwidth500Service) markPoolAssigned(user *models.OciUser, nlbID, instanceID, vcnID string) {
	if nlbID == "" {
		return
	}
	var pool models.Bandwidth500Pool
	poolErr := database.GetDB().Where("nlb_id = ?", nlbID).First(&pool).Error
	if poolErr != nil {
		pool = models.Bandwidth500Pool{
			ID:         uuid.New().String(),
			UserID:     user.ID,
			TenantID:   user.OciTenantID,
			Region:     user.OciRegion,
			VcnID:      vcnID,
			PoolID:     s.poolIDForVcn(vcnID),
			NlbID:      nlbID,
			Mode:       ModeSharedPorts,
			Origin:     OriginAdopted,
			AccessIP:   s.lookupBindingIP(instanceID),
			Status:     PoolStatusAssigned,
			InstanceID: instanceID,
		}
		_ = database.GetDB().Create(&pool).Error
		return
	}
	pool.Status = PoolStatusAssigned
	// 共享池可能被多台实例同时使用，保留首个分配者作为展示归属
	if pool.InstanceID == "" || s.countEnabledMembers(nlbID) <= 1 {
		pool.InstanceID = instanceID
	}
	pool.LastError = ""
	if pool.AccessIP == "" {
		pool.AccessIP = s.lookupBindingIP(instanceID)
	}
	_ = database.GetDB().Save(&pool).Error
}

// deletePoolRecord 云端 NLB 被删除后清理本地池记录
func (s *Bandwidth500Service) deletePoolRecord(nlbID string) {
	if nlbID == "" {
		return
	}
	_ = database.GetDB().Where("nlb_id = ?", nlbID).Delete(&models.Bandwidth500Pool{}).Error
}

// releasePoolNlb 实例关闭后，若该 NLB 已无其它成员则回池
func (s *Bandwidth500Service) releasePoolNlb(nlbID string) {
	if nlbID == "" {
		return
	}
	var pool models.Bandwidth500Pool
	if err := database.GetDB().Where("nlb_id = ?", nlbID).First(&pool).Error; err != nil {
		return
	}
	if s.countEnabledMembers(nlbID) > 0 {
		return
	}
	pool.Status = PoolStatusAvailable
	pool.InstanceID = ""
	_ = database.GetDB().Save(&pool).Error
}

// countEnabledMembers 统计共享该 NLB 且状态为 enabled 的实例绑定数
func (s *Bandwidth500Service) countEnabledMembers(nlbID string) int {
	var count int64
	database.GetDB().Model(&models.Bandwidth500Binding{}).
		Where("nlb_id = ? AND status = ?", nlbID, StatusEnabled).Count(&count)
	return int(count)
}

func (s *Bandwidth500Service) firstEnabledMember(nlbID string) string {
	var binding models.Bandwidth500Binding
	if err := database.GetDB().Where("nlb_id = ? AND status = ?", nlbID, StatusEnabled).
		Order("created_at asc").First(&binding).Error; err != nil {
		return ""
	}
	return binding.InstanceID
}

func (s *Bandwidth500Service) lookupBindingIP(instanceID string) string {
	var binding models.Bandwidth500Binding
	if err := database.GetDB().Where("instance_id = ?", instanceID).First(&binding).Error; err != nil {
		return ""
	}
	return binding.ObservedIP
}

func (s *Bandwidth500Service) instanceDisplayName(user *models.OciUser, instanceID string) string {
	if instanceID == "" {
		return ""
	}
	if inst, err := s.ociService.GetInstanceById(user, instanceID); err == nil && inst.DisplayName != nil {
		return *inst.DisplayName
	}
	return ""
}

// lookupNlbVcnID 通过 NLB 所在子网反查 VCN
func (s *Bandwidth500Service) lookupNlbVcnID(ctx context.Context, user *models.OciUser, nlb networkloadbalancer.NetworkLoadBalancerSummary) string {
	if nlb.SubnetId == nil || *nlb.SubnetId == "" {
		return ""
	}
	vnClient, err := s.ociService.GetVirtualNetworkClient(user)
	if err != nil {
		return ""
	}
	sub, err := vnClient.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: nlb.SubnetId})
	if err != nil || sub.VcnId == nil {
		return ""
	}
	return *sub.VcnId
}

// pickPublicSubnet 在同 VCN 中挑一个允许公网 IP 的可用子网
func (s *Bandwidth500Service) pickPublicSubnet(ctx context.Context, vnClient core.VirtualNetworkClient, compartmentID, vcnID string) (string, error) {
	subnets, err := paginate(func(page *string) ([]core.Subnet, *string, error) {
		resp, err := vnClient.ListSubnets(ctx, core.ListSubnetsRequest{
			CompartmentId:  &compartmentID,
			VcnId:          &vcnID,
			LifecycleState: core.SubnetLifecycleStateAvailable,
			Page:           page,
		})
		if err != nil {
			return nil, nil, err
		}
		return resp.Items, resp.OpcNextPage, nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to list subnets: %w", err)
	}
	if len(subnets) == 0 {
		return "", fmt.Errorf("该 VCN 下没有可用子网，无法创建 ELB")
	}
	for _, sub := range subnets {
		if sub.Id == nil {
			continue
		}
		if sub.ProhibitPublicIpOnVnic != nil && *sub.ProhibitPublicIpOnVnic {
			continue
		}
		return *sub.Id, nil
	}
	if subnets[0].Id != nil {
		return *subnets[0].Id, nil
	}
	return "", fmt.Errorf("该 VCN 下没有可用子网，无法创建 ELB")
}

func (s *Bandwidth500Service) poolIDForVcn(vcnID string) string {
	return fmt.Sprintf("pool-%s", vcnID[:min(8, len(vcnID))])
}

func (s *Bandwidth500Service) savePoolRecord(pool *models.Bandwidth500Pool) {
	var existing models.Bandwidth500Pool
	if pool.NlbID != "" {
		if err := database.GetDB().Where("nlb_id = ?", pool.NlbID).First(&existing).Error; err == nil && existing.ID != "" {
			pool.ID = existing.ID
			pool.CreatedAt = existing.CreatedAt
		}
	}
	if pool.ID == "" {
		pool.ID = uuid.New().String()
	}
	_ = database.GetDB().Save(pool).Error
}

// isNotFoundError 判断 OCI 错误是否为「资源不存在」
func isNotFoundError(err error) bool {
	var svcErr common.ServiceError
	if !errors.As(err, &svcErr) {
		return false
	}
	return svcErr.GetHTTPStatusCode() == 404 || strings.Contains(strings.ToLower(svcErr.GetCode()), "notfound")
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
