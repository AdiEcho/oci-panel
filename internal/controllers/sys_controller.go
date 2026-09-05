package controllers

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image/png"
	"net/http"
	"sync"
	"time"

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
	mfaMu            sync.Mutex
	mfaChallenges    map[string]mfaChallenge
	loginMu          sync.Mutex
	loginFailures    map[string]loginFailure
}

const (
	mfaChallengeTTL        = 5 * time.Minute
	mfaChallengeTries      = 5
	mfaChallengeBytes      = 32
	maxMFAChallenges       = 4096
	loginFailureWindow     = time.Minute
	loginFailureLimit      = 10
	maxLoginFailureEntries = 4096
)

type mfaChallenge struct {
	account   string
	expiresAt time.Time
	attempts  int
}

type loginFailure struct {
	windowStart time.Time
	count       int
}

func NewSysController(cfg *config.Config, schedulerService *services.SchedulerService) *SysController {
	return &SysController{
		cfg:              cfg,
		schedulerService: schedulerService,
		mfaChallenges:    make(map[string]mfaChallenge),
		loginFailures:    make(map[string]loginFailure),
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
	MFATicket      string `json:"mfaTicket,omitempty"`
}

func (sc *SysController) issueMFAChallenge(account string) (string, error) {
	raw := make([]byte, mfaChallengeBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	ticket := hex.EncodeToString(raw)
	now := time.Now()

	sc.mfaMu.Lock()
	defer sc.mfaMu.Unlock()
	for existing, challenge := range sc.mfaChallenges {
		if now.After(challenge.expiresAt) {
			delete(sc.mfaChallenges, existing)
		}
	}
	if len(sc.mfaChallenges) >= maxMFAChallenges {
		return "", fmt.Errorf("too many pending MFA challenges")
	}
	sc.mfaChallenges[ticket] = mfaChallenge{
		account:   account,
		expiresAt: now.Add(mfaChallengeTTL),
	}
	return ticket, nil
}

func (sc *SysController) consumeMFAChallenge(ticket, account, code, secret string) bool {
	now := time.Now()
	sc.mfaMu.Lock()
	defer sc.mfaMu.Unlock()

	challenge, ok := sc.mfaChallenges[ticket]
	if !ok || now.After(challenge.expiresAt) {
		delete(sc.mfaChallenges, ticket)
		return false
	}
	if challenge.account != account {
		return false
	}
	if challenge.attempts >= mfaChallengeTries {
		delete(sc.mfaChallenges, ticket)
		return false
	}

	if !totp.Validate(code, secret) {
		challenge.attempts++
		if challenge.attempts >= mfaChallengeTries {
			delete(sc.mfaChallenges, ticket)
		} else {
			sc.mfaChallenges[ticket] = challenge
		}
		return false
	}

	delete(sc.mfaChallenges, ticket)
	return true
}

func (sc *SysController) loginRateLimited(account string) bool {
	now := time.Now()
	sc.loginMu.Lock()
	defer sc.loginMu.Unlock()

	if sc.loginFailures == nil {
		sc.loginFailures = make(map[string]loginFailure)
	}
	for key, failure := range sc.loginFailures {
		if now.Sub(failure.windowStart) >= loginFailureWindow {
			delete(sc.loginFailures, key)
		}
	}
	failure, ok := sc.loginFailures[account]
	return ok && now.Sub(failure.windowStart) < loginFailureWindow && failure.count >= loginFailureLimit
}

func (sc *SysController) recordLoginFailure(account string) {
	now := time.Now()
	sc.loginMu.Lock()
	defer sc.loginMu.Unlock()

	if sc.loginFailures == nil {
		sc.loginFailures = make(map[string]loginFailure)
	}
	failure, ok := sc.loginFailures[account]
	if !ok || now.Sub(failure.windowStart) >= loginFailureWindow {
		if len(sc.loginFailures) >= maxLoginFailureEntries {
			return
		}
		sc.loginFailures[account] = loginFailure{windowStart: now, count: 1}
		return
	}
	failure.count++
	sc.loginFailures[account] = failure
}

func (sc *SysController) clearLoginFailures(account string) {
	sc.loginMu.Lock()
	delete(sc.loginFailures, account)
	sc.loginMu.Unlock()
}

func (sc *SysController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}
	if sc.loginRateLimited(req.Account) {
		c.JSON(http.StatusTooManyRequests, models.ErrorResponse(429, "Too many login attempts; try again later"))
		return
	}

	// 常量时间比较账号 + 支持 bcrypt 的密码校验；两个判断都先求值再合并，
	// 避免账号错误时提前返回造成的时序侧信道。
	accountOK := middleware.SecureCompareString(req.Account, sc.cfg.Web.Account)
	passwordOK := middleware.VerifyPassword(sc.cfg.Web.Password, req.Password)
	if !accountOK || !passwordOK {
		sc.recordLoginFailure(req.Account)
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(401, "Invalid credentials"))
		return
	}
	sc.clearLoginFailures(req.Account)

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
		mfaTicket := ""
		if mfaEnabled {
			var err error
			mfaTicket, err = sc.issueMFAChallenge(req.Account)
			if err != nil {
				c.JSON(http.StatusInternalServerError, models.ErrorResponse(500, "Failed to create MFA challenge"))
				return
			}
		}
		c.JSON(http.StatusOK, models.SuccessResponse(LoginResponse{
			Token:          "",
			Username:       req.Account,
			NeedMFA:        mfaEnabled,
			NeedPasskey:    passkeyEnabled,
			PasskeyEnabled: passkeyEnabled,
			MFATicket:      mfaTicket,
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

	// upsert：保持原有「失败静默」行为（原代码亦未检查写入错误）
	_ = database.UpsertSysSetting(MfaSecretKey, req.Secret)
	_ = database.UpsertSysSetting(MfaEnabledKey, "true")

	c.JSON(http.StatusOK, models.SuccessResponse(nil, "MFA enabled successfully"))
}

func (sc *SysController) DisableMfa(c *gin.Context) {
	db := database.GetDB()
	db.Model(&models.SysSetting{}).Where("key = ?", MfaEnabledKey).Update("value", "false")
	db.Model(&models.SysSetting{}).Where("key = ?", MfaSecretKey).Update("value", "")
	c.JSON(http.StatusOK, models.SuccessResponse(nil, "MFA disabled successfully"))
}

type CheckMfaCodeRequest struct {
	Ticket string `json:"ticket" binding:"required"`
	Code   string `json:"code" binding:"required,len=6"`
}

func (sc *SysController) CheckMfaCode(c *gin.Context) {
	var req CheckMfaCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, err.Error()))
		return
	}

	db := database.GetDB()
	var enabledSetting models.SysSetting
	if err := db.Where("key = ?", MfaEnabledKey).First(&enabledSetting).Error; err != nil || enabledSetting.Value != "true" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "MFA not enabled"))
		return
	}

	var secretSetting models.SysSetting
	if err := db.Where("key = ?", MfaSecretKey).First(&secretSetting).Error; err != nil || secretSetting.Value == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse(400, "MFA not configured"))
		return
	}

	if !sc.consumeMFAChallenge(req.Ticket, sc.cfg.Web.Account, req.Code, secretSetting.Value) {
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
