package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/adiecho/oci-panel/internal/models"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// oci-panel 为受管网络负载均衡器打的 FreeformTag 标记。
// 删除操作只针对带这些标记、且实例标签匹配的 NLB，绝不误删租户内无关的负载均衡器。
const (
	nlbManagedTagKey     = "oci-panel-managed"
	nlbManagedTagValue   = "true"
	nlbInstanceTagKey    = "oci-panel-instance"
	nlbManagedNamePrefix = "oci-panel-nlb-"
)

// managedNlbFreeformTags 返回为指定实例创建 NLB 时应附加的 FreeformTags。
func managedNlbFreeformTags(instanceID string) map[string]string {
	return map[string]string{
		nlbManagedTagKey:  nlbManagedTagValue,
		nlbInstanceTagKey: instanceID,
	}
}

// isManagedNlbForInstance 判断某个网络负载均衡器是否由 oci-panel 为指定实例创建。
// 只有返回 true 的 NLB 才允许被自动删除。判定严格基于 FreeformTags：
// 必须同时满足「受管标记为 true」且「实例标签等于该实例」，从而即使同一租户/compartment
// 内存在其他实例的受管 NLB 或用户自建 NLB，也不会被误删。
func isManagedNlbForInstance(freeformTags map[string]string, instanceID string) bool {
	if freeformTags == nil {
		return false
	}
	if freeformTags[nlbManagedTagKey] != nlbManagedTagValue {
		return false
	}
	return freeformTags[nlbInstanceTagKey] == instanceID && instanceID != ""
}

// Check500MbpsSupport 检查实例是否支持500Mbps功能
// 仅 VM.Standard.E2.1.Micro (AMD) 实例支持此功能
func (s *OCIService) Check500MbpsSupport(user *models.OciUser, instanceID string) (bool, string, error) {
	instance, err := s.GetInstanceById(user, instanceID)
	if err != nil {
		return false, "", fmt.Errorf("failed to get instance: %w", err)
	}

	shape := ""
	if instance.Shape != nil {
		shape = *instance.Shape
	}
	supported := strings.Contains(shape, "E2.1.Micro")
	return supported, shape, nil
}

// Enable500Mbps 一键开启下行500Mbps（接入安全生命周期与编排引擎，支持独享与共享复用）
func (s *OCIService) Enable500Mbps(user *models.OciUser, instanceID string, sshPort int) (string, error) {
	bwService := GetBandwidth500Service(s)
	ctx := context.Background()

	return bwService.Enable500MbpsOrchestrated(ctx, user, EnableRequest{
		InstanceID: instanceID,
		Mode:       ModeDedicated, // 传统入口默认独享全端口，但幂等复用，不再强制删除旧 NLB
		SSHPort:    sshPort,
	})
}

// Disable500Mbps 关闭下行500Mbps（接入安全生命周期，杜绝高危清空整个 VCN 路由或删除共享 NAT 网关）
func (s *OCIService) Disable500Mbps(user *models.OciUser, instanceID string, retainNatGw, retainNlb bool) error {
	bwService := GetBandwidth500Service(s)
	ctx := context.Background()

	return bwService.Disable500MbpsOrchestrated(ctx, user, DisableRequest{
		InstanceID:  instanceID,
		RetainNatGw: &retainNatGw,
		RetainNlb:   &retainNlb,
	})
}

// ReleaseSecurityRules 放行安全规则（保留给管理员手动放行调用，500Mbps 自动流程不再调用，避免破坏用户安全列表）
func (s *OCIService) ReleaseSecurityRules(user *models.OciUser, vcnId string) error {
	ctx := context.Background()

	vnClient, err := s.GetVirtualNetworkClient(user)
	if err != nil {
		return fmt.Errorf("failed to get virtual network client: %w", err)
	}

	// 获取VCN
	vcnResp, err := vnClient.GetVcn(ctx, core.GetVcnRequest{VcnId: &vcnId})
	if err != nil {
		return fmt.Errorf("failed to get VCN: %w", err)
	}

	// 获取安全列表
	secListResp, err := vnClient.ListSecurityLists(ctx, core.ListSecurityListsRequest{
		CompartmentId: vcnResp.CompartmentId,
		VcnId:         &vcnId,
	})
	if err != nil {
		return fmt.Errorf("failed to list security lists: %w", err)
	}

	allProtocol := "all"
	ipv4Cidr := "0.0.0.0/0"
	ipv6Cidr := "::/0"
	internalCidr := "10.0.0.0/16"

	for _, secList := range secListResp.Items {
		// 更新入站规则
		ingressRules := []core.IngressSecurityRule{
			{
				Protocol: &allProtocol,
				Source:   &ipv4Cidr,
			},
			{
				Protocol: &allProtocol,
				Source:   &ipv6Cidr,
			},
			{
				Protocol: &allProtocol,
				Source:   &internalCidr,
			},
		}

		// 更新出站规则
		egressRules := []core.EgressSecurityRule{
			{
				Protocol:    &allProtocol,
				Destination: &ipv4Cidr,
			},
			{
				Protocol:    &allProtocol,
				Destination: &ipv6Cidr,
			},
		}

		_, err = vnClient.UpdateSecurityList(ctx, core.UpdateSecurityListRequest{
			SecurityListId: secList.Id,
			UpdateSecurityListDetails: core.UpdateSecurityListDetails{
				IngressSecurityRules: ingressRules,
				EgressSecurityRules:  egressRules,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to update security list: %w", err)
		}
	}

	return nil
}
