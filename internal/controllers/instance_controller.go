package controllers

import (
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/adiecho/oci-panel/internal/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

type InstanceController struct {
	instanceService *services.InstanceService
}

func NewInstanceController(instanceService *services.InstanceService) *InstanceController {
	return &InstanceController{instanceService: instanceService}
}

type ListInstancesRequest struct {
	UserId        string `json:"userId" binding:"required"`
	CompartmentId string `json:"compartmentId" binding:"required"`
}

func (ic *InstanceController) ListInstances(c *gin.Context) {
	var req ListInstancesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	instances, err := ic.instanceService.ListInstances(req.UserId, req.CompartmentId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(instances, "获取实例列表成功"))
}

type InstanceActionRequest struct {
	UserId     string `json:"userId" binding:"required"`
	InstanceId string `json:"instanceId" binding:"required"`
}

func (ic *InstanceController) StartInstance(c *gin.Context) {
	var req InstanceActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	if err := ic.instanceService.StartInstance(req.UserId, req.InstanceId); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "实例启动成功"))
}

func (ic *InstanceController) StopInstance(c *gin.Context) {
	var req InstanceActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	if err := ic.instanceService.StopInstance(req.UserId, req.InstanceId); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "实例停止成功"))
}

func (ic *InstanceController) RebootInstance(c *gin.Context) {
	var req InstanceActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	if err := ic.instanceService.RebootInstance(req.UserId, req.InstanceId); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "实例重启成功"))
}

func (ic *InstanceController) TerminateInstance(c *gin.Context) {
	var req InstanceActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	if err := ic.instanceService.TerminateInstance(req.UserId, req.InstanceId); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "实例删除成功"))
}

type UpdateInstanceNameRequest struct {
	UserId      string `json:"userId" binding:"required"`
	InstanceId  string `json:"instanceId" binding:"required"`
	DisplayName string `json:"displayName" binding:"required"`
}

func (ic *InstanceController) UpdateInstanceName(c *gin.Context) {
	var req UpdateInstanceNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	if err := ic.instanceService.UpdateInstanceName(req.UserId, req.InstanceId, req.DisplayName); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "实例名称更新成功"))
}
