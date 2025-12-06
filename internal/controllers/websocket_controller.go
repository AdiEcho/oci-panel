package controllers

import (
	"log"
	"net/http"

	"github.com/adiecho/oci-panel/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketController struct {
	wsService *services.WebSocketService
}

func NewWebSocketController(wsService *services.WebSocketService) *WebSocketController {
	return &WebSocketController{wsService: wsService}
}

func (wc *WebSocketController) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
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
