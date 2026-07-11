// guardian/guardian-server/services/ban_service.go

package services

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
	"guardian.com/server/agentclient"
	"guardian.com/server/models"
)

// ActiveBan, verilen anahtar için hâlâ geçerli olan bir yasak varsa döner.
// Yasak yoksa (nil, nil) döner.
func ActiveBan(db *sql.DB, publicKeyID int) (*models.KeyBan, error) {
	var ban models.KeyBan
	var reason sql.NullString
	query := `
		SELECT id, public_key_id, reason, banned_at, banned_until
		FROM key_bans
		WHERE public_key_id = $1 AND banned_until > NOW() AT TIME ZONE 'utc'
		ORDER BY banned_until DESC
		LIMIT 1`
	err := db.QueryRow(query, publicKeyID).Scan(&ban.ID, &ban.PublicKeyID, &reason, &ban.BannedAt, &ban.BannedUntil)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("yasak durumu sorgulanamadı: %w", err)
	}
	ban.Reason = reason.String
	return &ban, nil
}

// BanKey, bir SSH anahtarını belirtilen süre boyunca yasaklar: yeni bir
// key_bans kaydı oluşturur, bu anahtara ait bekleyen/aktif tüm kuralları
// derhal iptal eder, ilgili aktif oturumları sonlandırır ve hedef
// sunuculardaki authorized_keys girdilerinin kaldırılması için agent'lara
// haber verir.
func BanKey(db *sql.DB, ac agentclient.AgentCommunicator, publicKeyID int, duration time.Duration, reason string) (*models.KeyBan, error) {
	bannedUntil := time.Now().UTC().Add(duration)

	var ban models.KeyBan
	insertQuery := `
		INSERT INTO key_bans (public_key_id, reason, banned_until)
		VALUES ($1, $2, $3)
		RETURNING id, public_key_id, reason, banned_at, banned_until`
	var nullableReason sql.NullString
	if reason != "" {
		nullableReason = sql.NullString{String: reason, Valid: true}
	}
	if err := db.QueryRow(insertQuery, publicKeyID, nullableReason, bannedUntil).Scan(
		&ban.ID, &ban.PublicKeyID, &nullableReason, &ban.BannedAt, &ban.BannedUntil,
	); err != nil {
		return nil, fmt.Errorf("yasak kaydı oluşturulamadı: %w", err)
	}
	ban.Reason = nullableReason.String

	revokeRulesForBannedKey(db, ac, publicKeyID)

	return &ban, nil
}

// UnbanKey, bir anahtarın üzerindeki tüm aktif yasakları hemen kaldırır
// (banned_until'ı geçmişe çekerek).
func UnbanKey(db *sql.DB, publicKeyID int) error {
	_, err := db.Exec(
		`UPDATE key_bans SET banned_until = NOW() AT TIME ZONE 'utc' WHERE public_key_id = $1 AND banned_until > NOW() AT TIME ZONE 'utc'`,
		publicKeyID,
	)
	if err != nil {
		return fmt.Errorf("yasak kaldırılamadı: %w", err)
	}
	return nil
}

// revokeRulesForBannedKey, yasaklanan anahtara ait pending/active tüm
// kuralları 'revoked' yapar, hedef sunuculardan anahtarı kaldırtır ve
// bu kurallara bağlı aktif oturumları sonlandırır.
func revokeRulesForBannedKey(db *sql.DB, ac agentclient.AgentCommunicator, publicKeyID int) {
	query := `
		SELECT ar.id, su.username, pk.ssh_public_key, sv.ip_address
		FROM access_rules ar
		JOIN system_users su ON ar.system_user_id = su.id
		JOIN public_keys pk ON ar.public_key_id = pk.id
		JOIN servers sv ON ar.server_id = sv.id
		WHERE ar.public_key_id = $1 AND ar.status IN ('pending', 'active')`

	rows, err := db.Query(query, publicKeyID)
	if err != nil {
		log.Printf("[ERROR] Yasaklanan anahtarın kuralları sorgulanamadı: %v", err)
		return
	}
	var ruleIDs []int
	type ruleTarget struct {
		ruleID    int
		username  string
		publicKey string
		ipAddress string
	}
	var targets []ruleTarget
	for rows.Next() {
		var t ruleTarget
		if scanErr := rows.Scan(&t.ruleID, &t.username, &t.publicKey, &t.ipAddress); scanErr != nil {
			log.Printf("[WARN] Kural verisi okunurken hata: %v", scanErr)
			continue
		}
		ruleIDs = append(ruleIDs, t.ruleID)
		targets = append(targets, t)
	}
	rows.Close()

	if len(ruleIDs) == 0 {
		return
	}

	log.Printf("🚫 Anahtar yasaklandı (public_key_id=%d), %d kural iptal ediliyor: %v", publicKeyID, len(ruleIDs), ruleIDs)

	// Bu kurallara bağlı aktif oturumları sonlandır.
	sessionQuery := `SELECT id FROM sessions WHERE rule_id = ANY($1) AND status = 'active'`
	sessionRows, err := db.Query(sessionQuery, pq.Array(ruleIDs))
	if err != nil {
		log.Printf("[ERROR] Yasaklanan anahtarın aktif oturumları sorgulanamadı: %v", err)
	} else {
		var sessionIDs []int
		for sessionRows.Next() {
			var id int
			if scanErr := sessionRows.Scan(&id); scanErr == nil {
				sessionIDs = append(sessionIDs, id)
			}
		}
		sessionRows.Close()
		for _, sessionID := range sessionIDs {
			if err := UpdateAndTerminateSession(db, ac, sessionID, "terminated_by_ban", nil); err != nil {
				log.Printf("[ERROR] Yasak nedeniyle oturum %d sonlandırılamadı: %v", sessionID, err)
			}
		}
	}

	if _, err := db.Exec(`UPDATE access_rules SET status = 'revoked' WHERE id = ANY($1)`, pq.Array(ruleIDs)); err != nil {
		log.Printf("[ERROR] Kurallar 'revoked' olarak güncellenemedi: %v", err)
	}

	for _, t := range targets {
		payload := models.KeyPayload{
			RuleID:       t.ruleID,
			Username:     t.username,
			SshPublicKey: t.publicKey,
		}
		if err := ac.SendKeyCommand(t.ipAddress, "remove", payload); err != nil {
			log.Printf("[ERROR] Yasaklanan anahtar hedef sunucudan kaldırılamadı (Host: %s, Kural ID: %d): %v", t.ipAddress, t.ruleID, err)
		}
	}
}
