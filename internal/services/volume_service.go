package services

import (
	"context"
	"fmt"
	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/oracle/oci-go-sdk/v65/core"
)

type VolumeService struct {
	ociService *OCIService
}

func NewVolumeService(ociService *OCIService) *VolumeService {
	return &VolumeService{ociService: ociService}
}

type BootVolumeInfo struct {
	ID                 string `json:"id"`
	DisplayName        string `json:"displayName"`
	State              string `json:"state"`
	SizeInGBs          int64  `json:"sizeInGBs"`
	AvailabilityDomain string `json:"availabilityDomain"`
	TimeCreated        string `json:"timeCreated"`
	VpusPerGB          int64  `json:"vpusPerGB"`
}

func (s *VolumeService) ListBootVolumes(userId string, compartmentId string, availabilityDomain string) ([]BootVolumeInfo, error) {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	client, err := s.ociService.GetBlockstorageClient(&user)
	if err != nil {
		return nil, err
	}

	req := core.ListBootVolumesRequest{
		CompartmentId:      &compartmentId,
		AvailabilityDomain: &availabilityDomain,
	}

	resp, err := client.ListBootVolumes(context.Background(), req)
	if err != nil {
		return nil, err
	}

	var result []BootVolumeInfo
	for _, bv := range resp.Items {
		info := BootVolumeInfo{
			ID:                 *bv.Id,
			DisplayName:        *bv.DisplayName,
			State:              string(bv.LifecycleState),
			SizeInGBs:          *bv.SizeInGBs,
			AvailabilityDomain: *bv.AvailabilityDomain,
			TimeCreated:        bv.TimeCreated.String(),
		}
		if bv.VpusPerGB != nil {
			info.VpusPerGB = *bv.VpusPerGB
		}
		result = append(result, info)
	}

	return result, nil
}

func (s *VolumeService) UpdateBootVolume(userId string, bootVolumeId string, sizeInGBs int64, displayName string) error {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	client, err := s.ociService.GetBlockstorageClient(&user)
	if err != nil {
		return err
	}

	details := core.UpdateBootVolumeDetails{}

	if sizeInGBs > 0 {
		details.SizeInGBs = &sizeInGBs
	}

	if displayName != "" {
		details.DisplayName = &displayName
	}

	req := core.UpdateBootVolumeRequest{
		BootVolumeId:            &bootVolumeId,
		UpdateBootVolumeDetails: details,
	}

	_, err = client.UpdateBootVolume(context.Background(), req)
	return err
}

func (s *VolumeService) DeleteBootVolume(userId string, bootVolumeId string) error {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	client, err := s.ociService.GetBlockstorageClient(&user)
	if err != nil {
		return err
	}

	req := core.DeleteBootVolumeRequest{
		BootVolumeId: &bootVolumeId,
	}

	_, err = client.DeleteBootVolume(context.Background(), req)
	return err
}

type BlockVolumeInfo struct {
	ID                 string `json:"id"`
	DisplayName        string `json:"displayName"`
	State              string `json:"state"`
	SizeInGBs          int64  `json:"sizeInGBs"`
	AvailabilityDomain string `json:"availabilityDomain"`
	TimeCreated        string `json:"timeCreated"`
}

func (s *VolumeService) ListBlockVolumes(userId string, compartmentId string) ([]BlockVolumeInfo, error) {
	var user models.OciUser
	if err := database.GetDB().Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	client, err := s.ociService.GetBlockstorageClient(&user)
	if err != nil {
		return nil, err
	}

	req := core.ListVolumesRequest{
		CompartmentId: &compartmentId,
	}

	resp, err := client.ListVolumes(context.Background(), req)
	if err != nil {
		return nil, err
	}

	var result []BlockVolumeInfo
	for _, vol := range resp.Items {
		info := BlockVolumeInfo{
			ID:                 *vol.Id,
			DisplayName:        *vol.DisplayName,
			State:              string(vol.LifecycleState),
			SizeInGBs:          *vol.SizeInGBs,
			AvailabilityDomain: *vol.AvailabilityDomain,
			TimeCreated:        vol.TimeCreated.String(),
		}
		result = append(result, info)
	}

	return result, nil
}
