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
				INSERT INTO sessions (rule_id, server_id, username, status)
				VALUES ($1, $2, $3, 'active')
				RETURNING id, rule_id
			)
			SELECT ns.id, ar.valid_until FROM new_session ns
			LEFT JOIN access_rules ar ON ns.rule_id = ar.id`
		err := db.QueryRow(sqlStatement, req.RuleID, req.ServerID, req.Username).Scan(
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

func GetSessionReplay(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := strconv.Atoi(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, "Geçersiz oturum ID'si", http.StatusBadRequest)
			return
		}

		log.Printf("Oturum tekrar oynatma isteği alındı: Session ID %d", sessionID)
		query := `
			SELECT id, session_id, event_type, data, event_time
			FROM session_events
			WHERE session_id = $1
			ORDER BY event_time ASC, id ASC`
		rows, err := db.Query(query, sessionID)
		if err != nil {
			log.Printf("Veritabanı replay sorgu hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var events []models.SessionEvent
		for rows.Next() {
			var event models.SessionEvent
			if err := rows.Scan(&event.ID, &event.SessionID, &event.EventType, &event.Data, &event.EventTime); err != nil {
				log.Printf("Oturum olayı verisi okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			events = append(events, event)
		}
		if err = rows.Err(); err != nil {
			log.Printf("Satır okuma hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(events)
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
