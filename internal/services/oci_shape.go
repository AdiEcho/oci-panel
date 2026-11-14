package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/adiecho/oci-panel/internal/models"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// ShapeInfo 可创建实例的规格（对齐 Oracle 控制台创建实例时的规格卡片）。
type ShapeInfo struct {
	Shape                  string      `json:"shape"`
	Architecture           string      `json:"architecture"`
	ProcessorDescription   string      `json:"processorDescription"`
	Ocpus                  float64     `json:"ocpus"`
	MemoryInGBs            float64     `json:"memoryInGBs"`
	NetworkBandwidthInGbps float64     `json:"networkBandwidthInGbps"`
	IsFlex                 bool        `json:"isFlex"`
	MinOcpus               float64     `json:"minOcpus"`
	MaxOcpus               float64     `json:"maxOcpus"`
	MinMemoryInGBs         float64     `json:"minMemoryInGBs"`
	MaxMemoryInGBs         float64     `json:"maxMemoryInGBs"`
	LocalDisks             int         `json:"localDisks"`
	LocalDiskSizeInGBs     float64     `json:"localDiskSizeInGBs"`
	Gpus                   int         `json:"gpus"`
	GpuDescription         string      `json:"gpuDescription"`
	FreeTier               bool        `json:"freeTier"`
	FreeTierMaxOcpus       float64     `json:"freeTierMaxOcpus"`
	FreeTierMaxMemoryInGBs float64     `json:"freeTierMaxMemoryInGBs"`
	Price                  *ShapePrice `json:"price,omitempty"`
}

// alwaysFreeShapes 永久免费规格及其免费额度上限。E2.1.Micro 为固定规格，
// 上限即默认规格；A1.Flex 免费额度为 4 OCPU / 24 GB。
var alwaysFreeShapes = map[string]struct {
	MaxOcpus float64
	MaxMem   float64
}{
	"VM.Standard.E2.1.Micro": {MaxOcpus: 1, MaxMem: 1},
	"VM.Standard.A1.Flex":    {MaxOcpus: 4, MaxMem: 24},
}

// ListShapes 获取区域内容器可创建的实例规格。
func (s *OCIService) ListShapes(ctx context.Context, user *models.OciUser, region string) ([]ShapeInfo, error) {
	computeClient, err := s.GetComputeClientForRegion(user, region)
	if err != nil {
		return nil, fmt.Errorf("获取计算客户端失败: %w", err)
	}

	compartmentId := user.OciTenantID
	items, err := paginate(func(page *string) ([]core.Shape, *string, error) {
		resp, err := computeClient.ListShapes(ctx, core.ListShapesRequest{
			CompartmentId: &compartmentId,
			Page:          page,
		})
		if err != nil {
			return nil, nil, err
		}
		return resp.Items, resp.OpcNextPage, nil
	})
	if err != nil {
		return nil, fmt.Errorf("获取规格列表失败: %w", err)
	}

	shapes := make([]ShapeInfo, 0, len(items))
	for _, item := range items {
		if item.Shape == nil {
			continue
		}
		info := ShapeInfo{
			Shape:                  *item.Shape,
			Architecture:           shapeArchitecture(item),
			ProcessorDescription:   stringValue(item.ProcessorDescription),
			Ocpus:                  floatValue(item.Ocpus),
			MemoryInGBs:            floatValue(item.MemoryInGBs),
			NetworkBandwidthInGbps: floatValue(item.NetworkingBandwidthInGbps),
			IsFlex:                 boolValue(item.IsFlexible),
			MinOcpus:               floatValue(item.Ocpus),
			MaxOcpus:               floatValue(item.Ocpus),
			MinMemoryInGBs:         floatValue(item.MemoryInGBs),
			MaxMemoryInGBs:         floatValue(item.MemoryInGBs),
			LocalDisks:             intValue(item.LocalDisks),
			LocalDiskSizeInGBs:     floatValue(item.LocalDisksTotalSizeInGBs),
			Gpus:                   intValue(item.Gpus),
			GpuDescription:         stringValue(item.GpuDescription),
		}
		if item.OcpuOptions != nil {
			info.MinOcpus = floatValue(item.OcpuOptions.Min)
			info.MaxOcpus = floatValue(item.OcpuOptions.Max)
		}
		if item.MemoryOptions != nil {
			info.MinMemoryInGBs = floatValue(item.MemoryOptions.MinInGBs)
			info.MaxMemoryInGBs = floatValue(item.MemoryOptions.MaxInGBs)
		}
		if free, ok := alwaysFreeShapes[info.Shape]; ok {
			info.FreeTier = true
			info.FreeTierMaxOcpus = free.MaxOcpus
			info.FreeTierMaxMemoryInGBs = free.MaxMem
		}
		shapes = append(shapes, info)
	}

	// Oracle 可能对同一规格返回多条（例如按可用域重复），按规格名去重。
	shapes = dedupeShapes(shapes)

	// 免费规格排前，其次按名称稳定排序，保证卡片顺序可预期。
	sort.SliceStable(shapes, func(i, j int) bool {
		if shapes[i].FreeTier != shapes[j].FreeTier {
			return shapes[i].FreeTier
		}
		return shapes[i].Shape < shapes[j].Shape
	})

	return shapes, nil
}

func dedupeShapes(shapes []ShapeInfo) []ShapeInfo {
	if len(shapes) == 0 {
		return shapes
	}
	seen := make(map[string]struct{}, len(shapes))
	result := shapes[:0]
	for _, shape := range shapes {
		if shape.Shape == "" {
			continue
		}
		if _, ok := seen[shape.Shape]; ok {
			continue
		}
		seen[shape.Shape] = struct{}{}
		result = append(result, shape)
	}
	return result
}

func shapeArchitecture(shape core.Shape) string {
	name := ""
	if shape.Shape != nil {
		name = *shape.Shape
	}
	processor := strings.ToLower(stringValue(shape.ProcessorDescription))
	lowerName := strings.ToLower(name)

	// Ampere 为 ARM；其余 x86 规格在面板内统一归为 AMD，用于镜像过滤的向后兼容。
	if strings.Contains(lowerName, ".a1.") || strings.Contains(lowerName, ".a2.") ||
		strings.Contains(lowerName, ".a3.") || strings.Contains(lowerName, ".a4.") ||
		strings.Contains(processor, "ampere") || strings.Contains(processor, "arm") {
		return "ARM"
	}
	return "AMD"
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func floatValue(v *float32) float64 {
	if v == nil {
		return 0
	}
	return float64(*v)
}

func intValue(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func boolValue(v *bool) bool {
	return v != nil && *v
}
