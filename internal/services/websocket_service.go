package services

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketService struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

func NewWebSocketService() *WebSocketService {
	ws := &WebSocketService{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
	go ws.run()
	return ws
}

func (ws *WebSocketService) run() {
	for {
		select {
		case client := <-ws.register:
			ws.mu.Lock()
			ws.clients[client] = true
			ws.mu.Unlock()
			log.Printf("Client connected. Total clients: %d", len(ws.clients))

		case client := <-ws.unregister:
			ws.mu.Lock()
			if _, ok := ws.clients[client]; ok {
				delete(ws.clients, client)
				client.Close()
			}
			ws.mu.Unlock()
			log.Printf("Client disconnected. Total clients: %d", len(ws.clients))

		case message := <-ws.broadcast:
			// 广播分支会在写入失败时从 clients 中删除连接，属于写操作，
			// 必须持有写锁（此前用 RLock 下 delete map 属错误锁纪律）。
			ws.mu.Lock()
			for client := range ws.clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Printf("Error writing to client: %v", err)
					client.Close()
					delete(ws.clients, client)
				}
			}
			ws.mu.Unlock()
		}
	}
}

func (ws *WebSocketService) RegisterClient(conn *websocket.Conn) {
	ws.register <- conn
}

func (ws *WebSocketService) UnregisterClient(conn *websocket.Conn) {
	ws.unregister <- conn
}

func (ws *WebSocketService) BroadcastMessage(message []byte) {
	ws.broadcast <- message
}

func (ws *WebSocketService) SendLog(level string, message string) {
	logMsg := fmt.Sprintf("[%s] %s: %s", time.Now().Format("2006-01-02 15:04:05"), level, message)
	ws.BroadcastMessage([]byte(logMsg))
}

func (ws *WebSocketService) SendInfo(message string) {
	ws.SendLog("INFO", message)
}

func (ws *WebSocketService) SendError(message string) {
	ws.SendLog("ERROR", message)
}

func (ws *WebSocketService) SendWarning(message string) {
	ws.SendLog("WARN", message)
}

func (ws *WebSocketService) SendDebug(message string) {
	ws.SendLog("DEBUG", message)
}

func (ws *WebSocketService) SendSuccess(message string) {
	ws.SendLog("SUCCESS", message)
}

type LogMessage struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

func (ws *WebSocketService) SendStructuredLog(logMsg LogMessage) {
	data, err := json.Marshal(logMsg)
	if err != nil {
		ws.SendError(fmt.Sprintf("failed to marshal log message: %v", err))
		return
	}
	ws.BroadcastMessage(data)
}
