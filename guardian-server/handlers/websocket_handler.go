package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"guardian.com/server/hub"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ViewerSessionStreamHandler, web arayüzündeki canlı izleyicilerin bağlandığı
// (yalnızca okuma amaçlı) WebSocket endpoint'idir. AdminWSAuth middleware'i
// ile korunur.
func ViewerSessionStreamHandler(h *hub.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := strconv.Atoi(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, "Geçersiz oturum ID'si", http.StatusBadRequest)
			return
		}
		log.Printf("Canlı izleme isteği alındı: Session ID %d", sessionID)
		h.ServeWs(w, r, sessionID)
	}
}

// AgentSessionStreamHandler, guardian-agent'ın oturum kaydını (input/output/
// heartbeat) gönderdiği WebSocket endpoint'idir. AgentAuth middleware'i ile
// korunur.
func AgentSessionStreamHandler(db *sql.DB, h *hub.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := strconv.Atoi(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, "Geçersiz oturum ID'si", http.StatusBadRequest)
			return
		}
		handleAgentConnection(w, r, db, h, sessionID)
	}
}

func handleAgentConnection(w http.ResponseWriter, r *http.Request, db *sql.DB, h *hub.Hub, sessionID int) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Agent WebSocket upgrade hatası: %v", err)
		return
	}
	defer conn.Close()
	log.Printf("Agent WebSocket bağlantısı kuruldu: Session ID %d", sessionID)

	eventSQL := `INSERT INTO session_events (session_id, event_type, data) VALUES ($1, $2, $3)`
	heartbeatSQL := `UPDATE sessions SET last_heartbeat = NOW() AT TIME ZONE 'utc' WHERE id = $1`

	if _, err := db.Exec(heartbeatSQL, sessionID); err != nil {
		log.Printf("[ERROR] İlk heartbeat kaydedilemedi (Session ID %d): %v", sessionID, err)
	}

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if messageType == websocket.TextMessage {
			var msg struct {
				Type string `json:"type"`
				Data []byte `json:"data"`
			}
			if err := json.Unmarshal(message, &msg); err == nil {
				switch msg.Type {
				case "heartbeat":
					if _, dbErr := db.Exec(heartbeatSQL, sessionID); dbErr != nil {
						log.Printf("[ERROR] Heartbeat DB'ye kaydedilemedi (Session ID %d): %v", sessionID, dbErr)
					}

				case "input", "output":
					if _, dbErr := db.Exec(eventSQL, sessionID, msg.Type, msg.Data); dbErr != nil {
						log.Printf("HATA: Oturum olayı veritabanına kaydedilemedi (Oturum ID %d): %v", sessionID, dbErr)
					}
					broadcastMsg := &hub.BroadcastMessage{
						SessionID: sessionID,
						Data:      message,
					}
					h.Broadcast <- broadcastMsg

				default:
					log.Printf("[WARN] Bilinmeyen WebSocket mesaj tipi alındı: %s", msg.Type)
				}
			}
		}
	}
}
