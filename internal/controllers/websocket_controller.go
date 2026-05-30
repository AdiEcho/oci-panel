package controllers

import (
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/adiecho/oci-panel/internal/middleware"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/adiecho/oci-panel/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WebSocketController struct {
	wsService      *services.WebSocketService
	allowedOrigins []string
	upgrader       websocket.Upgrader
}

func NewWebSocketController(wsService *services.WebSocketService, allowedOrigins []string) *WebSocketController {
	wc := &WebSocketController{
		wsService:      wsService,
		allowedOrigins: allowedOrigins,
	}
	wc.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     wc.checkOrigin,
	}
	return wc
}

// checkOrigin 仅允许同源（Origin 的 host 与请求 Host 一致）或显式白名单内的来源建立连接，
// 替换此前无条件返回 true 的实现——否则任意网站都能跨站发起 WebSocket 连接（CSWSH 风险）。
func (wc *WebSocketController) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// 非浏览器客户端（如本机脚本）通常没有 Origin 头；放行，鉴权仍由 token 保证。
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	for _, a := range wc.allowedOrigins {
		if a == "*" || strings.EqualFold(strings.TrimSpace(a), origin) {
			return true
		}
	}
	return false
}

func (wc *WebSocketController) HandleWebSocket(c *gin.Context) {
	// 升级前先校验 token。浏览器无法为 WebSocket 设置自定义请求头，
	// 因此 token 通过查询参数传递（此路由在 /api 之外，不经过 AuthMiddleware）。
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(401, "Unauthorized"))
		return
	}
	if _, err := middleware.ParseToken(token); err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse(401, "Invalid token"))
		return
	}

	conn, err := wc.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	wc.wsService.RegisterClient(conn)
	defer wc.wsService.UnregisterClient(conn)

	wc.wsService.SendInfo("Connected to log stream")

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}
