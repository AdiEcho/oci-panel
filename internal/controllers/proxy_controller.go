package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/adiecho/oci-panel/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ProxyController struct {
	ociService *services.OCIService
}

func NewProxyController(ociService *services.OCIService) *ProxyController {
	return &ProxyController{ociService: ociService}
}

type ProxyPageRequest struct {
	Page     int    `json:"page" binding:"required,min=1"`
	PageSize int    `json:"pageSize" binding:"required,min=1,max=100"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
}

type SaveProxyRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol" binding:"required"`
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required,min=1,max=65535"`
	Username string `json:"username"`
	Password string `json:"password"`
	Enabled  *bool  `json:"enabled"`
	Remark   string `json:"remark"`
}

type DeleteProxyRequest struct {
	IDs []string `json:"ids" binding:"required,min=1"`
}

type TestProxyRequest struct {
	ID       string `json:"id"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Target   string `json:"target"`
}

type BatchBindProxyRequest struct {
	ConfigIDs []string `json:"configIds" binding:"required"`
	ProxyID   string   `json:"proxyId"`
}

func normalizeProxyProtocol(protocol string) string {
	p := strings.ToLower(strings.TrimSpace(protocol))
	switch p {
	case "socks5", "socks5h":
		return "socks5"
	case "https":
		return "https"
	default:
		return "http"
	}
}

func validateProxyReq(req *SaveProxyRequest) string {
	req.Name = strings.TrimSpace(req.Name)
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	if req.Name == "" {
		return "代理名称不能为空"
	}
	if req.Host == "" {
		return "代理地址不能为空"
	}
	switch strings.ToLower(strings.TrimSpace(req.Protocol)) {
	case "http", "https", "socks5", "socks5h":
	default:
		return "仅支持 HTTP/HTTPS/SOCKS5 代理"
	}
	req.Protocol = normalizeProxyProtocol(req.Protocol)
	if req.Port < 1 || req.Port > 65535 {
		return "代理端口需在 1-65535 之间"
	}
	return ""
}

func toProxyResponse(p models.ProxyConfig, bindCount int64) models.ProxyConfigResponse {
	return models.ProxyConfigResponse{
		ID:         p.ID,
		Name:       p.Name,
		Protocol:   p.Protocol,
		Host:       p.Host,
		Port:       p.Port,
		Username:   p.Username,
		HasAuth:    p.Username != "" || p.Password != "",
		Enabled:    p.Enabled,
		Remark:     p.Remark,
		BindCount:  bindCount,
		CreateTime: p.CreateTime.Format("2006-01-02 15:04:05"),
	}
}

func proxyBindCount(proxyID string) int64 {
	var count int64
	database.GetDB().Model(&models.OciUser{}).Where("proxy_id = ?", proxyID).Count(&count)
	return count
}

// ProxyPage 代理分页列表。
func (pc *ProxyController) ProxyPage(c *gin.Context) {
	var req ProxyPageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	db := database.GetDB().Model(&models.ProxyConfig{})
	if req.Name != "" {
		db = db.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Protocol != "" {
		db = db.Where("protocol = ?", normalizeProxyProtocol(req.Protocol))
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "查询失败"))
		return
	}

	var proxies []models.ProxyConfig
	offset := (req.Page - 1) * req.PageSize
	if err := db.Order("create_time DESC").Offset(offset).Limit(req.PageSize).Find(&proxies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "查询失败"))
		return
	}

	list := make([]models.ProxyConfigResponse, len(proxies))
	for i, p := range proxies {
		list[i] = toProxyResponse(p, proxyBindCount(p.ID))
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{
		"list":     list,
		"total":    total,
		"page":     req.Page,
		"pageSize": req.PageSize,
	}, "success"))
}

// ProxyList 全量代理选项，供配置选择器与批量绑定使用。
func (pc *ProxyController) ProxyList(c *gin.Context) {
	var proxies []models.ProxyConfig
	if err := database.GetDB().Order("create_time DESC").Find(&proxies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "查询失败"))
		return
	}

	list := make([]models.ProxyConfigResponse, len(proxies))
	for i, p := range proxies {
		list[i] = toProxyResponse(p, 0)
	}
	c.JSON(http.StatusOK, models.SuccessResponse(list, "success"))
}

// CreateProxy 新增代理。
func (pc *ProxyController) CreateProxy(c *gin.Context) {
	var req SaveProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}
	if msg := validateProxyReq(&req); msg != "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, msg))
		return
	}

	var dup int64
	database.GetDB().Model(&models.ProxyConfig{}).Where("name = ?", req.Name).Count(&dup)
	if dup > 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "代理名称已存在"))
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	proxy := models.ProxyConfig{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Protocol:   req.Protocol,
		Host:       req.Host,
		Port:       req.Port,
		Username:   req.Username,
		Password:   req.Password,
		Enabled:    enabled,
		Remark:     req.Remark,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}
	if err := database.GetDB().Create(&proxy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "创建代理失败"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(toProxyResponse(proxy, 0), "创建成功"))
}

// UpdateProxy 更新代理；密码留空表示保持原密码。
func (pc *ProxyController) UpdateProxy(c *gin.Context) {
	var req SaveProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "代理ID不能为空"))
		return
	}
	if msg := validateProxyReq(&req); msg != "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, msg))
		return
	}

	var proxy models.ProxyConfig
	if err := database.GetDB().Where("id = ?", req.ID).First(&proxy).Error; err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse(404, "代理不存在"))
		return
	}

	var dup int64
	database.GetDB().Model(&models.ProxyConfig{}).
		Where("name = ? AND id <> ?", req.Name, req.ID).Count(&dup)
	if dup > 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "代理名称已存在"))
		return
	}

	proxy.Name = req.Name
	proxy.Protocol = req.Protocol
	proxy.Host = req.Host
	proxy.Port = req.Port
	proxy.Username = req.Username
	proxy.Remark = req.Remark
	// 前端不回显密码：空值视为未修改，避免编辑其他字段时清空凭据。
	if req.Password != "" {
		proxy.Password = req.Password
	}
	if req.Enabled != nil {
		proxy.Enabled = *req.Enabled
	}
	proxy.UpdateTime = time.Now()

	if err := database.GetDB().Save(&proxy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "更新代理失败"))
		return
	}

	// 代理参数变更后必须重建传输层与绑定该代理的 SDK 客户端。
	pc.ociService.ProxyService().Invalidate(proxy.ID)
	pc.ociService.InvalidateClientsByProxy(proxy.ID)

	c.JSON(http.StatusOK, models.SuccessResponse(toProxyResponse(proxy, proxyBindCount(proxy.ID)), "更新成功"))
}

// DeleteProxy 删除代理；被配置引用的代理先解绑再删除。
func (pc *ProxyController) DeleteProxy(c *gin.Context) {
	var req DeleteProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	if err := database.GetDB().Model(&models.OciUser{}).
		Where("proxy_id IN ?", req.IDs).Update("proxy_id", "").Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "解绑配置失败"))
		return
	}
	if err := database.GetDB().Where("id IN ?", req.IDs).Delete(&models.ProxyConfig{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "删除代理失败"))
		return
	}

	for _, id := range req.IDs {
		pc.ociService.ProxyService().Invalidate(id)
		pc.ociService.InvalidateClientsByProxy(id)
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "删除成功"))
}

// TestProxy 测试代理连通性。支持直接测试未保存的表单参数；
// 传入 id 且未提供地址时读取已保存记录（密码沿用库内值）。
func (pc *ProxyController) TestProxy(c *gin.Context) {
	var req TestProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	proxy := &models.ProxyConfig{
		Protocol: strings.TrimSpace(req.Protocol),
		Host:     strings.TrimSpace(req.Host),
		Port:     req.Port,
		Username: strings.TrimSpace(req.Username),
		Password: req.Password,
	}

	// 编辑已有代理时补齐表单未提交的字段：密码不回显，留空即沿用已保存凭据。
	if req.ID != "" {
		var saved models.ProxyConfig
		if err := database.GetDB().Where("id = ?", req.ID).First(&saved).Error; err == nil {
			if proxy.Protocol == "" {
				proxy.Protocol = saved.Protocol
			}
			if proxy.Host == "" {
				proxy.Host = saved.Host
				proxy.Port = saved.Port
			}
			if proxy.Username == "" {
				proxy.Username = saved.Username
			}
			if proxy.Password == "" {
				proxy.Password = saved.Password
			}
		} else if proxy.Host == "" {
			c.JSON(http.StatusNotFound, models.ErrorResponse(404, "代理不存在"))
			return
		}
	}
	if proxy.Host == "" || proxy.Port < 1 || proxy.Port > 65535 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "代理地址或端口无效"))
		return
	}

	proxy.Protocol = normalizeProxyProtocol(proxy.Protocol)
	// 连通性测试不受启用开关限制，便于先验证再启用。
	proxy.Enabled = true

	result := pc.ociService.ProxyService().Test(c.Request.Context(), proxy, req.Target)
	c.JSON(http.StatusOK, models.SuccessResponse(result, "success"))
}

// BatchBindProxy 批量设置配置代理；proxyId 为空表示改为直连。
func (pc *ProxyController) BatchBindProxy(c *gin.Context) {
	var req BatchBindProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}
	if len(req.ConfigIDs) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "请选择配置"))
		return
	}

	proxyID := strings.TrimSpace(req.ProxyID)
	if proxyID != "" {
		var proxy models.ProxyConfig
		if err := database.GetDB().Where("id = ?", proxyID).First(&proxy).Error; err != nil {
			c.JSON(http.StatusNotFound, models.ErrorResponse(404, "代理不存在"))
			return
		}
		if !proxy.Enabled {
			c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "代理已禁用，无法绑定"))
			return
		}
	}

	res := database.GetDB().Model(&models.OciUser{}).
		Where("id IN ?", req.ConfigIDs).
		Update("proxy_id", proxyID)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "批量设置代理失败"))
		return
	}

	for _, configID := range req.ConfigIDs {
		pc.ociService.InvalidateClientCache(configID)
	}

	c.JSON(http.StatusOK, models.SuccessResponse(gin.H{"updated": res.RowsAffected}, "设置成功"))
}
