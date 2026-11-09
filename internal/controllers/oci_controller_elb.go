package controllers

import (
	"context"
	"net/http"

	"github.com/adiecho/oci-panel/internal/models"
	"github.com/adiecho/oci-panel/internal/services"
	"github.com/gin-gonic/gin"
)

// oci_controller_elb.go — 500Mbps 所用 ELB（OCI 网络负载均衡器 NLB）的独立池管理。
// 支持在配置详情里先创建 ELB、之后再分配给实例，避免只能随「开启500Mbps」顺带创建。

// ListElbPools 列出当前配置纳管的 ELB 池
func (oc *OciController) ListElbPools(c *gin.Context) {
	var req GetConfigDetailsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	user, ok := oc.loadUser(c, req.ConfigID)
	if !ok {
		return
	}

	bwService := services.GetBandwidth500Service(oc.ociService)
	pools, err := bwService.ListNlbPools(context.Background(), &user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(pools, "Success"))
}

type CreateElbPoolRequest struct {
	ConfigID string `json:"configId" binding:"required"`
	VcnId    string `json:"vcnId" binding:"required"`
	SubnetId string `json:"subnetId"`
	NlbName  string `json:"nlbName"`
}

// CreateElbPool 预建一个受管共享 ELB（不绑定实例）
func (oc *OciController) CreateElbPool(c *gin.Context) {
	var req CreateElbPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	user, ok := oc.loadUser(c, req.ConfigID)
	if !ok {
		return
	}

	bwService := services.GetBandwidth500Service(oc.ociService)
	pool, err := bwService.CreateNlbPool(context.Background(), &user, services.CreatePoolRequest{
		VcnID:    req.VcnId,
		SubnetID: req.SubnetId,
		NlbName:  req.NlbName,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(pool, "ELB 创建成功"))
}

type DeleteElbPoolRequest struct {
	ConfigID string `json:"configId" binding:"required"`
	NlbId    string `json:"nlbId" binding:"required"`
}

// DeleteElbPool 删除未被任何实例占用的 ELB
func (oc *OciController) DeleteElbPool(c *gin.Context) {
	var req DeleteElbPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	user, ok := oc.loadUser(c, req.ConfigID)
	if !ok {
		return
	}

	bwService := services.GetBandwidth500Service(oc.ociService)
	if err := bwService.DeleteNlbPool(context.Background(), &user, req.NlbId); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "ELB 已删除"))
}
