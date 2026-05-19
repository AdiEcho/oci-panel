package services

import (
	"context"

	"github.com/adiecho/oci-panel/internal/models"
	"github.com/oracle/oci-go-sdk/v65/core"
)

func (s *OCIService) ListBootVolumes(ctx context.Context, user *models.OciUser, compartmentId string) ([]models.VolumeInfo, error) {
	client, err := s.GetBlockstorageClient(user)
	if err != nil {
		return nil, err
	}

	computeClient, err := s.GetComputeClient(user)
	if err != nil {
		return nil, err
	}

	req := core.ListBootVolumesRequest{
		CompartmentId: &compartmentId,
	}

	resp, err := client.ListBootVolumes(ctx, req)
	if err != nil {
		return nil, err
	}

	volumes := make([]models.VolumeInfo, 0)
	for _, bv := range resp.Items {
		volume := models.VolumeInfo{
			ID:          *bv.Id,
			DisplayName: *bv.DisplayName,
			State:       string(bv.LifecycleState),
		}
		if bv.SizeInGBs != nil {
			volume.SizeInGBs = *bv.SizeInGBs
		}
		if bv.VpusPerGB != nil {
			volume.VpusPerGB = *bv.VpusPerGB
		}
		if bv.AvailabilityDomain != nil {
			volume.AvailabilityDomain = *bv.AvailabilityDomain
		}
		if bv.TimeCreated != nil {
			volume.CreateTime = bv.TimeCreated.Format("2006-01-02 15:04:05")
		}

		// 检查是否已附加到实例
		attachReq := core.ListBootVolumeAttachmentsRequest{
			CompartmentId:      &compartmentId,
			BootVolumeId:       bv.Id,
			AvailabilityDomain: bv.AvailabilityDomain,
		}
		attachResp, err := computeClient.ListBootVolumeAttachments(ctx, attachReq)
		if err == nil && len(attachResp.Items) > 0 {
			volume.Attached = true
			if attachResp.Items[0].InstanceId != nil {
				// 获取实例名称
				instReq := core.GetInstanceRequest{InstanceId: attachResp.Items[0].InstanceId}
				instResp, err := computeClient.GetInstance(ctx, instReq)
				if err == nil && instResp.DisplayName != nil {
					volume.InstanceName = *instResp.DisplayName
				}
			}
		}

		volumes = append(volumes, volume)
	}

	return volumes, nil
}
