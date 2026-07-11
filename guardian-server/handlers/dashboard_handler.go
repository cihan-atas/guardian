package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"guardian.com/server/services"
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
	BannedKeys     int `json:"banned_keys"`
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
				(SELECT COUNT(*) FROM sessions WHERE status IN ('lost_contact', 'error', 'terminated_by_expiry', 'terminated_by_admin', 'terminated_by_rule_deletion')),
				(SELECT COUNT(DISTINCT public_key_id) FROM key_bans WHERE banned_until > NOW() AT TIME ZONE 'utc')
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
			&stats.BannedKeys,
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

		results := []ChartData{}
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

func GetSessionStatusBreakdown(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := `SELECT status, COUNT(*) AS value FROM sessions GROUP BY status ORDER BY value DESC`
		rows, err := db.Query(query)
		if err != nil {
			log.Printf("Oturum durum dağılımı alınırken hata: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		results := []ChartData{}
		for rows.Next() {
			var entry ChartData
			if err := rows.Scan(&entry.Name, &entry.Value); err != nil {
				log.Printf("Oturum durum satırı okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			results = append(results, entry)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func GetRuleStatusBreakdown(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := `SELECT status, COUNT(*) AS value FROM access_rules GROUP BY status ORDER BY value DESC`
		rows, err := db.Query(query)
		if err != nil {
			log.Printf("Kural durum dağılımı alınırken hata: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		results := []ChartData{}
		for rows.Next() {
			var entry ChartData
			if err := rows.Scan(&entry.Name, &entry.Value); err != nil {
				log.Printf("Kural durum satırı okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			results = append(results, entry)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

// GetTopCommands, en son oturumlarda en sık çalıştırılan komutları
// (ilk kelimeye göre gruplanmış) döner. ?limit= ile döndürülecek komut
// sayısı, ?sessions= ile taranacak son oturum sayısı ayarlanabilir.
func GetTopCommands(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil || limit < 1 {
			limit = 10
		}
		sessionLimit, err := strconv.Atoi(r.URL.Query().Get("sessions"))
		if err != nil || sessionLimit < 1 {
			sessionLimit = 200
		}

		results, err := services.GetTopCommands(db, sessionLimit, limit)
		if err != nil {
			log.Printf("En çok kullanılan komutlar alınırken hata: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

// GetAlerts, en son güvenlik uyarılarını (riskli komut tespitleri) döner.
func GetAlerts(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil || limit < 1 {
			limit = 20
		}
		results, err := services.RecentAlerts(db, limit)
		if err != nil {
			log.Printf("Uyarılar alınırken hata: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

// GetCommandStats, TAM komut bazında (örn. "ls -la") kullanım istatistiğini
// sunucu kırılımıyla döner. ?sessions= ile taranacak son oturum sayısı ayarlanır.
func GetCommandStats(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionLimit, err := strconv.Atoi(r.URL.Query().Get("sessions"))
		if err != nil || sessionLimit < 1 {
			sessionLimit = 200
		}

		results, err := services.GetCommandStats(db, sessionLimit)
		if err != nil {
			log.Printf("Komut istatistikleri alınırken hata: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		if results == nil {
			results = []services.CommandStat{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

// GetUserActivity, sunucu+kullanıcı ikilisine göre en çok oturum açan
// kombinasyonları döner ("kim hangi sunucuda en aktif").
func GetUserActivity(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := `
			SELECT s.username || ' @ ' || sv.hostname AS name, COUNT(*) AS value
			FROM sessions s
			JOIN servers sv ON s.server_id = sv.id
			GROUP BY s.username, sv.hostname
			ORDER BY value DESC
			LIMIT 10`
		rows, err := db.Query(query)
		if err != nil {
			log.Printf("Kullanıcı aktivite verisi alınırken hata: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		results := []ChartData{}
		for rows.Next() {
			var entry ChartData
			if err := rows.Scan(&entry.Name, &entry.Value); err != nil {
				log.Printf("Kullanıcı aktivite satırı okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			results = append(results, entry)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

// GetHourlyActivity, oturumların günün hangi saatinde başladığını (0-23)
// sayarak "yoğun saatler" grafiği için veri döner. Saat, sunucunun kendi
// saat dilimine göre hesaplanır.
func GetHourlyActivity(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Saatleri 0-23 olarak üretip oturum sayısına göre eşliyoruz, böylece
		// hiç oturum olmayan saatler de grafikte 0 olarak görünür.
		query := `
			SELECT lpad(h::text, 2, '0') || ':00' AS name, COALESCE(cnt, 0) AS value
			FROM generate_series(0, 23) AS h
			LEFT JOIN (
				SELECT EXTRACT(HOUR FROM start_time)::int AS hour, COUNT(*) AS cnt
				FROM sessions
				GROUP BY hour
			) s ON s.hour = h
			ORDER BY h`
		rows, err := db.Query(query)
		if err != nil {
			log.Printf("Saatlik aktivite verisi alınırken hata: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		results := []ChartData{}
		for rows.Next() {
			var entry ChartData
			if err := rows.Scan(&entry.Name, &entry.Value); err != nil {
				log.Printf("Saatlik aktivite satırı okunurken hata: %v", err)
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

		results := []ChartData{}
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
