package controllers

import (
	"github.com/adiecho/oci-panel/internal/config"
	"github.com/adiecho/oci-panel/internal/middleware"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/gin-gonic/gin"
	"net/http"
)

type SysController struct {
	cfg *config.Config
}

func NewSysController(cfg *config.Config) *SysController {
	return &SysController{cfg: cfg}
}

type LoginRequest struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	NeedMFA  bool   `json:"needMfa"`
}

func (sc *SysController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	if req.Account != sc.cfg.Web.Account || req.Password != sc.cfg.Web.Password {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(401, "Invalid credentials"))
		return
	}

	token, err := middleware.GenerateToken(req.Account)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "Failed to generate token"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(LoginResponse{
		Token:    token,
		Username: req.Account,
		NeedMFA:  false,
	}, "Login successful"))
}

type GlanceResponse struct {
	TotalUsers     int64 `json:"totalUsers"`
	TotalTasks     int64 `json:"totalTasks"`
	TotalInstances int64 `json:"totalInstances"`
	SystemUptime   int64 `json:"systemUptime"`
}

func (sc *SysController) GetGlance(c *gin.Context) {
	c.JSON(http.StatusOK, models.SuccessResponse(GlanceResponse{
		TotalUsers:     0,
		TotalTasks:     0,
		TotalInstances: 0,
		SystemUptime:   0,
	}, "success"))
}

type SysCfgResponse struct {
	LogLevel  string `json:"logLevel"`
	AIEnabled bool   `json:"aiEnabled"`
}

func (sc *SysController) GetSysCfg(c *gin.Context) {
	c.JSON(http.StatusOK, models.SuccessResponse(SysCfgResponse{
		LogLevel:  sc.cfg.Logging.Level,
		AIEnabled: sc.cfg.AI.APIKey != "",
	}, "success"))
}
