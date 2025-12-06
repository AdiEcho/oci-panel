package services

import (
	"context"
	"fmt"

	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type IpService struct {
	ociService *OCIService
}

func NewIpService(ociService *OCIService) *IpService {
	return &IpService{ociService: ociService}
}

func (s *IpService) GetVnicAttachments(userId string, compartmentId string, instanceId string) ([]core.VnicAttachment, error) {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	client, err := s.ociService.GetComputeClient(&user)
	if err != nil {
		return nil, err
	}

	req := core.ListVnicAttachmentsRequest{
		CompartmentId: &compartmentId,
		InstanceId:    &instanceId,
	}

	resp, err := client.ListVnicAttachments(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return resp.Items, nil
}

func (s *IpService) GetVnic(userId string, vnicId string) (*core.Vnic, error) {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	client, err := s.ociService.GetVirtualNetworkClient(&user)
	if err != nil {
		return nil, err
	}

	req := core.GetVnicRequest{
		VnicId: &vnicId,
	}

	resp, err := client.GetVnic(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return &resp.Vnic, nil
}

func (s *IpService) ChangePublicIp(userId string, instanceId string, compartmentId string) (string, error) {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}

	vnicAttachments, err := s.GetVnicAttachments(userId, compartmentId, instanceId)
	if err != nil {
		return "", err
	}

	if len(vnicAttachments) == 0 {
		return "", fmt.Errorf("no vnic attachments found")
	}

	vnicId := vnicAttachments[0].VnicId
	if vnicId == nil {
		return "", fmt.Errorf("vnic id is nil")
	}

	vnic, err := s.GetVnic(userId, *vnicId)
	if err != nil {
		return "", err
	}

	if vnic.PublicIp == nil {
		return "", fmt.Errorf("no public ip found")
	}

	networkClient, err := s.ociService.GetVirtualNetworkClient(&user)
	if err != nil {
		return "", err
	}

	listReq := core.ListPublicIpsRequest{
		Scope:         core.ListPublicIpsScopeRegion,
		CompartmentId: &compartmentId,
	}

	listResp, err := networkClient.ListPublicIps(context.Background(), listReq)
	if err != nil {
		return "", err
	}

	var publicIpId *string
	for _, pip := range listResp.Items {
		if pip.IpAddress != nil && *pip.IpAddress == *vnic.PublicIp {
			publicIpId = pip.Id
			break
		}
	}

	if publicIpId == nil {
		return "", fmt.Errorf("public ip not found")
	}

	deleteReq := core.DeletePublicIpRequest{
		PublicIpId: publicIpId,
	}

	_, err = networkClient.DeletePublicIp(context.Background(), deleteReq)
	if err != nil {
		return "", fmt.Errorf("failed to delete old public ip: %w", err)
	}

	privateIpId := vnic.PrivateIp
	if privateIpId == nil {
		return "", fmt.Errorf("private ip id is nil")
	}

	lifetime := core.CreatePublicIpDetailsLifetimeEphemeral
	createReq := core.CreatePublicIpRequest{
		CreatePublicIpDetails: core.CreatePublicIpDetails{
			CompartmentId: &compartmentId,
			Lifetime:      lifetime,
			PrivateIpId:   privateIpId,
		},
	}

	createResp, err := networkClient.CreatePublicIp(context.Background(), createReq)
	if err != nil {
		return "", fmt.Errorf("failed to create new public ip: %w", err)
	}

	if createResp.IpAddress == nil {
		return "", fmt.Errorf("new public ip address is nil")
	}

	return *createResp.IpAddress, nil
}

func (s *IpService) AttachIpv6(userId string, vnicId string, ipv6SubnetCidr string) error {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	networkClient, err := s.ociService.GetVirtualNetworkClient(&user)
	if err != nil {
		return err
	}

	req := core.CreateIpv6Request{
		CreateIpv6Details: core.CreateIpv6Details{
			VnicId:         &vnicId,
			Ipv6SubnetCidr: &ipv6SubnetCidr,
		},
	}

	_, err = networkClient.CreateIpv6(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to attach ipv6: %w", err)
	}

	return nil
}
