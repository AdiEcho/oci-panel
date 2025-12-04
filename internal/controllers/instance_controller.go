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

type ChangeIPRequest struct {
	UserId     string `json:"userId" binding:"required"`
	InstanceId string `json:"instanceId" binding:"required"`
}

func (ic *InstanceController) ChangePublicIP(c *gin.Context) {
	var req ChangeIPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	newIP, err := ic.instanceService.ChangePublicIP(req.UserId, req.InstanceId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(map[string]string{"newIP": newIP}, "IP更改成功"))
}

type UpdateInstanceConfigRequest struct {
	UserId      string  `json:"userId" binding:"required"`
	InstanceId  string  `json:"instanceId" binding:"required"`
	Ocpus       float32 `json:"ocpus" binding:"required,gt=0"`
	MemoryInGBs float32 `json:"memoryInGBs" binding:"required,gt=0"`
}

func (ic *InstanceController) UpdateInstanceConfig(c *gin.Context) {
	var req UpdateInstanceConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	if err := ic.instanceService.UpdateInstanceConfig(req.UserId, req.InstanceId, req.Ocpus, req.MemoryInGBs); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "实例配置更新成功"))
}

type UpdateBootVolumeRequest struct {
	UserId     string `json:"userId" binding:"required"`
	InstanceId string `json:"instanceId" binding:"required"`
	SizeInGBs  int64  `json:"sizeInGBs" binding:"required,gt=0"`
	VpusPerGB  int64  `json:"vpusPerGB" binding:"required,gt=0"`
}

func (ic *InstanceController) UpdateBootVolume(c *gin.Context) {
	var req UpdateBootVolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	if err := ic.instanceService.UpdateBootVolumeConfig(req.UserId, req.InstanceId, req.SizeInGBs, req.VpusPerGB); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "引导卷配置更新成功"))
}

type CreateCloudShellRequest struct {
	UserId     string `json:"userId" binding:"required"`
	InstanceId string `json:"instanceId" binding:"required"`
	PublicKey  string `json:"publicKey" binding:"required"`
}

func (ic *InstanceController) CreateCloudShell(c *gin.Context) {
	var req CreateCloudShellRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	result, err := ic.instanceService.CreateCloudShellConnection(req.UserId, req.InstanceId, req.PublicKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(result, "Cloud Shell连接创建成功"))
}

type AttachIPv6Request struct {
	UserId     string `json:"userId" binding:"required"`
	InstanceId string `json:"instanceId" binding:"required"`
}

func (ic *InstanceController) AttachIPv6(c *gin.Context) {
	var req AttachIPv6Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	ipv6Address, err := ic.instanceService.AttachIPv6(req.UserId, req.InstanceId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(map[string]string{"ipv6": ipv6Address}, "IPv6附加成功"))
}

type AutoRescueRequest struct {
	UserId       string `json:"userId" binding:"required"`
	InstanceId   string `json:"instanceId" binding:"required"`
	InstanceName string `json:"instanceName"`
	KeepBackup   bool   `json:"keepBackup"`
}

func (ic *InstanceController) AutoRescue(c *gin.Context) {
	var req AutoRescueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	// 异步执行救援任务
	go func() {
		progressChan := make(chan services.AutoRescueProgress, 10)
		go func() {
			for progress := range progressChan {
				// 进度可通过WebSocket推送，这里仅记录日志
				_ = progress
			}
		}()

		err := ic.instanceService.AutoRescue(req.UserId, req.InstanceId, req.InstanceName, req.KeepBackup, progressChan)
		close(progressChan)
		if err != nil {
			// 记录错误日志
			_ = err
		}
	}()

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "自动救援任务已启动，请等待完成"))
}

type Enable500MbpsRequest struct {
	UserId     string `json:"userId" binding:"required"`
	InstanceId string `json:"instanceId" binding:"required"`
	SSHPort    int    `json:"sshPort"`
}

func (ic *InstanceController) Enable500Mbps(c *gin.Context) {
	var req Enable500MbpsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	if req.SSHPort == 0 {
		req.SSHPort = 22
	}

	// 异步执行
	go func() {
		publicIP, err := ic.instanceService.Enable500Mbps(req.UserId, req.InstanceId, req.SSHPort)
		if err != nil {
			_ = err
		} else {
			_ = publicIP
		}
	}()

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "500Mbps开启任务已启动，请等待完成"))
}

type Disable500MbpsRequest struct {
	UserId      string `json:"userId" binding:"required"`
	InstanceId  string `json:"instanceId" binding:"required"`
	RetainNatGw bool   `json:"retainNatGw"`
	RetainNlb   bool   `json:"retainNlb"`
}

func (ic *InstanceController) Disable500Mbps(c *gin.Context) {
	var req Disable500MbpsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	// 异步执行
	go func() {
		err := ic.instanceService.Disable500Mbps(req.UserId, req.InstanceId, req.RetainNatGw, req.RetainNlb)
		if err != nil {
			_ = err
		}
	}()

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "500Mbps关闭任务已启动，请等待完成"))
}
