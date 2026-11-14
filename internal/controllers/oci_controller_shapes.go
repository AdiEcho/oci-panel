package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/adiecho/oci-panel/internal/services"
	"github.com/gin-gonic/gin"
)

// oci_controller_shapes.go — 可创建实例规格查询（按区域 + 租户维度缓存）。
// 预设配置页用它渲染与 Oracle 控制台一致的规格卡片。

// ListShapesRequest 获取规格列表请求
type ListShapesRequest struct {
	ConfigID   string `json:"configId" binding:"required"`
	Region     string `json:"region" binding:"required"`
	ClearCache bool   `json:"clearCache"`
}

// ListShapes 获取区域可创建的实例规格
func (oc *OciController) ListShapes(c *gin.Context) {
	var req ListShapesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	user, ok := oc.loadUser(c, req.ConfigID)
	if !ok {
		return
	}
	db := database.GetDB()
	cacheKey := req.Region + "_" + user.OciTenantID

	if !req.ClearCache {
		var cache models.OciShapeCache
		if err := db.Where("id = ?", cacheKey).First(&cache).Error; err == nil && cache.ShapesData != "" {
			var shapes []services.ShapeInfo
			if json.Unmarshal([]byte(cache.ShapesData), &shapes) == nil {
				oc.ociService.AttachPrices(shapes)
				c.JSON(http.StatusOK, models.SuccessResponse(shapes, "获取规格列表成功(缓存)"))
				return
			}
		}
	}

	ctx := context.Background()
	shapes, err := oc.ociService.ListShapes(ctx, &user, req.Region)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	oc.ociService.AttachPrices(shapes)
	shapesJSON, _ := json.Marshal(shapes)
	var cache models.OciShapeCache
	if err := db.Where("id = ?", cacheKey).First(&cache).Error; err != nil {
		cache = models.OciShapeCache{ID: cacheKey, Region: req.Region}
	}
	cache.ShapesData = string(shapesJSON)
	cache.UpdateTime = time.Now()
	db.Save(&cache)

	c.JSON(http.StatusOK, models.SuccessResponse(shapes, "获取规格列表成功"))
}
