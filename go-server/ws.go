package main

import (
	"log"
	"net/http"
	"sync"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	conn   *websocket.Conn
	room   string
	userID int64
	send   chan []byte
}

// Hub manages WebSocket clients
type Hub struct {
	clients map[*Client]bool
	mu      sync.Mutex
}

func newHub() *Hub {
	return &Hub{
		clients: make(map[*Client]bool),
	}
}

func (c *Client) writePump(h *Hub) {
	defer func() {
		c.conn.Close()
	}()
	for {
		message, ok := <-c.send
		if !ok {
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
}

func (h *Hub) handleWS(w http.ResponseWriter, r *http.Request) {
	room := r.URL.Query().Get("room")
	
	var userID int64
	if session, ok := checkSession(r); ok {
		userID = session.UserID
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	
	client := &Client{
		conn:   conn,
		room:   room,
		userID: userID,
		send:   make(chan []byte, 256), // 256 messages buffer per client
	}
	
	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()

	go client.writePump(h)

	// Keep connection alive
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}

	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
	h.mu.Unlock()
}

func (h *Hub) broadcast(message []byte) {
	h.broadcastToRoom("", message)
}

func (h *Hub) broadcastToRoom(room string, message []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	for client := range h.clients {
		if client.room == room {
			select {
			case client.send <- message:
			default:
				// Buffer full, drop message for this client
			}
		}
	}
}

func (h *Hub) broadcastToUser(userID int64, message []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	for client := range h.clients {
		if client.userID == userID {
			select {
			case client.send <- message:
			default:
			}
		}
	}
}

// sendToUser sends to all WebSocket connections of a specific user (across any room)
func (h *Hub) sendToUser(userID int64, message []byte) {
	h.broadcastToUser(userID, message)
}
