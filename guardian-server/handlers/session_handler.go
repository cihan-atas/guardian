// guardian/guardian-server/handlers/session_handler.go (DOĞRU VE TEMİZ HALİ)

package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lib/pq"
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

		// Opsiyonel filtreler: ?search= (kullanıcı/sunucu adı, ILIKE) ve
		// ?status= (virgülle ayrılmış durum listesi, örn. "ended,timed_out").
		var conds []string
		args := []interface{}{}
		if search := strings.TrimSpace(r.URL.Query().Get("search")); search != "" {
			args = append(args, "%"+search+"%")
			conds = append(conds, fmt.Sprintf("(s.username ILIKE $%d OR sv.hostname ILIKE $%d)", len(args), len(args)))
		}
		if statusParam := strings.TrimSpace(r.URL.Query().Get("status")); statusParam != "" {
			statuses := strings.Split(statusParam, ",")
			for i := range statuses {
				statuses[i] = strings.TrimSpace(statuses[i])
			}
			args = append(args, pq.Array(statuses))
			conds = append(conds, fmt.Sprintf("s.status = ANY($%d)", len(args)))
		}
		where := ""
		if len(conds) > 0 {
			where = " WHERE " + strings.Join(conds, " AND ")
		}

		query := fmt.Sprintf(`
			SELECT
				s.id, s.rule_id, s.server_id, s.username, s.start_time, s.end_time, s.status,
				sv.hostname AS server_hostname,
				sv.ip_address AS server_ip
			FROM sessions s
			JOIN servers sv ON s.server_id = sv.id
			%s ORDER BY s.id DESC
			LIMIT $%d OFFSET $%d`, where, len(args)+1, len(args)+2)
		countArgs := append([]interface{}{}, args...)
		args = append(args, limit, offset)
		rows, err := db.Query(query, args...)
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
		countQuery := "SELECT COUNT(*) FROM sessions s JOIN servers sv ON s.server_id = sv.id" + where
		db.QueryRow(countQuery, countArgs...).Scan(&totalRecords)

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
		// Yeni oturum bildirimi (opsiyonel dış kanallara).
		var hostname string
		db.QueryRow(`SELECT hostname FROM servers WHERE id = $1`, req.ServerID).Scan(&hostname)
		if hostname == "" {
			hostname = fmt.Sprintf("server#%d", req.ServerID)
		}
		services.Notify(services.NotifyEvent{
			Kind:    "session_start",
			Title:   fmt.Sprintf("Yeni oturum #%d", response.ID),
			Text:    fmt.Sprintf("%s kullanıcısı %s sunucusunda oturum açtı.", req.Username, hostname),
			Level:   "info",
			Session: response.ID,
		})

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

// ExportSessionAsciicast, bir oturumu asciinema asciicast v2 formatında indirir
// (paylaşım/delil için). Yalnızca "output" olayları dahil edilir (asciinema
// standardı; girdiler terminalde zaten yankılanır). Çıktı satır-satır akıtılır,
// böylece uzun oturumlar bellekte tümüyle tutulmaz.
//
// Format: ilk satır JSON başlık {version,width,height,timestamp,title};
// sonraki her satır [zaman_ofseti_sn, "o", "veri"] üçlüsü.
func ExportSessionAsciicast(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := strconv.Atoi(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, "Geçersiz oturum ID'si", http.StatusBadRequest)
			return
		}

		cols, rows, err := sessionTerminalSize(db, sessionID)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("Asciicast: oturum meta verisi alınamadı: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		if cols <= 0 {
			cols = 80
		}
		if rows <= 0 {
			rows = 24
		}

		// Başlık için oturum bilgisi (varsa) — kullanıcı@host.
		var username, hostname string
		var startTime time.Time
		metaErr := db.QueryRow(`
			SELECT s.username, COALESCE(sv.hostname, ''), s.start_time
			FROM sessions s LEFT JOIN servers sv ON sv.id = s.server_id
			WHERE s.id = $1`, sessionID).Scan(&username, &hostname, &startTime)
		if metaErr == sql.ErrNoRows {
			http.Error(w, "Oturum bulunamadı", http.StatusNotFound)
			return
		} else if metaErr != nil {
			log.Printf("Asciicast: oturum bilgisi alınamadı: %v", metaErr)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		queryRows, err := db.Query(`
			SELECT data, event_time FROM session_events
			WHERE session_id = $1 AND event_type = 'output'
			ORDER BY event_time ASC, id ASC`, sessionID)
		if err != nil {
			log.Printf("Asciicast: olaylar sorgulanamadı: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer queryRows.Close()

		title := username
		if hostname != "" {
			title += "@" + hostname
		}
		filename := fmt.Sprintf("guardian-session-%d.cast", sessionID)
		w.Header().Set("Content-Type", "application/x-asciicast; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

		enc := json.NewEncoder(w)
		// Başlık satırı (asciicast v2).
		header := map[string]any{
			"version":   2,
			"width":     cols,
			"height":    rows,
			"timestamp": startTime.Unix(),
			"title":     title,
		}
		if err := enc.Encode(header); err != nil {
			return // istemci koptu
		}

		var base time.Time
		first := true
		for queryRows.Next() {
			var data []byte
			var t time.Time
			if err := queryRows.Scan(&data, &t); err != nil {
				log.Printf("Asciicast: olay okunamadı: %v", err)
				return
			}
			if first {
				base = t
				first = false
			}
			offset := t.Sub(base).Seconds()
			if offset < 0 {
				offset = 0
			}
			// [zaman, "o", "veri"] — json.Marshal veriyi doğru şekilde kaçışlar.
			if err := enc.Encode([]any{offset, "o", string(data)}); err != nil {
				return // istemci koptu
			}
		}
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
