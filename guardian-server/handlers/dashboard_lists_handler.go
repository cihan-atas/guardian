package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"guardian.com/server/models"
)

func GetActiveSessionsList(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := `
			SELECT 
				s.id, s.username, s.start_time, sv.hostname AS server_hostname
			FROM sessions s
			JOIN servers sv ON s.server_id = sv.id
			WHERE s.status = 'active'
			ORDER BY s.start_time DESC
			LIMIT 5;
		`
		rows, err := db.Query(query)
		if err != nil {
			log.Printf("Aktif oturum listesi alınırken hata: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type ActiveSessionInfo struct {
			ID             int       `json:"id"`
			Username       string    `json:"username"`
			StartTime      time.Time `json:"start_time"`
			ServerHostname string    `json:"server_hostname"`
		}

		var results []ActiveSessionInfo
		for rows.Next() {
			var entry ActiveSessionInfo
			if err := rows.Scan(&entry.ID, &entry.Username, &entry.StartTime, &entry.ServerHostname); err != nil {
				log.Printf("Aktif oturum satırı okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			results = append(results, entry)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func GetAuditLogStream(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := `
			SELECT id, admin_ref, action, target_type, target_id, status, error_message, created_at
			FROM audit_logs
			ORDER BY created_at DESC
			LIMIT 5;
		`
		rows, err := db.Query(query)
		if err != nil {
			log.Printf("Denetim kaydı listesi alınırken hata: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var results []models.AuditLog
		for rows.Next() {
			var entry models.AuditLog
			if err := rows.Scan(&entry.ID, &entry.AdminRef, &entry.Action, &entry.TargetType, &entry.TargetID, &entry.Status, &entry.ErrorMessage, &entry.CreatedAt); err != nil {
				log.Printf("Denetim kaydı satırı okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			results = append(results, entry)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}
