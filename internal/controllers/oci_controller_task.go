package controllers

import (
	"net/http"
	"time"

	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// oci_controller_task.go — 实例创建任务的提交与分页查询（异步建机工作流的入口）。

type CreateInstanceRequest struct {
	UserID          string  `json:"userId" binding:"required"`
	OciRegion       string  `json:"ociRegion" binding:"required"`
	Ocpus           float64 `json:"ocpus"`
	Memory          float64 `json:"memory"`
	Disk            int     `json:"disk"`
	Architecture    string  `json:"architecture"`
	OperationSystem string  `json:"operationSystem"`
	SSHKeyID        string  `json:"sshKeyId" binding:"required"`
}

func (oc *OciController) CreateInstance(c *gin.Context) {
	var req CreateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	// 验证SSH密钥是否存在
	var sshKey models.SSHKey
	if err := database.GetDB().First(&sshKey, "id = ?", req.SSHKeyID).Error; err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "SSH密钥不存在"))
		return
	}

	task := models.OciCreateTask{
		ID:              uuid.New().String(),
		UserID:          req.UserID,
		OciRegion:       req.OciRegion,
		Ocpus:           req.Ocpus,
		Memory:          req.Memory,
		Disk:            req.Disk,
		Architecture:    req.Architecture,
		OperationSystem: req.OperationSystem,
		SSHKeyID:        req.SSHKeyID,
		CreateTime:      time.Now(),
	}

	if err := database.GetDB().Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "Failed to create task"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "Task created successfully"))
}

type CreateTaskPageRequest struct {
	Page     int    `json:"page" binding:"required,min=1"`
	PageSize int    `json:"pageSize" binding:"required,min=1,max=100"`
	UserID   string `json:"userId"`
}

type CreateTaskPageResponse struct {
	List     []models.OciCreateTask `json:"list"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
}

func (oc *OciController) CreateTaskPage(c *gin.Context) {
	var req CreateTaskPageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	db := database.GetDB()
	var tasks []models.OciCreateTask
	var total int64

	query := db.Model(&models.OciCreateTask{})
	if req.UserID != "" {
		query = query.Where("user_id = ?", req.UserID)
	}

	query.Count(&total)
	offset := (req.Page - 1) * req.PageSize
	query.Order("create_time DESC").Limit(req.PageSize).Offset(offset).Find(&tasks)

	c.JSON(http.StatusOK, models.SuccessResponse(CreateTaskPageResponse{
		List:     tasks,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, "success"))
}
