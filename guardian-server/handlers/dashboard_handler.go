package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
)

type DashboardStats struct {
	ActiveSessions int `json:"active_sessions"`
	ExpiredRules   int `json:"expired_rules"`
	TotalServers   int `json:"total_servers"`
	TotalUsers     int `json:"total_users"`
	PendingRules   int `json:"pending_rules"`
	TotalKeys      int `json:"total_keys"`
	TodaySessions  int `json:"today_sessions"`
	FailedSessions int `json:"failed_sessions"`
}

type ChartData struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func GetDashboardStats(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var stats DashboardStats

		query := `
			SELECT
				(SELECT COUNT(*) FROM sessions WHERE status = 'active'),
				(SELECT COUNT(*) FROM access_rules WHERE status = 'expired'),
				(SELECT COUNT(*) FROM servers),
				(SELECT COUNT(*) FROM system_users),
				(SELECT COUNT(*) FROM access_rules WHERE status = 'pending'),
				(SELECT COUNT(*) FROM public_keys),
				(SELECT COUNT(*) FROM sessions WHERE start_time >= current_date),
				(SELECT COUNT(*) FROM sessions WHERE status IN ('lost_contact', 'error', 'terminated_by_expiry', 'terminated_by_admin', 'terminated_by_rule_deletion'))
		`

		err := db.QueryRow(query).Scan(
			&stats.ActiveSessions,
			&stats.ExpiredRules,
			&stats.TotalServers,
			&stats.TotalUsers,
			&stats.PendingRules,
			&stats.TotalKeys,
			&stats.TodaySessions,
			&stats.FailedSessions,
		)

		if err != nil {
			log.Printf("Dashboard istatistikleri alınırken hata oluştu: %v", err)
			http.Error(w, "Sunucu hatası: İstatistikler alınamadı.", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(stats)
	}
}

func GetSessionActivity(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := `
			SELECT 
				to_char(date_series, 'YYYY-MM-DD') AS name,
				COALESCE(COUNT(sessions.id), 0) AS value
			FROM 
				generate_series(current_date - interval '6 days', current_date, '1 day') AS date_series
			LEFT JOIN 
				sessions ON date(sessions.start_time) = date(date_series)
			GROUP BY 
				date_series
			ORDER BY 
				date_series;
		`
		rows, err := db.Query(query)
		if err != nil {
			log.Printf("Oturum aktivite verisi alınırken hata: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var results []ChartData
		for rows.Next() {
			var entry ChartData
			if err := rows.Scan(&entry.Name, &entry.Value); err != nil {
				log.Printf("Oturum aktivite satırı okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			results = append(results, entry)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func GetTopServers(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := `
			SELECT 
				s.hostname AS name, 
				COUNT(ses.id) AS value
			FROM 
				sessions ses
			JOIN 
				servers s ON ses.server_id = s.id
			GROUP BY 
				s.hostname
			ORDER BY 
				value DESC
			LIMIT 5;
		`
		rows, err := db.Query(query)
		if err != nil {
			log.Printf("En çok kullanılan sunucu verisi alınırken hata: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var results []ChartData
		for rows.Next() {
			var entry ChartData
			if err := rows.Scan(&entry.Name, &entry.Value); err != nil {
				log.Printf("En çok kullanılan sunucu satırı okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			results = append(results, entry)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}
