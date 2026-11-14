package services

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/adiecho/oci-panel/internal/models"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// ListVCNs 列出虚拟云网络
func (s *OCIService) ListVCNs(ctx context.Context, user *models.OciUser, compartmentId string) ([]models.VCNInfo, error) {
	client, err := s.GetVirtualNetworkClient(user)
	if err != nil {
		return nil, err
	}

	// 全量分页：消除只取首页导致的 VCN 截断。
	vcnItems, err := paginate(func(page *string) ([]core.Vcn, *string, error) {
		resp, err := client.ListVcns(ctx, core.ListVcnsRequest{
			CompartmentId: &compartmentId,
			Page:          page,
		})
		if err != nil {
			return nil, nil, err
		}
		return resp.Items, resp.OpcNextPage, nil
	})
	if err != nil {
		return nil, err
	}

	vcns := make([]models.VCNInfo, 0, len(vcnItems))
	for _, vcn := range vcnItems {
		vcnInfo := models.VCNInfo{
			ID:          *vcn.Id,
			DisplayName: *vcn.DisplayName,
			State:       string(vcn.LifecycleState),
			Subnets:     []models.SubnetInfo{},
		}
		// 使用 CidrBlocks 替代已弃用的 CidrBlock
		if len(vcn.CidrBlocks) > 0 {
			vcnInfo.CIDRBlock = vcn.CidrBlocks[0]
		}
		if vcn.TimeCreated != nil {
			vcnInfo.CreateTime = vcn.TimeCreated.Format("2006-01-02 15:04:05")
		}

		// 获取子网列表（全量分页）
		vcnID := vcn.Id
		subnets, _ := paginate(func(page *string) ([]core.Subnet, *string, error) {
			resp, err := client.ListSubnets(ctx, core.ListSubnetsRequest{
				CompartmentId: &compartmentId,
				VcnId:         vcnID,
				Page:          page,
			})
			if err != nil {
				return nil, nil, err
			}
			return resp.Items, resp.OpcNextPage, nil
		})
		for _, subnet := range subnets {
			subnetInfo := models.SubnetInfo{
				ID:    *subnet.Id,
				State: string(subnet.LifecycleState),
			}
			if subnet.DisplayName != nil {
				subnetInfo.DisplayName = *subnet.DisplayName
			}
			// 使用 Ipv4CidrBlocks 替代已弃用的 CidrBlock
			if len(subnet.Ipv4CidrBlocks) > 0 {
				subnetInfo.CIDRBlock = subnet.Ipv4CidrBlocks[0]
			}
			if subnet.AvailabilityDomain != nil {
				subnetInfo.AvailabilityDomain = *subnet.AvailabilityDomain
			}
			if subnet.ProhibitPublicIpOnVnic != nil {
				subnetInfo.IsPublic = !*subnet.ProhibitPublicIpOnVnic
			}
			vcnInfo.Subnets = append(vcnInfo.Subnets, subnetInfo)
		}

		vcns = append(vcns, vcnInfo)
	}

	return vcns, nil
}

// CreateVcnParams 新建 VCN 参数（VCN + Internet 网关 + 默认路由 + 公有子网，与新建机器时的自动建网逻辑一致）。
type CreateVcnParams struct {
	DisplayName   string
	CidrBlock     string
	IsIpv6Enabled bool
}

// CreateVcnWithNetwork 创建 VCN 并配套 Internet 网关、默认路由与公有子网。
// 与新建机器时「无可用子网则自动建网」的路径共用同一套步骤（VCN → IGW → 0.0.0.0/0 路由 → 公有子网），
// 入口开放给 VCN 页面，允许自定义名称、CIDR 与 IPv6。
func (s *OCIService) CreateVcnWithNetwork(ctx context.Context, user *models.OciUser, params CreateVcnParams) (*models.VCNInfo, error) {
	vnClient, err := s.GetVirtualNetworkClient(user)
	if err != nil {
		return nil, fmt.Errorf("获取网络客户端失败: %w", err)
	}

	compartmentId := user.OciTenantID

	cidrBlock := params.CidrBlock
	if cidrBlock == "" {
		cidrBlock = "10.0.0.0/16"
	}
	if _, _, err := net.ParseCIDR(cidrBlock); err != nil {
		return nil, fmt.Errorf("CIDR 格式不正确: %w", err)
	}

	displayName := params.DisplayName
	if displayName == "" {
		displayName = fmt.Sprintf("oci-panel-vcn-%d", time.Now().Unix())
	}

	// 1. 创建 VCN
	isIpv6Enabled := params.IsIpv6Enabled
	createVcnResp, err := vnClient.CreateVcn(ctx, core.CreateVcnRequest{
		CreateVcnDetails: core.CreateVcnDetails{
			CompartmentId: &compartmentId,
			DisplayName:   &displayName,
			CidrBlock:     &cidrBlock,
			IsIpv6Enabled: &isIpv6Enabled,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("创建VCN失败: %w", err)
	}
	vcnResp, ok := waitForState(ctx, 30, time.Second,
		func() (core.GetVcnResponse, error) {
			return vnClient.GetVcn(ctx, core.GetVcnRequest{VcnId: createVcnResp.Id})
		},
		func(r core.GetVcnResponse) bool { return r.LifecycleState == core.VcnLifecycleStateAvailable })
	if !ok {
		return nil, fmt.Errorf("等待VCN创建超时")
	}
	vcn := vcnResp.Vcn

	// 2. Internet 网关：新建 VCN 无网关，缺失即创建，已有则复用
	igwResp, err := vnClient.ListInternetGateways(ctx, core.ListInternetGatewaysRequest{
		CompartmentId: &compartmentId,
		VcnId:         vcn.Id,
	})
	if err != nil {
		return nil, fmt.Errorf("获取Internet网关列表失败: %w", err)
	}

	var internetGatewayId *string
	if len(igwResp.Items) == 0 {
		igwName := displayName + "-igw"
		isEnabled := true
		createIgwResp, err := vnClient.CreateInternetGateway(ctx, core.CreateInternetGatewayRequest{
			CreateInternetGatewayDetails: core.CreateInternetGatewayDetails{
				CompartmentId: &compartmentId,
				VcnId:         vcn.Id,
				DisplayName:   &igwName,
				IsEnabled:     &isEnabled,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("创建Internet网关失败: %w", err)
		}
		_, ok := waitForState(ctx, 30, time.Second,
			func() (core.GetInternetGatewayResponse, error) {
				return vnClient.GetInternetGateway(ctx, core.GetInternetGatewayRequest{IgId: createIgwResp.Id})
			},
			func(r core.GetInternetGatewayResponse) bool {
				return r.LifecycleState == core.InternetGatewayLifecycleStateAvailable
			})
		if !ok {
			return nil, fmt.Errorf("等待Internet网关创建超时")
		}
		internetGatewayId = createIgwResp.Id
	} else {
		internetGatewayId = igwResp.Items[0].Id
	}

	// 3. 默认路由表补 0.0.0.0/0 指向 Internet 网关
	if vcn.DefaultRouteTableId != nil {
		getRtResp, err := vnClient.GetRouteTable(ctx, core.GetRouteTableRequest{RtId: vcn.DefaultRouteTableId})
		if err == nil {
			hasDefaultRoute := false
			for _, rule := range getRtResp.RouteRules {
				if rule.Destination != nil && *rule.Destination == "0.0.0.0/0" {
					hasDefaultRoute = true
					break
				}
			}
			if !hasDefaultRoute {
				destination := "0.0.0.0/0"
				updatedRules := append(getRtResp.RouteRules, core.RouteRule{
					Destination:     &destination,
					DestinationType: core.RouteRuleDestinationTypeCidrBlock,
					NetworkEntityId: internetGatewayId,
				})
				_, err = vnClient.UpdateRouteTable(ctx, core.UpdateRouteTableRequest{
					RtId: vcn.DefaultRouteTableId,
					UpdateRouteTableDetails: core.UpdateRouteTableDetails{
						RouteRules: updatedRules,
					},
				})
				if err != nil {
					return nil, fmt.Errorf("更新路由表失败: %w", err)
				}
			}
		}
	}

	// 4. 公有子网（允许公网 IP）
	subnetName := displayName + "-subnet"
	prohibitPublicIp := false
	createSubnetResp, err := vnClient.CreateSubnet(ctx, core.CreateSubnetRequest{
		CreateSubnetDetails: core.CreateSubnetDetails{
			CompartmentId:          &compartmentId,
			VcnId:                  vcn.Id,
			DisplayName:            &subnetName,
			CidrBlock:              &cidrBlock,
			RouteTableId:           vcn.DefaultRouteTableId,
			ProhibitPublicIpOnVnic: &prohibitPublicIp,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("创建子网失败: %w", err)
	}
	subnetResp, ok := waitForState(ctx, 30, time.Second,
		func() (core.GetSubnetResponse, error) {
			return vnClient.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: createSubnetResp.Id})
		},
		func(r core.GetSubnetResponse) bool { return r.LifecycleState == core.SubnetLifecycleStateAvailable })
	if !ok {
		return nil, fmt.Errorf("等待子网创建超时")
	}

	// 5. 默认安全列表入站放行所有协议（含全部端口），IPv6 开启时同时放行 ::/0
	if vcn.DefaultSecurityListId != nil {
		_, err = vnClient.UpdateSecurityList(ctx, core.UpdateSecurityListRequest{
			SecurityListId: vcn.DefaultSecurityListId,
			UpdateSecurityListDetails: core.UpdateSecurityListDetails{
				IngressSecurityRules: openIngressRules(isIpv6Enabled),
				EgressSecurityRules:  openEgressRules(isIpv6Enabled),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("放行安全列表失败: %w", err)
		}
	}

	info := &models.VCNInfo{
		ID:          *vcn.Id,
		DisplayName: displayName,
		CIDRBlock:   cidrBlock,
		State:       string(vcn.LifecycleState),
		Subnets: []models.SubnetInfo{
			{
				ID:          *subnetResp.Subnet.Id,
				DisplayName: subnetName,
				CIDRBlock:   cidrBlock,
				State:       string(subnetResp.Subnet.LifecycleState),
				IsPublic:    true,
			},
		},
	}
	if vcn.TimeCreated != nil {
		info.CreateTime = vcn.TimeCreated.Format("2006-01-02 15:04:05")
	}
	if subnetResp.Subnet.AvailabilityDomain != nil {
		info.Subnets[0].AvailabilityDomain = *subnetResp.Subnet.AvailabilityDomain
	}

	return info, nil
}

// openIngressRules 入站放行所有协议（TCP/UDP/ICMP 等全部放行）。
// 使用协议 all 而非 TCP 1-65535，确保 UDP 与 ICMP 不会被默认拦截。
func openIngressRules(ipv6Enabled bool) []core.IngressSecurityRule {
	allProtocol := "all"
	ipv4Cidr := "0.0.0.0/0"
	rules := []core.IngressSecurityRule{
		{Protocol: &allProtocol, Source: &ipv4Cidr},
	}
	if ipv6Enabled {
		ipv6Cidr := "::/0"
		rules = append(rules, core.IngressSecurityRule{Protocol: &allProtocol, Source: &ipv6Cidr})
	}
	return rules
}

// openEgressRules 出站放行所有协议，保持与 OCI 默认出站一致。
func openEgressRules(ipv6Enabled bool) []core.EgressSecurityRule {
	allProtocol := "all"
	ipv4Cidr := "0.0.0.0/0"
	rules := []core.EgressSecurityRule{
		{Protocol: &allProtocol, Destination: &ipv4Cidr},
	}
	if ipv6Enabled {
		ipv6Cidr := "::/0"
		rules = append(rules, core.EgressSecurityRule{Protocol: &allProtocol, Destination: &ipv6Cidr})
	}
	return rules
}

// GetPrivateIpIdForVnic 获取VNIC的私有IP的OCID
func (s *OCIService) GetPrivateIpIdForVnic(ctx context.Context, user *models.OciUser, vnicId string) (string, error) {
	vnClient, err := s.GetVirtualNetworkClient(user)
	if err != nil {
		return "", err
	}

	req := core.ListPrivateIpsRequest{
		VnicId: &vnicId,
	}

	resp, err := vnClient.ListPrivateIps(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to list private IPs: %w", err)
	}

	if len(resp.Items) == 0 {
		return "", fmt.Errorf("no private IP found for VNIC: %s", vnicId)
	}

	return *resp.Items[0].Id, nil
}

// ChangePublicIP 更改实例公网IP（参考oci-helper实现）
func (s *OCIService) ChangePublicIP(ctx context.Context, user *models.OciUser, vnicId string) (string, error) {
	vnClient, err := s.GetVirtualNetworkClient(user)
	if err != nil {
		return "", err
	}

	// 获取当前VNIC信息
	vnicReq := core.GetVnicRequest{VnicId: &vnicId}
	vnicResp, err := vnClient.GetVnic(ctx, vnicReq)
	if err != nil {
		return "", fmt.Errorf("failed to get VNIC: %w", err)
	}

	// Step 1: 如果有现有公网IP，先删除
	if vnicResp.PublicIp != nil && *vnicResp.PublicIp != "" {
		// 通过IP地址获取Public IP的OCID
		getPublicIpReq := core.GetPublicIpByIpAddressRequest{
			GetPublicIpByIpAddressDetails: core.GetPublicIpByIpAddressDetails{
				IpAddress: vnicResp.PublicIp,
			},
		}
		getPublicIpResp, err := vnClient.GetPublicIpByIpAddress(ctx, getPublicIpReq)
		if err != nil {
			return "", fmt.Errorf("failed to get public IP by address: %w", err)
		}

		// 删除现有公网IP
		deleteReq := core.DeletePublicIpRequest{PublicIpId: getPublicIpResp.Id}
		_, err = vnClient.DeletePublicIp(ctx, deleteReq)
		if err != nil {
			return "", fmt.Errorf("failed to delete public IP: %w", err)
		}
	}

	// Step 2: 获取VNIC的私有IP的OCID（这是关键，不能使用IP地址字符串）
	privateIpId, err := s.GetPrivateIpIdForVnic(ctx, user, vnicId)
	if err != nil {
		return "", fmt.Errorf("failed to get private IP ID: %w", err)
	}

	// Step 3: 创建新的临时公网IP
	displayName := "publicIp"
	createReq := core.CreatePublicIpRequest{
		CreatePublicIpDetails: core.CreatePublicIpDetails{
			CompartmentId: &user.OciTenantID,
			Lifetime:      core.CreatePublicIpDetailsLifetimeEphemeral,
			DisplayName:   &displayName,
			PrivateIpId:   &privateIpId,
		},
	}
	createResp, err := vnClient.CreatePublicIp(ctx, createReq)
	if err != nil {
		return "", fmt.Errorf("failed to create new public IP: %w", err)
	}

	if createResp.IpAddress == nil {
		return "", fmt.Errorf("new public IP address is nil")
	}

	return *createResp.IpAddress, nil
}

// CreateIpv6ByInstanceId 通过实例ID创建并附加IPv6地址
func (s *OCIService) CreateIpv6ByInstanceId(ctx context.Context, user *models.OciUser, instanceId string) (string, error) {
	computeClient, err := s.GetComputeClient(user)
	if err != nil {
		return "", err
	}

	vnClient, err := s.GetVirtualNetworkClient(user)
	if err != nil {
		return "", err
	}

	// 获取实例信息
	instance, err := s.GetInstance(ctx, user, instanceId)
	if err != nil {
		return "", fmt.Errorf("failed to get instance: %w", err)
	}

	// 获取VNIC附件
	listVnicReq := core.ListVnicAttachmentsRequest{
		CompartmentId: instance.CompartmentId,
		InstanceId:    instance.Id,
	}
	vnicResp, err := computeClient.ListVnicAttachments(ctx, listVnicReq)
	if err != nil {
		return "", fmt.Errorf("failed to list VNIC attachments: %w", err)
	}

	if len(vnicResp.Items) == 0 {
		return "", fmt.Errorf("no VNIC found for instance")
	}

	// 获取第一个VNIC
	vnicId := vnicResp.Items[0].VnicId
	if vnicId == nil {
		return "", fmt.Errorf("VNIC ID is nil")
	}

	// 获取VNIC详情
	vnicReq := core.GetVnicRequest{VnicId: vnicId}
	vnic, err := vnClient.GetVnic(ctx, vnicReq)
	if err != nil {
		return "", fmt.Errorf("failed to get VNIC: %w", err)
	}

	// 检查是否已有IPv6
	ipv6ListReq := core.ListIpv6sRequest{VnicId: vnicId}
	ipv6ListResp, err := vnClient.ListIpv6s(ctx, ipv6ListReq)
	if err == nil && len(ipv6ListResp.Items) > 0 {
		if ipv6ListResp.Items[0].IpAddress != nil {
			return "", fmt.Errorf("instance already has IPv6 address: %s", *ipv6ListResp.Items[0].IpAddress)
		}
	}

	// 获取子网信息
	subnetReq := core.GetSubnetRequest{SubnetId: vnic.SubnetId}
	subnetResp, err := vnClient.GetSubnet(ctx, subnetReq)
	if err != nil {
		return "", fmt.Errorf("failed to get subnet: %w", err)
	}

	// 获取VCN信息
	vcnReq := core.GetVcnRequest{VcnId: subnetResp.VcnId}
	vcnResp, err := vnClient.GetVcn(ctx, vcnReq)
	if err != nil {
		return "", fmt.Errorf("failed to get VCN: %w", err)
	}

	// 检查VCN是否启用了IPv6
	if len(vcnResp.Ipv6CidrBlocks) == 0 {
		return "", fmt.Errorf("VCN does not have IPv6 enabled. Please enable IPv6 on VCN first via Oracle Cloud Console")
	}

	ipv6SubnetCidr := vcnResp.Ipv6CidrBlocks[0]

	// 创建IPv6
	createIpv6Req := core.CreateIpv6Request{
		CreateIpv6Details: core.CreateIpv6Details{
			VnicId:         vnicId,
			Ipv6SubnetCidr: &ipv6SubnetCidr,
		},
	}
	createIpv6Resp, err := vnClient.CreateIpv6(ctx, createIpv6Req)
	if err != nil {
		return "", fmt.Errorf("failed to create IPv6: %w", err)
	}

	if createIpv6Resp.IpAddress == nil {
		return "", fmt.Errorf("IPv6 address is nil")
	}

	return *createIpv6Resp.IpAddress, nil
}

// GetVcnByInstanceId 根据实例ID获取VCN
func (s *OCIService) GetVcnByInstanceId(user *models.OciUser, instanceID string) (*core.Vcn, error) {
	ctx := context.Background()

	vnClient, err := s.GetVirtualNetworkClient(user)
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual network client: %w", err)
	}

	vnic, err := s.GetVnicByInstanceId(user, instanceID)
	if err != nil {
		return nil, err
	}

	// 获取子网
	subnetResp, err := vnClient.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: vnic.SubnetId})
	if err != nil {
		return nil, fmt.Errorf("failed to get subnet: %w", err)
	}

	// 获取VCN
	vcnResp, err := vnClient.GetVcn(ctx, core.GetVcnRequest{VcnId: subnetResp.VcnId})
	if err != nil {
		return nil, fmt.Errorf("failed to get VCN: %w", err)
	}

	return &vcnResp.Vcn, nil
}

// GetVnicByInstanceId 根据实例ID获取VNIC
func (s *OCIService) GetVnicByInstanceId(user *models.OciUser, instanceID string) (*core.Vnic, error) {
	ctx := context.Background()

	computeClient, err := s.GetComputeClient(user)
	if err != nil {
		return nil, fmt.Errorf("failed to get compute client: %w", err)
	}

	vnClient, err := s.GetVirtualNetworkClient(user)
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual network client: %w", err)
	}

	// 获取实例
	instResp, err := computeClient.GetInstance(ctx, core.GetInstanceRequest{InstanceId: &instanceID})
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	// 获取VNIC附件
	vnicAttachResp, err := computeClient.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
		CompartmentId: instResp.CompartmentId,
		InstanceId:    &instanceID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list VNIC attachments: %w", err)
	}

	if len(vnicAttachResp.Items) == 0 {
		return nil, fmt.Errorf("no VNIC attachment found")
	}

	// 获取VNIC
	vnicResp, err := vnClient.GetVnic(ctx, core.GetVnicRequest{VnicId: vnicAttachResp.Items[0].VnicId})
	if err != nil {
		return nil, fmt.Errorf("failed to get VNIC: %w", err)
	}

	return &vnicResp.Vnic, nil
}
