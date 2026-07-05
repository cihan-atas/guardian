// guardian/guardian-server/handlers/session_handler.go (DOĞRU VE TEMİZ HALİ)

package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"guardian.com/server/agentclient"
	"guardian.com/server/models"
	"guardian.com/server/services"
)

func ListSessions(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil || page < 1 {
			page = 1
		}
		limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil || limit < 1 {
			limit = 8
		}
		offset := (page - 1) * limit

		query := `
			SELECT 
				s.id, s.rule_id, s.server_id, s.username, s.start_time, s.end_time, s.status,
				sv.hostname AS server_hostname,
				sv.ip_address AS server_ip
			FROM sessions s
			JOIN servers sv ON s.server_id = sv.id
			ORDER BY s.id DESC 
			LIMIT $1 OFFSET $2`

		rows, err := db.Query(query, limit, offset)
		if err != nil {
			log.Printf("Veritabanı zenginleştirilmiş oturum listeleme hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var sessions []models.SessionDetailsAPI
		for rows.Next() {
			var s models.SessionDetailsAPI
			if err := rows.Scan(
				&s.ID, &s.RuleID, &s.ServerID, &s.Username, &s.StartTime, &s.EndTime, &s.Status,
				&s.ServerHostname, &s.ServerIP,
			); err != nil {
				log.Printf("Oturum verisi okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			sessions = append(sessions, s)
		}
		if err = rows.Err(); err != nil {
			log.Printf("Satır okuma hatası (sessions): %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		var totalRecords int
		countQuery := "SELECT COUNT(*) FROM sessions"
		db.QueryRow(countQuery).Scan(&totalRecords)

		response := struct {
			TotalRecords int                        `json:"total_records"`
			Page         int                        `json:"page"`
			Limit        int                        `json:"limit"`
			Data         []models.SessionDetailsAPI `json:"data"`
		}{
			TotalRecords: totalRecords,
			Page:         page,
			Limit:        limit,
			Data:         sessions,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func StartSession(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Desteklenmeyen metod", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			RuleID   int    `json:"rule_id"`
			ServerID int    `json:"server_id"`
			Username string `json:"username"`
			Cols     int    `json:"cols"`
			Rows     int    `json:"rows"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
			return
		}
		var response struct {
			ID         int           `json:"id"`
			ValidUntil *sql.NullTime `json:"valid_until,omitempty"`
		}
		sqlStatement := `
			WITH new_session AS (
				INSERT INTO sessions (rule_id, server_id, username, status, cols, rows)
				VALUES ($1, $2, $3, 'active', NULLIF($4, 0), NULLIF($5, 0))
				RETURNING id, rule_id
			)
			SELECT ns.id, ar.valid_until FROM new_session ns
			LEFT JOIN access_rules ar ON ns.rule_id = ar.id`
		err := db.QueryRow(sqlStatement, req.RuleID, req.ServerID, req.Username, req.Cols, req.Rows).Scan(
			&response.ID, &response.ValidUntil,
		)
		if err != nil {
			log.Printf("Veritabanına yeni oturum eklenemedi: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}
}

func EndSession(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := strconv.Atoi(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, "Geçersiz oturum ID'si", http.StatusBadRequest)
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
			return
		}

		sqlStatement := `UPDATE sessions SET status = $1, end_time = NOW() WHERE id = $2`
		result, err := db.Exec(sqlStatement, req.Status, sessionID)
		if err != nil {
			log.Printf("Oturum durumu güncellenemedi: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			http.Error(w, "Güncellenecek oturum bulunamadı", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Oturum başarıyla güncellendi."))
	}
}

// sessionTerminalSize, bir oturumun kaydedildiği PTY boyutunu (cols/rows) döner.
// Kayıt sırasında kullanılan gerçek terminal boyutu ile tekrar oynatma/canlı izleme
// tarafında oluşturulan terminalin boyutu eşleşmezse, mutlak imleç konumlandırma
// ve scroll-region ANSI kodları yanlış yorumlanır (ekran birden fazla ekranı
// dolduğunda imlecin en üste sıçraması ve ekranın bozulması bu yüzdendir).
func sessionTerminalSize(db *sql.DB, sessionID int) (cols, rows int, err error) {
	var nCols, nRows sql.NullInt64
	err = db.QueryRow(`SELECT cols, rows FROM sessions WHERE id = $1`, sessionID).Scan(&nCols, &nRows)
	if err != nil {
		return 0, 0, err
	}
	if nCols.Valid {
		cols = int(nCols.Int64)
	}
	if nRows.Valid {
		rows = int(nRows.Int64)
	}
	return cols, rows, nil
}

// GetSessionMeta, bir oturumun kaydedildiği terminal boyutunu (cols/rows) döner.
// Canlı izleme arayüzü, WebSocket bağlantısını açmadan önce terminalini bu
// boyuta göre oluşturmak için bu endpoint'i kullanır.
func GetSessionMeta(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := strconv.Atoi(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, "Geçersiz oturum ID'si", http.StatusBadRequest)
			return
		}

		cols, rows, err := sessionTerminalSize(db, sessionID)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Oturum bulunamadı", http.StatusNotFound)
				return
			}
			log.Printf("Oturum meta verisi alınamadı: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Cols int `json:"cols"`
			Rows int `json:"rows"`
		}{Cols: cols, Rows: rows})
	}
}

func GetSessionReplay(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := strconv.Atoi(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, "Geçersiz oturum ID'si", http.StatusBadRequest)
			return
		}

		log.Printf("Oturum tekrar oynatma isteği alındı: Session ID %d", sessionID)

		cols, rows, err := sessionTerminalSize(db, sessionID)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("Oturum meta verisi alınamadı: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		query := `
			SELECT id, session_id, event_type, data, event_time
			FROM session_events
			WHERE session_id = $1
			ORDER BY event_time ASC, id ASC`
		rows2, err := db.Query(query, sessionID)
		if err != nil {
			log.Printf("Veritabanı replay sorgu hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows2.Close()

		var events []models.SessionEvent
		for rows2.Next() {
			var event models.SessionEvent
			if err := rows2.Scan(&event.ID, &event.SessionID, &event.EventType, &event.Data, &event.EventTime); err != nil {
				log.Printf("Oturum olayı verisi okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			events = append(events, event)
		}
		if err = rows2.Err(); err != nil {
			log.Printf("Satır okuma hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Cols   int                   `json:"cols"`
			Rows   int                   `json:"rows"`
			Events []models.SessionEvent `json:"events"`
		}{Cols: cols, Rows: rows, Events: events})
	}
}

func TerminateSession(db *sql.DB, ac agentclient.AgentCommunicator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := strconv.Atoi(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, "Geçersiz oturum ID'si", http.StatusBadRequest)
			return
		}

		err = services.UpdateAndTerminateSession(db, ac, sessionID, "terminated_by_admin", r)
		if err != nil {
			log.Printf("[ERROR] Yönetici tarafından oturum sonlandırılamadı: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Oturum %d için sonlandırma komutu başarıyla gönderildi ve durum güncellendi.", sessionID)
	}
}
