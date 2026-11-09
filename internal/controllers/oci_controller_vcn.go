package controllers

import (
	"context"
	"net/http"

	"github.com/adiecho/oci-panel/internal/models"
	"github.com/adiecho/oci-panel/internal/services"
	"github.com/adiecho/oci-panel/internal/util"
	"github.com/gin-gonic/gin"
)

// oci_controller_vcn.go — VCN 与安全规则：新建 VCN、查询安全列表、添加规则、一键放行、删除 VCN。

// GetSecurityListRequest 获取安全列表请求
type GetSecurityListRequest struct {
	ConfigID string `json:"configId" binding:"required"`
	VcnID    string `json:"vcnId" binding:"required"`
}

// GetSecurityList 获取VCN安全列表
func (oc *OciController) GetSecurityList(c *gin.Context) {
	var req GetSecurityListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	user, ok := oc.loadUser(c, req.ConfigID)
	if !ok {
		return
	}

	ctx := context.Background()
	securityList, err := oc.ociService.GetSecurityListByVcnId(ctx, &user, req.VcnID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(securityList, "Success"))
}

// AddSecurityRuleRequest 添加安全规则请求
type AddSecurityRuleRequest struct {
	ConfigID    string `json:"configId" binding:"required"`
	VcnID       string `json:"vcnId" binding:"required"`
	IsIngress   bool   `json:"isIngress"`
	Protocol    string `json:"protocol" binding:"required"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	PortMin     int    `json:"portMin"`
	PortMax     int    `json:"portMax"`
	Description string `json:"description"`
	IsStateless bool   `json:"isStateless"`
}

// AddSecurityRule 添加安全规则
func (oc *OciController) AddSecurityRule(c *gin.Context) {
	var req AddSecurityRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	user, ok := oc.loadUser(c, req.ConfigID)
	if !ok {
		return
	}

	rule := &models.SecurityRule{
		Protocol:     req.Protocol,
		Source:       req.Source,
		Destination:  req.Destination,
		PortRangeMin: req.PortMin,
		PortRangeMax: req.PortMax,
		Description:  req.Description,
		IsStateless:  req.IsStateless,
	}

	ctx := context.Background()
	if err := oc.ociService.AddSecurityRule(ctx, &user, req.VcnID, rule, req.IsIngress); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "安全规则添加成功"))
}

// ReleaseSecurityRulesRequest 放行安全规则请求
type ReleaseSecurityRulesRequest struct {
	ConfigID string `json:"configId" binding:"required"`
	VcnID    string `json:"vcnId" binding:"required"`
}

// ReleaseSecurityRules 一键放行安全规则
func (oc *OciController) ReleaseSecurityRules(c *gin.Context) {
	var req ReleaseSecurityRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	user, ok := oc.loadUser(c, req.ConfigID)
	if !ok {
		return
	}

	if err := oc.ociService.ReleaseSecurityRules(&user, req.VcnID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "安全规则放行成功"))
}

// CreateVcnRequest 新建VCN请求
type CreateVcnRequest struct {
	ConfigID      string `json:"configId" binding:"required"`
	DisplayName   string `json:"displayName"`
	CidrBlock     string `json:"cidrBlock"`
	IsIpv6Enabled bool   `json:"isIpv6Enabled"`
}

// CreateVcn 新建VCN（含Internet网关、默认路由与公有子网）
func (oc *OciController) CreateVcn(c *gin.Context) {
	var req CreateVcnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	user, ok := oc.loadUser(c, req.ConfigID)
	if !ok {
		return
	}

	ctx := context.Background()
	vcn, err := oc.ociService.CreateVcnWithNetwork(ctx, &user, services.CreateVcnParams{
		DisplayName:   req.DisplayName,
		CidrBlock:     req.CidrBlock,
		IsIpv6Enabled: req.IsIpv6Enabled,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	util.Go("UpdateConfigCache", func() { oc.schedulerService.UpdateConfigCache(req.ConfigID) })

	c.JSON(http.StatusOK, models.SuccessResponse(vcn, "VCN创建成功"))
}

// DeleteVcnRequest 删除VCN请求
type DeleteVcnRequest struct {
	ConfigID string `json:"configId" binding:"required"`
	VcnID    string `json:"vcnId" binding:"required"`
}

// DeleteVcn 删除VCN
func (oc *OciController) DeleteVcn(c *gin.Context) {
	var req DeleteVcnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	user, ok := oc.loadUser(c, req.ConfigID)
	if !ok {
		return
	}

	ctx := context.Background()
	if err := oc.ociService.DeleteVcn(ctx, &user, req.VcnID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "VCN删除成功"))
}
