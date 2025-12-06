package controllers

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"net/http"

	"github.com/adiecho/oci-panel/internal/config"
	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/middleware"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/adiecho/oci-panel/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
)

type SysController struct {
	cfg              *config.Config
	schedulerService *services.SchedulerService
}

func NewSysController(cfg *config.Config, schedulerService *services.SchedulerService) *SysController {
	return &SysController{
		cfg:              cfg,
		schedulerService: schedulerService,
	}
}

type LoginRequest struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token          string `json:"token"`
	Username       string `json:"username"`
	NeedMFA        bool   `json:"needMfa"`
	NeedPasskey    bool   `json:"needPasskey"`
	PasskeyEnabled bool   `json:"passkeyEnabled"`
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

	db := database.GetDB()
	mfaEnabled := false
	passkeyEnabled := false

	var mfaSetting models.SysSetting
	if err := db.Where("key = ?", "mfa_enabled").First(&mfaSetting).Error; err == nil {
		mfaEnabled = mfaSetting.Value == "true"
	}

	var passkeySetting models.SysSetting
	if err := db.Where("key = ?", "passkey_enabled").First(&passkeySetting).Error; err == nil {
		passkeyEnabled = passkeySetting.Value == "true"
	}

	if mfaEnabled || passkeyEnabled {
		c.JSON(http.StatusOK, models.SuccessResponse(LoginResponse{
			Token:          "",
			Username:       req.Account,
			NeedMFA:        mfaEnabled,
			NeedPasskey:    passkeyEnabled,
			PasskeyEnabled: passkeyEnabled,
		}, "Additional verification required"))
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
	TotalConfigs int64 `json:"totalConfigs"`
	TotalTasks   int64 `json:"totalTasks"`
}

func (sc *SysController) GetGlance(c *gin.Context) {
	db := database.GetDB()

	var totalConfigs int64
	db.Model(&models.OciUser{}).Count(&totalConfigs)

	var totalTasks int64
	db.Model(&models.OciCreateTask{}).Count(&totalTasks)

	c.JSON(http.StatusOK, models.SuccessResponse(GlanceResponse{
		TotalConfigs: totalConfigs,
		TotalTasks:   totalTasks,
	}, "success"))
}

type SysCfgResponse struct {
	LogLevel      string `json:"logLevel"`
	CacheEnabled  bool   `json:"cacheEnabled"`
	CacheInterval int    `json:"cacheInterval"`
}

func (sc *SysController) GetSysCfg(c *gin.Context) {
	c.JSON(http.StatusOK, models.SuccessResponse(SysCfgResponse{
		LogLevel:      sc.cfg.Logging.Level,
		CacheEnabled:  sc.schedulerService.IsCacheEnabled(),
		CacheInterval: sc.schedulerService.GetCacheInterval(),
	}, "success"))
}

type UpdateCacheCfgRequest struct {
	CacheEnabled  bool `json:"cacheEnabled"`
	CacheInterval int  `json:"cacheInterval"`
}

func (sc *SysController) UpdateCacheCfg(c *gin.Context) {
	var req UpdateCacheCfgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	if err := sc.schedulerService.SetCacheEnabled(req.CacheEnabled); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "Failed to update cache enabled"))
		return
	}

	if req.CacheInterval > 0 {
		if err := sc.schedulerService.SetCacheInterval(req.CacheInterval); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "Failed to update cache interval"))
			return
		}
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "Cache configuration updated"))
}

func (sc *SysController) RefreshCache(c *gin.Context) {
	if !sc.schedulerService.IsCacheEnabled() {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "Cache is not enabled"))
		return
	}

	sc.schedulerService.RefreshAllCaches()
	c.JSON(http.StatusOK, models.SuccessResponse(nil, "Cache refresh started"))
}

const (
	MfaEnabledKey = "mfa_enabled"
	MfaSecretKey  = "mfa_secret"
)

type AuthStatusResponse struct {
	MfaEnabled     bool `json:"mfaEnabled"`
	PasskeyEnabled bool `json:"passkeyEnabled"`
}

func (sc *SysController) GetAuthStatus(c *gin.Context) {
	db := database.GetDB()
	mfaEnabled := false
	passkeyEnabled := false

	var mfaSetting models.SysSetting
	if err := db.Where("key = ?", MfaEnabledKey).First(&mfaSetting).Error; err == nil {
		mfaEnabled = mfaSetting.Value == "true"
	}

	var passkeySetting models.SysSetting
	if err := db.Where("key = ?", "passkey_enabled").First(&passkeySetting).Error; err == nil {
		passkeyEnabled = passkeySetting.Value == "true"
	}

	c.JSON(http.StatusOK, models.SuccessResponse(AuthStatusResponse{
		MfaEnabled:     mfaEnabled,
		PasskeyEnabled: passkeyEnabled,
	}, "success"))
}

type GenerateMfaResponse struct {
	Secret string `json:"secret"`
	QrCode string `json:"qrCode"`
}

func (sc *SysController) GenerateMfaSecret(c *gin.Context) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "OCI Panel",
		AccountName: sc.cfg.Web.Account,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "Failed to generate MFA secret"))
		return
	}

	var buf bytes.Buffer
	img, err := key.Image(200, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "Failed to generate QR code"))
		return
	}
	if err := png.Encode(&buf, img); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "Failed to encode QR code"))
		return
	}
	qrCode := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	c.JSON(http.StatusOK, models.SuccessResponse(GenerateMfaResponse{
		Secret: key.Secret(),
		QrCode: qrCode,
	}, "success"))
}

type EnableMfaRequest struct {
	Secret string `json:"secret" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

func (sc *SysController) EnableMfa(c *gin.Context) {
	var req EnableMfaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	valid := totp.Validate(req.Code, req.Secret)
	if !valid {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "Invalid verification code"))
		return
	}

	db := database.GetDB()

	var secretSetting models.SysSetting
	if err := db.Where("key = ?", MfaSecretKey).First(&secretSetting).Error; err != nil {
		secretSetting = models.SysSetting{
			ID:    "mfa_secret_id",
			Key:   MfaSecretKey,
			Value: req.Secret,
		}
		db.Create(&secretSetting)
	} else {
		db.Model(&secretSetting).Update("value", req.Secret)
	}

	var enabledSetting models.SysSetting
	if err := db.Where("key = ?", MfaEnabledKey).First(&enabledSetting).Error; err != nil {
		enabledSetting = models.SysSetting{
			ID:    "mfa_enabled_id",
			Key:   MfaEnabledKey,
			Value: "true",
		}
		db.Create(&enabledSetting)
	} else {
		db.Model(&enabledSetting).Update("value", "true")
	}

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "MFA enabled successfully"))
}

func (sc *SysController) DisableMfa(c *gin.Context) {
	db := database.GetDB()
	db.Model(&models.SysSetting{}).Where("key = ?", MfaEnabledKey).Update("value", "false")
	db.Model(&models.SysSetting{}).Where("key = ?", MfaSecretKey).Update("value", "")
	c.JSON(http.StatusOK, models.SuccessResponse(nil, "MFA disabled successfully"))
}

type CheckMfaCodeRequest struct {
	Code string `json:"code" binding:"required"`
}

func (sc *SysController) CheckMfaCode(c *gin.Context) {
	var req CheckMfaCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	db := database.GetDB()
	var secretSetting models.SysSetting
	if err := db.Where("key = ?", MfaSecretKey).First(&secretSetting).Error; err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "MFA not configured"))
		return
	}

	valid := totp.Validate(req.Code, secretSetting.Value)
	if !valid {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(401, "Invalid verification code"))
		return
	}

	token, err := middleware.GenerateToken(sc.cfg.Web.Account)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "Failed to generate token"))
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse(LoginResponse{
		Token:    token,
		Username: sc.cfg.Web.Account,
		NeedMFA:  false,
	}, "MFA verification successful"))
}
