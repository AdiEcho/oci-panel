package controllers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/adiecho/oci-panel/internal/models"
	"github.com/adiecho/oci-panel/internal/services"
	"github.com/adiecho/oci-panel/internal/util"
	"github.com/gin-gonic/gin"
)

type InstanceController struct {
	instanceService *services.InstanceService
	wsService       *services.WebSocketService
}

func NewInstanceController(instanceService *services.InstanceService, wsService *services.WebSocketService) *InstanceController {
	return &InstanceController{instanceService: instanceService, wsService: wsService}
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
	AutoRestart bool    `json:"autoRestart"` // 是否自动重启实例，默认false
}

func (ic *InstanceController) UpdateInstanceConfig(c *gin.Context) {
	var req UpdateInstanceConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	if err := ic.instanceService.UpdateInstanceConfig(req.UserId, req.InstanceId, req.Ocpus, req.MemoryInGBs, req.AutoRestart); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	msg := "实例配置更新成功"
	if req.AutoRestart {
		msg = "实例配置更新成功，正在重启实例"
	}
	c.JSON(http.StatusOK, models.SuccessResponse(nil, msg))
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

type UpdateBootVolumeByIdRequest struct {
	UserId       string `json:"userId" binding:"required"`
	BootVolumeId string `json:"bootVolumeId" binding:"required"`
	SizeInGBs    int64  `json:"sizeInGBs" binding:"required,gt=0"`
	VpusPerGB    int64  `json:"vpusPerGB" binding:"required,gt=0"`
}

func (ic *InstanceController) UpdateBootVolumeById(c *gin.Context) {
	var req UpdateBootVolumeByIdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	if err := ic.instanceService.UpdateBootVolumeById(req.UserId, req.BootVolumeId, req.SizeInGBs, req.VpusPerGB); err != nil {
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
	util.Go("AutoRescue", func() {
		progressChan := make(chan services.AutoRescueProgress, 10)
		util.Go("AutoRescue.progress", func() {
			for progress := range progressChan {
				ic.wsService.SendInfo(fmt.Sprintf("AutoRescue [%s] Step %d/%d: %s", req.InstanceId, progress.Step, progress.TotalSteps, progress.Message))
			}
		})

		err := ic.instanceService.AutoRescue(req.UserId, req.InstanceId, req.InstanceName, req.KeepBackup, progressChan)
		close(progressChan)
		if err != nil {
			log.Printf("AutoRescue failed for instance %s: %v", req.InstanceId, err)
			ic.wsService.SendError(fmt.Sprintf("AutoRescue failed for instance %s: %v", req.InstanceId, err))
		} else {
			ic.wsService.SendSuccess(fmt.Sprintf("AutoRescue completed for instance %s", req.InstanceId))
		}
	})

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "自动救援任务已启动，请等待完成"))
}

// Enable500MbpsRequest 一键开启500Mbps请求
type Enable500MbpsRequest struct {
	UserId     string                        `json:"userId" binding:"required"`
	InstanceId string                        `json:"instanceId" binding:"required"`
	Mode       string                        `json:"mode"`     // dedicated, shared-ports
	SshPort    *int                          `json:"sshPort"`  // 实例健康检查端口，默认 22
	Mappings   []models.Bandwidth500Endpoint `json:"mappings"` // 共享模式映射列表
	NlbId      string                        `json:"nlbId"`    // 指定复用 NLB
}

// Enable500Mbps 一键开启下行500Mbps
// 支持独享全端口与共享端口两种模式；共享模式复用同一 NLB，极大规避区域 max-nlb-flexible-count 配额限制
func (ic *InstanceController) Enable500Mbps(c *gin.Context) {
	var req Enable500MbpsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	sshPort := 22
	if req.SshPort != nil && *req.SshPort > 0 && *req.SshPort <= 65535 {
		sshPort = *req.SshPort
	}
	mode := req.Mode
	if mode == "" {
		mode = services.ModeDedicated
	}

	// 异步执行
	util.Go("Enable500Mbps", func() {
		publicIP, err := ic.instanceService.Enable500MbpsWithOptions(req.UserId, services.EnableRequest{
			InstanceID: req.InstanceId,
			Mode:       mode,
			SSHPort:    sshPort,
			Mappings:   req.Mappings,
			NLBID:      req.NlbId,
		})
		if err != nil {
			log.Printf("Enable500Mbps failed for instance %s: %v", req.InstanceId, err)
			ic.wsService.SendError(fmt.Sprintf("Enable500Mbps 失败 [%s]: %v", req.InstanceId, err))
		} else {
			log.Printf("Enable500Mbps succeeded for instance %s, new IP: %s", req.InstanceId, publicIP)
			ic.wsService.SendSuccess(fmt.Sprintf("Enable500Mbps 成功 [%s]，NLB 访问 IP: %s", req.InstanceId, publicIP))
		}
	})

	c.JSON(http.StatusOK, models.SuccessResponse(map[string]interface{}{
		"warning": "开启后实例公网流量将通过负载均衡器/NAT网关转发。此操作仅支持 VM.Standard.E2.1.Micro 实例。",
	}, "500Mbps 开启任务已启动，请稍候在实例列表中查看 NLB 访问入口..."))
}

// Disable500MbpsRequest 一键关闭500Mbps请求
type Disable500MbpsRequest struct {
	UserId      string `json:"userId" binding:"required"`
	InstanceId  string `json:"instanceId" binding:"required"`
	RetainNatGw *bool  `json:"retainNatGw"` // 指针布尔，未传默认 true（保留共享 NAT）
	RetainNlb   *bool  `json:"retainNlb"`   // 指针布尔，未传默认 true（保留共享 NLB）
}

// Disable500Mbps 一键关闭下行500Mbps
// 安全解绑实例，恢复原始路由与 SkipSourceDestCheck，受保护的共享资源绝不误删
func (ic *InstanceController) Disable500Mbps(c *gin.Context) {
	var req Disable500MbpsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	// 默认安全保留
	retainNatGw := true
	if req.RetainNatGw != nil {
		retainNatGw = *req.RetainNatGw
	}
	retainNlb := true
	if req.RetainNlb != nil {
		retainNlb = *req.RetainNlb
	}

	// 异步执行
	util.Go("Disable500Mbps", func() {
		err := ic.instanceService.Disable500MbpsWithOptions(req.UserId, services.DisableRequest{
			InstanceID:  req.InstanceId,
			RetainNatGw: &retainNatGw,
			RetainNlb:   &retainNlb,
		})
		if err != nil {
			log.Printf("Disable500Mbps failed for instance %s: %v", req.InstanceId, err)
			ic.wsService.SendError(fmt.Sprintf("Disable500Mbps 失败 [%s]: %v", req.InstanceId, err))
		} else {
			ic.wsService.SendSuccess(fmt.Sprintf("500Mbps 已成功为实例 %s 关闭，已恢复原始路由", req.InstanceId))
		}
	})

	c.JSON(http.StatusOK, models.SuccessResponse(map[string]interface{}{
		"warning": "关闭任务已提交，实例将恢复原始路由。若保留了 NLB/NAT，其他实例的访问不受影响。",
	}, "500Mbps 关闭任务已启动，正在恢复实例网络配置..."))
}

// GetStatus500MbpsRequest 获取 500Mbps 状态请求
type GetStatus500MbpsRequest struct {
	UserId     string `json:"userId" binding:"required"`
	InstanceId string `json:"instanceId" binding:"required"`
}

// GetStatus500Mbps 获取指定实例的 500Mbps 详细状态与访问端点
func (ic *InstanceController) GetStatus500Mbps(c *gin.Context) {
	var req GetStatus500MbpsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	status, err := ic.instanceService.GetStatus500Mbps(req.UserId, req.InstanceId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(status, "获取 500Mbps 状态成功"))
}

// Plan500MbpsRequest 计划请求
type Plan500MbpsRequest struct {
	UserId     string                        `json:"userId" binding:"required"`
	InstanceId string                        `json:"instanceId" binding:"required"`
	Mode       string                        `json:"mode"`
	SshPort    int                           `json:"sshPort"`
	Mappings   []models.Bandwidth500Endpoint `json:"mappings"`
	NlbId      string                        `json:"nlbId"`
}

// Plan500Mbps 生成 500Mbps 预检与执行计划
func (ic *InstanceController) Plan500Mbps(c *gin.Context) {
	var req Plan500MbpsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	plan, err := ic.instanceService.Plan500Mbps(req.UserId, services.PlanRequest{
		InstanceID: req.InstanceId,
		Mode:       req.Mode,
		SSHPort:    req.SshPort,
		Mappings:   req.Mappings,
		NLBID:      req.NlbId,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(plan, "生成预检计划成功"))
}

// Migrate500MbpsRequest 迁移旧 NLB 为共享池请求
type Migrate500MbpsRequest struct {
	UserId     string                        `json:"userId" binding:"required"`
	InstanceId string                        `json:"instanceId" binding:"required"`
	SshPort    int                           `json:"sshPort"`
	Mappings   []models.Bandwidth500Endpoint `json:"mappings"`
}

// Migrate500Mbps 将历史单实例独享 NLB 原地改造为受管共享池
func (ic *InstanceController) Migrate500Mbps(c *gin.Context) {
	var req Migrate500MbpsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	util.Go("Migrate500Mbps", func() {
		err := ic.instanceService.MigrateLegacyNlbToShared(req.UserId, services.MigrateLegacyRequest{
			InstanceID: req.InstanceId,
			SSHPort:    req.SshPort,
			Mappings:   req.Mappings,
		})
		if err != nil {
			log.Printf("MigrateLegacyNlbToShared failed for instance %s: %v", req.InstanceId, err)
			ic.wsService.SendError(fmt.Sprintf("原地迁移为共享池失败 [%s]: %v", req.InstanceId, err))
		} else {
			ic.wsService.SendSuccess(fmt.Sprintf("实例 %s 对应的 NLB 已成功升级为共享模式，现在第二台实例可加入该共享池", req.InstanceId))
		}
	})

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "旧 NLB 原地升级为共享池任务已启动，请稍候..."))
}

// Reset500MbpsRequest 重置 500Mbps 状态请求
type Reset500MbpsRequest struct {
	UserId     string `json:"userId" binding:"required"`
	InstanceId string `json:"instanceId" binding:"required"`
}

// Reset500Mbps 清除实例的 500Mbps 失败或待修复状态
func (ic *InstanceController) Reset500Mbps(c *gin.Context) {
	var req Reset500MbpsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	err := ic.instanceService.Reset500MbpsStatus(req.UserId, req.InstanceId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "500Mbps 异常状态已清除"))
}

// Check500MbpsSupport 检查实例是否支持500Mbps功能
// 仅 VM.Standard.E2.1.Micro (AMD) 实例支持此功能
type Check500MbpsSupportRequest struct {
	UserId     string `json:"userId" binding:"required"`
	InstanceId string `json:"instanceId" binding:"required"`
}

func (ic *InstanceController) Check500MbpsSupport(c *gin.Context) {
	var req Check500MbpsSupportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	supported, shape, err := ic.instanceService.Check500MbpsSupport(req.UserId, req.InstanceId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(map[string]interface{}{
		"supported": supported,
		"shape":     shape,
		"message":   getShapeSupportMessage(supported),
	}, "检查完成"))
}

func getShapeSupportMessage(supported bool) string {
	if supported {
		return "此实例支持一键开启/关闭下行500Mbps功能"
	}
	return "此实例不支持500Mbps功能，仅 VM.Standard.E2.1.Micro 实例支持此功能"
}
