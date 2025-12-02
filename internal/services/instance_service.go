package services

import (
	"context"
	"fmt"
	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
)

type InstanceService struct {
	ociService *OCIService
}

func NewInstanceService(ociService *OCIService) *InstanceService {
	return &InstanceService{ociService: ociService}
}

type InstanceInfo struct {
	ID                 string `json:"id"`
	DisplayName        string `json:"displayName"`
	State              string `json:"state"`
	AvailabilityDomain string `json:"availabilityDomain"`
	Shape              string `json:"shape"`
	TimeCreated        string `json:"timeCreated"`
	PublicIp           string `json:"publicIp"`
	PrivateIp          string `json:"privateIp"`
}

func (s *InstanceService) ListInstances(userId string, compartmentId string) ([]InstanceInfo, error) {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	instances, err := s.ociService.ListInstances(context.Background(), &user, compartmentId)
	if err != nil {
		return nil, err
	}

	var result []InstanceInfo
	for _, inst := range instances {
		info := InstanceInfo{
			ID:                 *inst.Id,
			DisplayName:        *inst.DisplayName,
			State:              string(inst.LifecycleState),
			AvailabilityDomain: *inst.AvailabilityDomain,
			Shape:              *inst.Shape,
			TimeCreated:        inst.TimeCreated.String(),
		}
		result = append(result, info)
	}

	return result, nil
}

func (s *InstanceService) StartInstance(userId string, instanceId string) error {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	return s.ociService.InstanceAction(context.Background(), &user, instanceId, "START")
}

func (s *InstanceService) StopInstance(userId string, instanceId string) error {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	return s.ociService.InstanceAction(context.Background(), &user, instanceId, "STOP")
}

func (s *InstanceService) RebootInstance(userId string, instanceId string) error {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	return s.ociService.InstanceAction(context.Background(), &user, instanceId, "RESET")
}

func (s *InstanceService) TerminateInstance(userId string, instanceId string) error {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	return s.ociService.TerminateInstance(context.Background(), &user, instanceId)
}

func (s *InstanceService) UpdateInstanceName(userId string, instanceId string, displayName string) error {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	return s.ociService.UpdateInstance(context.Background(), &user, instanceId, displayName)
}
