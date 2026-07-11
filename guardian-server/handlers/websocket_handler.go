package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"guardian.com/server/agentclient"
	"guardian.com/server/hub"
	"guardian.com/server/services"
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
func AgentSessionStreamHandler(db *sql.DB, h *hub.Hub, ac agentclient.AgentCommunicator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := strconv.Atoi(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, "Geçersiz oturum ID'si", http.StatusBadRequest)
			return
		}
		handleAgentConnection(w, r, db, h, ac, sessionID)
	}
}

func handleAgentConnection(w http.ResponseWriter, r *http.Request, db *sql.DB, h *hub.Hub, ac agentclient.AgentCommunicator, sessionID int) {
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

	// Canlı riskli komut tespiti için kullanıcının bastığı tuşları biriktirip
	// Enter'da (\r/\n) bir komut olarak değerlendiririz (parser ile aynı mantık).
	var cmdBuf []rune

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
					if msg.Type == "input" {
						scanInputForRisky(db, h, ac, sessionID, msg.Data, &cmdBuf)
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

// scanInputForRisky, ham girdi baytlarını komut tamponuna işler ve kullanıcı
// Enter'a bastığında (\r/\n) biriken komutu riskli kalıplara karşı denetler.
// Eşleşme varsa bir alert yükseltir (persist + canlı uyarı + bildirim + oto-aksiyon).
func scanInputForRisky(db *sql.DB, h *hub.Hub, ac agentclient.AgentCommunicator, sessionID int, data []byte, cmdBuf *[]rune) {
	for _, r := range string(data) {
		switch r {
		case '\r', '\n':
			cmd := services.CleanCommand(string(*cmdBuf))
			*cmdBuf = (*cmdBuf)[:0]
			if cmd == "" {
				continue
			}
			if match := services.DetectRisky(cmd); match != nil {
				services.RaiseAlert(db, h, ac, sessionID, match)
			}
		case 0x7F, 0x08: // backspace/delete
			if len(*cmdBuf) > 0 {
				*cmdBuf = (*cmdBuf)[:len(*cmdBuf)-1]
			}
		default:
			*cmdBuf = append(*cmdBuf, r)
			// Aşırı uzun satırlarda tamponu sınırla (yapıştırma/ikili veri koruması).
			if len(*cmdBuf) > 8192 {
				*cmdBuf = (*cmdBuf)[:0]
			}
		}
	}
}
