package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/adiecho/oci-panel/internal/config"
	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret []byte

const (
	jwtSecretSettingID  = "jwt_secret_id"
	jwtSecretSettingKey = "jwt_secret"
)

// InitJwtSecret 解析并初始化 JWT 签名密钥，优先级：
//  1. config.toml 中显式配置的 web.jwt_secret；
//  2. 数据库 sys_setting 中此前自动生成并持久化的密钥；
//  3. 都没有时用 crypto/rand 生成 32 字节随机密钥并持久化到数据库。
//
// 这样既移除了此前硬编码的公开默认密钥（可被任何人伪造 token 的高危问题），
// 又保证未配置密钥的部署在重启后 token 依然有效。
// 注意：调用前必须已完成 database.InitDB。
func InitJwtSecret(cfg *config.Config) {
	if s := strings.TrimSpace(cfg.Web.JwtSecret); s != "" {
		jwtSecret = []byte(s)
		return
	}

	db := database.GetDB()
	if db != nil {
		var setting models.SysSetting
		if err := db.Where("key = ?", jwtSecretSettingKey).First(&setting).Error; err == nil && setting.Value != "" {
			jwtSecret = []byte(setting.Value)
			return
		}
	}

	secret, err := generateRandomSecret(32)
	if err != nil {
		log.Fatalf("failed to generate JWT secret: %v", err)
	}
	jwtSecret = []byte(secret)

	if db != nil {
		setting := models.SysSetting{ID: jwtSecretSettingID, Key: jwtSecretSettingKey, Value: secret}
		if err := db.Create(&setting).Error; err != nil {
			// 持久化失败通常意味着已存在一行（唯一索引冲突 / 并发启动）——回读并采用既有密钥，
			// 避免使用与数据库不一致的临时密钥而让此前签发的 token 失效。
			var existing models.SysSetting
			if qErr := db.Where("key = ?", jwtSecretSettingKey).First(&existing).Error; qErr == nil && existing.Value != "" {
				jwtSecret = []byte(existing.Value)
				return
			}
			log.Printf("warning: failed to persist generated JWT secret (tokens will not survive restart): %v", err)
		} else {
			log.Printf("no web.jwt_secret configured; generated and persisted a random JWT secret to the database")
		}
	} else {
		log.Printf("warning: database not initialized before InitJwtSecret; using an ephemeral JWT secret (tokens will not survive restart)")
	}
}

// generateRandomSecret 返回 n 字节的密码学随机数据的十六进制编码字符串。
func generateRandomSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func GenerateToken(username string) (string, error) {
	claims := Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 对于非API请求（前端路由页面），直接放行
		if !strings.HasPrefix(path, "/api") {
			c.Next()
			return
		}

		// API请求中不需要认证的路径
		if path == "/api/sys/login" ||
			path == "/api/sys/checkMfaCode" ||
			path == "/api/passkey/status" ||
			path == "/api/passkey/beginLogin" ||
			path == "/api/passkey/finishLogin" {
			c.Next()
			return
		}

		// 验证token
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" || !strings.HasPrefix(tokenString, "Bearer ") {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse(401, "Unauthorized"))
			c.Abort()
			return
		}

		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
		claims, err := ParseToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse(401, "Invalid token"))
			c.Abort()
			return
		}

		c.Set("username", claims.Username)
		c.Next()
	}
}

// RequestBodyLimit bounds every HTTP request body before handlers parse JSON or
// multipart data. Individual upload handlers may apply a smaller limit.
func RequestBodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// CORS 返回跨域中间件。仅当请求 Origin 命中白名单时才回显该 Origin 并允许携带凭证，
// 避免此前 "Access-Control-Allow-Origin: * + Allow-Credentials: true" 的非法且不安全组合。
// 白名单为空时不下发任何 ACAO 头（同源请求不受影响，生产环境前端由本服务同源托管）。
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && originAllowed(origin, allowedOrigins) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Add("Vary", "Origin")
		}
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// originAllowed 判断请求 Origin 是否被允许（大小写不敏感的精确匹配）。
// 若白名单包含 "*"，则反射回具体 Origin（而非字面量 "*"），从而与凭证模式兼容。
func originAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" {
			return true
		}
		if strings.EqualFold(strings.TrimSpace(a), origin) {
			return true
		}
	}
	return false
}
