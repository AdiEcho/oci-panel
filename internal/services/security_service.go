package services

import (
	"context"
	"fmt"
	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type SecurityService struct {
	ociService *OCIService
}

func NewSecurityService(ociService *OCIService) *SecurityService {
	return &SecurityService{ociService: ociService}
}

type SecurityListInfo struct {
	ID           string                     `json:"id"`
	DisplayName  string                     `json:"displayName"`
	IngressRules []core.IngressSecurityRule `json:"ingressRules"`
	EgressRules  []core.EgressSecurityRule  `json:"egressRules"`
	TimeCreated  string                     `json:"timeCreated"`
}

func (s *SecurityService) ListSecurityLists(userId string, compartmentId string, vcnId string) ([]SecurityListInfo, error) {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	client, err := s.ociService.GetVirtualNetworkClient(&user)
	if err != nil {
		return nil, err
	}

	req := core.ListSecurityListsRequest{
		CompartmentId: &compartmentId,
		VcnId:         &vcnId,
	}

	resp, err := client.ListSecurityLists(context.Background(), req)
	if err != nil {
		return nil, err
	}

	var result []SecurityListInfo
	for _, sl := range resp.Items {
		info := SecurityListInfo{
			ID:           *sl.Id,
			DisplayName:  *sl.DisplayName,
			IngressRules: sl.IngressSecurityRules,
			EgressRules:  sl.EgressSecurityRules,
			TimeCreated:  sl.TimeCreated.String(),
		}
		result = append(result, info)
	}

	return result, nil
}

func (s *SecurityService) AddIngressRule(userId string, securityListId string, protocol string, source string, tcpOptions *core.TcpOptions, udpOptions *core.UdpOptions) error {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	client, err := s.ociService.GetVirtualNetworkClient(&user)
	if err != nil {
		return err
	}

	getReq := core.GetSecurityListRequest{
		SecurityListId: &securityListId,
	}

	getResp, err := client.GetSecurityList(context.Background(), getReq)
	if err != nil {
		return err
	}

	newRule := core.IngressSecurityRule{
		Protocol: &protocol,
		Source:   &source,
	}

	if tcpOptions != nil {
		newRule.TcpOptions = tcpOptions
	}

	if udpOptions != nil {
		newRule.UdpOptions = udpOptions
	}

	ingressRules := append(getResp.IngressSecurityRules, newRule)

	updateReq := core.UpdateSecurityListRequest{
		SecurityListId: &securityListId,
		UpdateSecurityListDetails: core.UpdateSecurityListDetails{
			IngressSecurityRules: ingressRules,
		},
	}

	_, err = client.UpdateSecurityList(context.Background(), updateReq)
	return err
}

func (s *SecurityService) OpenAllPorts(userId string, securityListId string) error {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	client, err := s.ociService.GetVirtualNetworkClient(&user)
	if err != nil {
		return err
	}

	getReq := core.GetSecurityListRequest{
		SecurityListId: &securityListId,
	}

	getResp, err := client.GetSecurityList(context.Background(), getReq)
	if err != nil {
		return err
	}

	allSource := "0.0.0.0/0"
	allProtocol := "all"

	newIngressRule := core.IngressSecurityRule{
		Protocol: &allProtocol,
		Source:   &allSource,
	}

	ingressRules := append(getResp.IngressSecurityRules, newIngressRule)

	updateReq := core.UpdateSecurityListRequest{
		SecurityListId: &securityListId,
		UpdateSecurityListDetails: core.UpdateSecurityListDetails{
			IngressSecurityRules: ingressRules,
		},
	}

	_, err = client.UpdateSecurityList(context.Background(), updateReq)
	return err
}

func (s *SecurityService) ClearIngressRules(userId string, securityListId string) error {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	client, err := s.ociService.GetVirtualNetworkClient(&user)
	if err != nil {
		return err
	}

	updateReq := core.UpdateSecurityListRequest{
		SecurityListId: &securityListId,
		UpdateSecurityListDetails: core.UpdateSecurityListDetails{
			IngressSecurityRules: []core.IngressSecurityRule{},
		},
	}

	_, err = client.UpdateSecurityList(context.Background(), updateReq)
	return err
}
