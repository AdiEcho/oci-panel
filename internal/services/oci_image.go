package services

import (
	"context"
	"fmt"

	"github.com/adiecho/oci-panel/internal/models"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// ImageInfo 镜像信息
type ImageInfo struct {
	ID                     string `json:"id"`
	DisplayName            string `json:"displayName"`
	OperatingSystem        string `json:"operatingSystem"`
	OperatingSystemVersion string `json:"operatingSystemVersion"`
	SizeInMBs              int64  `json:"sizeInMBs"`
	TimeCreated            string `json:"timeCreated"`
}

// ListImages 获取可用镜像列表。shape 非空时优先按规格过滤（Flex 与非 Flex 镜像集合不同），
// 否则回退到按架构映射的默认免费规格。
func (s *OCIService) ListImages(ctx context.Context, user *models.OciUser, region, architecture, shape string) ([]ImageInfo, error) {
	computeClient, err := s.GetComputeClientForRegion(user, region)
	if err != nil {
		return nil, fmt.Errorf("获取计算客户端失败: %w", err)
	}

	compartmentId := user.OciTenantID

	// 确定Shape
	resolvedShape := shape
	if resolvedShape == "" {
		resolvedShape = "VM.Standard.A1.Flex"
		if architecture == "AMD" {
			resolvedShape = "VM.Standard.E2.1.Micro"
		}
	}

	// 全量分页：镜像可能有数百条，此前只取首页会截断。
	imgItems, err := paginate(func(page *string) ([]core.Image, *string, error) {
		resp, err := computeClient.ListImages(ctx, core.ListImagesRequest{
			CompartmentId: &compartmentId,
			Shape:         &resolvedShape,
			SortBy:        core.ListImagesSortByTimecreated,
			SortOrder:     core.ListImagesSortOrderDesc,
			Page:          page,
		})
		if err != nil {
			return nil, nil, err
		}
		return resp.Items, resp.OpcNextPage, nil
	})
	if err != nil {
		return nil, fmt.Errorf("获取镜像列表失败: %w", err)
	}

	var images []ImageInfo
	for _, img := range imgItems {
		info := ImageInfo{
			ID:                     *img.Id,
			DisplayName:            *img.DisplayName,
			OperatingSystem:        *img.OperatingSystem,
			OperatingSystemVersion: *img.OperatingSystemVersion,
		}
		if img.SizeInMBs != nil {
			info.SizeInMBs = *img.SizeInMBs
		}
		if img.TimeCreated != nil {
			info.TimeCreated = img.TimeCreated.Format("2006-01-02 15:04:05")
		}
		images = append(images, info)
	}

	return images, nil
}
