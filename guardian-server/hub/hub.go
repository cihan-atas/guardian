package hub

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pingPeriod = (pongWait * 9) / 10
	pongWait   = 3 * time.Second
	writeWait  = 5 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	sessionID int
	send      chan []byte
}

type BroadcastMessage struct {
	SessionID int
	Data      []byte
}

type Hub struct {
	sessions map[int]map[*Client]bool
	mu       sync.Mutex

	Broadcast chan *BroadcastMessage

	Register chan *Client

	Unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		sessions:   make(map[int]map[*Client]bool),
		Broadcast:  make(chan *BroadcastMessage),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			if _, ok := h.sessions[client.sessionID]; !ok {
				h.sessions[client.sessionID] = make(map[*Client]bool)
			}
			h.sessions[client.sessionID][client] = true
			log.Printf("Yeni izleyici kaydedildi: Session ID %d", client.sessionID)
			h.mu.Unlock()

		case client := <-h.Unregister:
			h.mu.Lock()
			if sessionClients, ok := h.sessions[client.sessionID]; ok {
				if _, ok := sessionClients[client]; ok {
					delete(sessionClients, client)
					if len(sessionClients) == 0 {
						delete(h.sessions, client.sessionID)
					}
					log.Printf("İzleyici kayıttan düşüldü: Session ID %d", client.sessionID)
				}
			}
			h.mu.Unlock()

		case message := <-h.Broadcast:
			h.mu.Lock()
			if clients, ok := h.sessions[message.SessionID]; ok {
				for client := range clients {
					select {
					case client.send <- message.Data:
					default:
						close(client.send)
						delete(clients, client)
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) ServeWs(w http.ResponseWriter, r *http.Request, sessionID int) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{
		hub:       h,
		conn:      conn,
		sessionID: sessionID,
		send:      make(chan []byte, 256),
	}
	client.hub.Register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {

		if _, _, err := c.conn.NextReader(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			var msgData map[string]interface{}
			if err := json.Unmarshal(message, &msgData); err == nil {
				if err := c.conn.WriteJSON(msgData); err != nil {
					return
				}
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
