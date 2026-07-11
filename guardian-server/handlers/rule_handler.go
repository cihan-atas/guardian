// guardian/guardian-server/handlers/rule_handler.go (GÜNCELLENMİŞ HALİ)

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
	"guardian.com/server/agentclient" // Bu import zaten olmalı
	"guardian.com/server/models"
	"guardian.com/server/services"
)

// GetRule fonksiyonunda değişiklik yok
func GetRule(db *sql.DB) http.HandlerFunc {
	// ... (içerik aynı)
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "ruleID"))
		if err != nil {
			http.Error(w, "Geçersiz kural ID'si", http.StatusBadRequest)
			return
		}

		var rule models.AccessRule
		query := "SELECT id, server_id, public_key_id, system_user_id, valid_from, valid_until, status, created_at FROM access_rules WHERE id = $1"
		err = db.QueryRow(query, id).Scan(&rule.ID, &rule.ServerID, &rule.PublicKeyID, &rule.SystemUserID, &rule.ValidFrom, &rule.ValidUntil, &rule.Status, &rule.CreatedAt)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Kural bulunamadı", http.StatusNotFound)
				return
			}
			log.Printf("Tek kural sorgu hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

// ListRules fonksiyonunda değişiklik yok
func ListRules(db *sql.DB) http.HandlerFunc {
	// ... (içerik aynı)
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

		// Opsiyonel arama: sunucu adı, kullanıcı adı veya anahtar adı üzerinde ILIKE.
		search := strings.TrimSpace(r.URL.Query().Get("search"))
		// Onay akışındaki (talep/reddedilmiş) kayıtlar Kurallar listesinde
		// gösterilmez; onlar Erişim Talepleri ekranında yönetilir.
		where := " WHERE ar.status NOT IN ('awaiting_approval','rejected')"
		args := []interface{}{}
		if search != "" {
			where += " AND (s.hostname ILIKE $1 OR su.username ILIKE $1 OR pk.key_name ILIKE $1)"
			args = append(args, "%"+search+"%")
		}
		query := fmt.Sprintf(`
			SELECT
				ar.id, ar.server_id, ar.public_key_id, ar.system_user_id,
				ar.valid_from, ar.valid_until, ar.status, ar.created_at,
				s.hostname AS server_hostname,
				su.username,
				pk.key_name
			FROM access_rules ar
			JOIN servers s ON ar.server_id = s.id
			JOIN system_users su ON ar.system_user_id = su.id
			JOIN public_keys pk ON ar.public_key_id = pk.id
			%s ORDER BY ar.id DESC
			LIMIT $%d OFFSET $%d`, where, len(args)+1, len(args)+2)
		countArgs := append([]interface{}{}, args...)
		args = append(args, limit, offset)
		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("Veritabanı zenginleştirilmiş kural sorgu hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var rules []models.AccessRuleDetails
		for rows.Next() {
			var rule models.AccessRuleDetails
			if err := rows.Scan(
				&rule.ID, &rule.ServerID, &rule.PublicKeyID, &rule.SystemUserID,
				&rule.ValidFrom, &rule.ValidUntil, &rule.Status, &rule.CreatedAt,
				&rule.ServerHostname, &rule.Username, &rule.KeyName,
			); err != nil {
				log.Printf("Kural verisi okunurken hata: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
			rules = append(rules, rule)
		}
		if err = rows.Err(); err != nil {
			log.Printf("Kural satırı okuma hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		var totalRecords int
		countQuery := `
			SELECT COUNT(*) FROM access_rules ar
			JOIN servers s ON ar.server_id = s.id
			JOIN system_users su ON ar.system_user_id = su.id
			JOIN public_keys pk ON ar.public_key_id = pk.id` + where
		db.QueryRow(countQuery, countArgs...).Scan(&totalRecords)

		response := struct {
			TotalRecords int                        `json:"total_records"`
			Page         int                        `json:"page"`
			Limit        int                        `json:"limit"`
			Data         []models.AccessRuleDetails `json:"data"`
		}{
			TotalRecords: totalRecords,
			Page:         page,
			Limit:        limit,
			Data:         rules,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// DEĞİŞİKLİK: *agentclient.Client yerine agentclient.AgentCommunicator kullan
func CreateRule(db *sql.DB, ac agentclient.AgentCommunicator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var rule models.AccessRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "Geçersiz istek gövdesi: "+err.Error(), http.StatusBadRequest)
			return
		}

		var targetIP, targetUsername string
		query := "SELECT s.ip_address, su.username FROM servers s, system_users su WHERE s.id = $1 AND su.id = $2"
		err := db.QueryRow(query, rule.ServerID, rule.SystemUserID).Scan(&targetIP, &targetUsername)
		if err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionCreateRule,
				TargetType:   "rule",
				Status:       "FAILURE",
				ErrorMessage: "Invalid server or user ID.",
			})
			http.Error(w, "Geçersiz sunucu veya kullanıcı ID'si.", http.StatusBadRequest)
			return
		}

		if err := ac.ValidateUser(targetIP, targetUsername); err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionCreateRule,
				TargetType:   "rule",
				Status:       "FAILURE",
				ErrorMessage: fmt.Sprintf("User validation failed on agent: %v", err),
			})
			http.Error(w, fmt.Sprintf("Kullanıcı doğrulanamadı: %v", err), http.StatusBadRequest)
			return
		}

		if ban, banErr := services.ActiveBan(db, rule.PublicKeyID); banErr == nil && ban != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionCreateRule,
				TargetType:   "rule",
				Status:       "FAILURE",
				ErrorMessage: fmt.Sprintf("Public key %d is banned until %s", rule.PublicKeyID, ban.BannedUntil),
			})
			http.Error(w, fmt.Sprintf("Bu SSH anahtarı %s tarihine kadar yasaklı.", ban.BannedUntil.Local().Format("2006-01-02 15:04")), http.StatusForbidden)
			return
		}

		sqlStatement := `INSERT INTO access_rules (server_id, public_key_id, system_user_id, valid_from, valid_until) VALUES ($1, $2, $3, $4, $5) RETURNING id, status, created_at`
		err = db.QueryRow(sqlStatement, rule.ServerID, rule.PublicKeyID, rule.SystemUserID, rule.ValidFrom, rule.ValidUntil).Scan(&rule.ID, &rule.Status, &rule.CreatedAt)
		if err != nil {
			services.Record(db, r, services.AuditLog{
				Action:       services.ActionCreateRule,
				TargetType:   "rule",
				Status:       "FAILURE",
				ErrorMessage: err.Error(),
			})
			http.Error(w, "Sunucu hatası: Kural oluşturulamadı.", http.StatusInternalServerError)
			return
		}

		services.Record(db, r, services.AuditLog{
			Action:     services.ActionCreateRule,
			TargetType: "rule",
			TargetID:   rule.ID,
			Status:     "SUCCESS",
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(rule)
	}
}

// DEĞİŞİKLİK: *agentclient.Client yerine agentclient.AgentCommunicator kullan
func DeleteRule(db *sql.DB, ac agentclient.AgentCommunicator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruleID, err := strconv.Atoi(chi.URLParam(r, "ruleID"))
		if err != nil {
			http.Error(w, "Geçersiz kural ID'si", http.StatusBadRequest)
			return
		}

		var payload models.KeyPayload
		var ipAddress string
		var ruleStatus string
		queryDetails := `
			SELECT su.username, pk.ssh_public_key, s.ip_address, r.status
			FROM access_rules r
			JOIN servers s ON r.server_id = s.id
			JOIN public_keys pk ON r.public_key_id = pk.id
			JOIN system_users su ON r.system_user_id = su.id
			WHERE r.id = $1`
		err = db.QueryRow(queryDetails, ruleID).Scan(&payload.Username, &payload.SshPublicKey, &ipAddress, &ruleStatus)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Silinecek kural bulunamadı", http.StatusNotFound)
			} else {
				log.Printf("[ERROR] Kural detayları alınamadı: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			}
			return
		}
		payload.RuleID = ruleID

		tx, err := db.Begin()
		if err != nil {
			log.Printf("[ERROR] Transaction başlatılamadı: %v", err)
			http.Error(w, "Veritabanı hatası", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		var sessionsToTerminate []struct {
			ID      int
			AgentIP string
		}
		if ruleStatus == "active" {
			querySessions := `SELECT s.id, sv.ip_address FROM sessions s JOIN servers sv ON s.server_id = sv.id WHERE s.rule_id = $1 AND s.status = 'active'`
			rows, err := tx.Query(querySessions, ruleID)
			if err != nil {
				log.Printf("[ERROR] Aktif oturumlar sorgulanamadı: %v", err)
				http.Error(w, "Sunucu hatası: Aktif oturumlar sorgulanamadı", http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			for rows.Next() {
				var session struct {
					ID      int
					AgentIP string
				}
				if err := rows.Scan(&session.ID, &session.AgentIP); err != nil {
					log.Printf("[WARN] Oturum verisi okunurken hata: %v", err)
					continue
				}
				sessionsToTerminate = append(sessionsToTerminate, session)
			}
		}

		if len(sessionsToTerminate) > 0 {
			sessionIDs := make([]int, len(sessionsToTerminate))
			for i, s := range sessionsToTerminate {
				sessionIDs[i] = s.ID
			}
			updateQuery := `UPDATE sessions SET status = 'terminated_by_rule_deletion', end_time = NOW() AT TIME ZONE 'utc' WHERE id = ANY($1)`
			if _, err := tx.Exec(updateQuery, pq.Array(sessionIDs)); err != nil {
				log.Printf("[ERROR] Aktif oturumların durumu güncellenemedi: %v", err)
				http.Error(w, "Sunucu hatası: Oturumlar güncellenemedi", http.StatusInternalServerError)
				return
			}
		}

		unlinkQuery := `UPDATE sessions SET rule_id = NULL WHERE rule_id = $1`
		if _, err := tx.Exec(unlinkQuery, ruleID); err != nil {
			log.Printf("[ERROR] Oturumların kural ile bağlantısı kesilemedi: %v", err)
			http.Error(w, "Sunucu hatası: Oturum bağlantıları güncellenemedi", http.StatusInternalServerError)
			return
		}

		if _, err := tx.Exec("DELETE FROM access_rules WHERE id = $1", ruleID); err != nil {
			log.Printf("[ERROR] Kural veritabanından silinemedi: %v", err)
			http.Error(w, "Sunucu hatası: Kural silinemedi", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			log.Printf("[ERROR] Transaction onaylanamadı: %v", err)
			http.Error(w, "İşlem onaylanamadı", http.StatusInternalServerError)
			return
		}

		if len(sessionsToTerminate) > 0 {
			log.Printf("%d adet aktif oturum için sonlandırma komutları gönderiliyor...", len(sessionsToTerminate))
			for _, session := range sessionsToTerminate {
				go func(ip string, sID int) {
					if err := ac.TerminateSession(ip, sID); err != nil {
						log.Printf("[WARN] Kural silinirken aktif oturum %d sonlandırılamadı: %v", sID, err)
					}
				}(session.AgentIP, session.ID)
			}
		}

		if ruleStatus == "active" {
			if err := ac.SendKeyCommand(ipAddress, "remove", payload); err != nil {
				log.Printf("[WARN] Kural (%d) silindikten sonra agent'a remove-key komutu gönderilemedi: %v", ruleID, err)
			}
		}

		services.Record(db, r, services.AuditLog{Action: services.ActionDeleteRule, TargetType: "rule", TargetID: ruleID, Status: "SUCCESS"})
		w.WriteHeader(http.StatusNoContent)
	}
}

// DEĞİŞİKLİK: *agentclient.Client yerine agentclient.AgentCommunicator kullan
func PatchRule(db *sql.DB, ac agentclient.AgentCommunicator) http.HandlerFunc {
	// ... (içerik aynı, sadece imza değişiyor)
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "ruleID"))
		if err != nil {
			http.Error(w, "Geçersiz kural ID'si", http.StatusBadRequest)
			return
		}

		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
			return
		}

		validFromStr, fromOk := updates["valid_from"].(string)
		validUntilStr, untilOk := updates["valid_until"].(string)
		if !fromOk || !untilOk {
			http.Error(w, "valid_from ve valid_until alanları zorunludur ve string olmalıdır.", http.StatusBadRequest)
			return
		}

		newValidFrom, err1 := time.Parse(time.RFC3339, validFromStr)
		if err1 != nil {
			http.Error(w, fmt.Sprintf("Geçersiz 'valid_from' tarih formatı: %v", err1), http.StatusBadRequest)
			return
		}

		newValidUntil, err2 := time.Parse(time.RFC3339, validUntilStr)
		if err2 != nil {
			http.Error(w, fmt.Sprintf("Geçersiz 'valid_until' tarih formatı: %v", err2), http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			log.Printf("[ERROR] Transaction başlatılamadı: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		var oldStatus string
		query := "SELECT status FROM access_rules WHERE id = $1 FOR UPDATE"
		err = tx.QueryRow(query, id).Scan(&oldStatus)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Kural bulunamadı", http.StatusNotFound)
			} else {
				log.Printf("Kural çekilirken veritabanı hatası: %v", err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			}
			return
		}

		now := time.Now().UTC()
		var newStatus string
		if now.After(newValidUntil) {
			newStatus = "expired"
		} else if now.After(newValidFrom) {
			newStatus = "active"
		} else {
			newStatus = "pending"
		}

		var sessionsToTerminate []struct {
			SessionID int
			AgentIP   string
		}
		if oldStatus == "active" && newStatus != "active" {
			log.Printf("Kural %d artık aktif değil. İlişkili aktif oturumlar sonlandırılacak.", id)
			sessionQuery := `
				SELECT s.id, sv.ip_address 
				FROM sessions s
				JOIN servers sv ON s.server_id = sv.id
				WHERE s.rule_id = $1 AND s.status = 'active'`
			rows, err := tx.Query(sessionQuery, id)
			if err != nil {
				log.Printf("[ERROR] Aktif oturumlar sorgulanamadı: %v", err)
				http.Error(w, "Sunucu hatası: Aktif oturumlar sorgulanamadı", http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			for rows.Next() {
				var session struct {
					SessionID int
					AgentIP   string
				}
				if err := rows.Scan(&session.SessionID, &session.AgentIP); err != nil {
					log.Printf("[WARN] Aktif oturum verisi okunurken hata (atlanıyor): %v", err)
					continue
				}
				sessionsToTerminate = append(sessionsToTerminate, session)
			}

			// Veritabanındaki oturumların durumunu güncelle
			if len(sessionsToTerminate) > 0 {
				sessionIDs := make([]int, len(sessionsToTerminate))
				for i, s := range sessionsToTerminate {
					sessionIDs[i] = s.SessionID
				}
				updateSessionQuery := `UPDATE sessions SET status = 'terminated_by_rule_change', end_time = NOW() AT TIME ZONE 'utc' WHERE id = ANY($1)`
				if _, err := tx.Exec(updateSessionQuery, pq.Array(sessionIDs)); err != nil {
					log.Printf("[ERROR] Aktif oturumların durumu güncellenemedi: %v", err)
					http.Error(w, "Sunucu hatası: Oturumlar güncellenemedi", http.StatusInternalServerError)
					return
				}
			}
		}

		updateQuery := `UPDATE access_rules SET valid_from = $1, valid_until = $2, status = $3 WHERE id = $4 RETURNING id, server_id, public_key_id, system_user_id, valid_from, valid_until, status, created_at`
		var updatedRule models.AccessRule
		err = tx.QueryRow(updateQuery, newValidFrom, newValidUntil, newStatus, id).Scan(
			&updatedRule.ID, &updatedRule.ServerID, &updatedRule.PublicKeyID, &updatedRule.SystemUserID,
			&updatedRule.ValidFrom, &updatedRule.ValidUntil, &updatedRule.Status, &updatedRule.CreatedAt,
		)
		if err != nil {
			log.Printf("Kural güncellenirken veritabanı hatası (Scan): %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		var agentPayload models.KeyPayload
		var agentIP string
		if oldStatus != newStatus {
			detailQuery := `SELECT su.username, pk.ssh_public_key, s.ip_address FROM access_rules r JOIN servers s ON r.server_id = s.id JOIN public_keys pk ON r.public_key_id = pk.id JOIN system_users su ON r.system_user_id = su.id WHERE r.id = $1`
			err := tx.QueryRow(detailQuery, id).Scan(&agentPayload.Username, &agentPayload.SshPublicKey, &agentIP)
			if err != nil {
				log.Printf("[WARN] Agent'ı bilgilendirmek için kural detayları alınamadı (id: %d): %v", id, err)
			} else {
				agentPayload.RuleID = id
			}
		}

		if err := tx.Commit(); err != nil {
			log.Printf("[ERROR] Transaction onaylanamadı: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		if len(sessionsToTerminate) > 0 {
			log.Printf("%d adet aktif oturum için sonlandırma komutları gönderiliyor...", len(sessionsToTerminate))
			for _, s := range sessionsToTerminate {
				go ac.TerminateSession(s.AgentIP, s.SessionID)
			}
		}

		if oldStatus != newStatus && agentIP != "" {
			log.Printf("Kural %d durumu değişti: %s -> %s. Agent (%s) bilgilendiriliyor.", id, oldStatus, newStatus, agentIP)
			go func() {
				if newStatus == "active" {
					ac.SendKeyCommand(agentIP, "add", agentPayload)
				} else if oldStatus == "active" {
					ac.SendKeyCommand(agentIP, "remove", agentPayload)
				}
			}()
		}

		services.Record(db, r, services.AuditLog{Action: services.ActionPatchRule, TargetType: "rule", TargetID: id, Status: "SUCCESS"})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(updatedRule)
	}
}

func UpdateRule(db *sql.DB) http.HandlerFunc {
	// ... (içerik aynı)
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "ruleID"))
		if err != nil {
			http.Error(w, "Geçersiz kural ID'si", http.StatusBadRequest)
			return
		}

		var rule models.AccessRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
			return
		}

		sqlStatement := `
			UPDATE access_rules
			SET server_id = $1, public_key_id = $2, system_user_id = $3, valid_from = $4, valid_until = $5, status = $6
			WHERE id = $7
			RETURNING id, server_id, public_key_id, system_user_id, valid_from, valid_until, status, created_at`

		var updatedRule models.AccessRule
		err = db.QueryRow(
			sqlStatement, rule.ServerID, rule.PublicKeyID, rule.SystemUserID, rule.ValidFrom, rule.ValidUntil, rule.Status, id,
		).Scan(
			&updatedRule.ID, &updatedRule.ServerID, &updatedRule.PublicKeyID, &updatedRule.SystemUserID, &updatedRule.ValidFrom, &updatedRule.ValidUntil, &updatedRule.Status, &updatedRule.CreatedAt,
		)

		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Kural bulunamadı", http.StatusNotFound)
				return
			}
			log.Printf("Veritabanı kural PUT hatası: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(updatedRule)
	}
}
