package services

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// enrollTokenTTL, bir kayıt (enrollment) token'ının geçerlilik süresi.
const enrollTokenTTL = 30 * time.Minute

// ErrEnrollTokenInvalid, token bulunamadı/süresi dolmuş/kullanılmışsa döner.
var ErrEnrollTokenInvalid = errors.New("kayıt token'ı geçersiz, süresi dolmuş veya kullanılmış")

// EnsureEnrollTable, agent kayıt token'ları tablosunu (yoksa) oluşturur.
func EnsureEnrollTable(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS agent_enroll_tokens (
			token varchar(64) PRIMARY KEY,
			server_id integer NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
			created_at timestamptz NOT NULL DEFAULT now(),
			expires_at timestamptz NOT NULL,
			used_at timestamptz
		)`,
		// İmzalanacak agent sertifikasının geçerlilik süresi (gün); 0/NULL → varsayılan.
		`ALTER TABLE agent_enroll_tokens ADD COLUMN IF NOT EXISTS validity_days integer NOT NULL DEFAULT 3650`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("agent_enroll_tokens tablosu oluşturulamadı: %w", err)
		}
	}
	return nil
}

// CreateEnrollToken, verilen sunucu için kısa ömürlü bir kayıt token'ı üretir.
// validityDays, bu token'la imzalanacak agent sertifikasının geçerlilik süresi
// (gün); <= 0 ise varsayılan (10 yıl) kullanılır.
func CreateEnrollToken(db *sql.DB, serverID, validityDays int) (string, time.Time, error) {
	if validityDays <= 0 {
		validityDays = defaultValidityDays
	}
	token := randomToken(24)
	expiresAt := time.Now().UTC().Add(enrollTokenTTL)
	if _, err := db.Exec(
		`INSERT INTO agent_enroll_tokens (token, server_id, expires_at, validity_days) VALUES ($1, $2, $3, $4)`,
		token, serverID, expiresAt, validityDays,
	); err != nil {
		return "", time.Time{}, fmt.Errorf("kayıt token'ı oluşturulamadı: %w", err)
	}
	return token, expiresAt, nil
}

// EnrollTokenValidityDays, token'a bağlı sertifika geçerlilik süresini (gün)
// döndürür; bulunamazsa varsayılanı verir.
func EnrollTokenValidityDays(db *sql.DB, token string) int {
	var days int
	if err := db.QueryRow(`SELECT validity_days FROM agent_enroll_tokens WHERE token = $1`, token).Scan(&days); err != nil || days <= 0 {
		return defaultValidityDays
	}
	return days
}

// ValidateEnrollToken, token geçerliyse bağlı server_id'yi döndürür
// (tüketmeden — script/binary indirme sırasında birden çok kez çağrılabilir).
func ValidateEnrollToken(db *sql.DB, token string) (int, error) {
	if token == "" {
		return 0, ErrEnrollTokenInvalid
	}
	var serverID int
	var expiresAt time.Time
	var usedAt sql.NullTime
	err := db.QueryRow(
		`SELECT server_id, expires_at, used_at FROM agent_enroll_tokens WHERE token = $1`, token,
	).Scan(&serverID, &expiresAt, &usedAt)
	if err == sql.ErrNoRows {
		return 0, ErrEnrollTokenInvalid
	}
	if err != nil {
		return 0, err
	}
	// Token TTL boyunca (varsayılan 30 dk) tekrar tekrar kullanılabilir: kurulum
	// akışı aynı token'la birden çok uç çağırır (install.sh + enroll + ca.crt +
	// binary). Bu yüzden yalnızca süre dolumuna bakılır; used_at yalnızca denetim
	// amaçlı işaretlenir, reddetme sebebi değildir.
	_ = usedAt
	if time.Now().UTC().After(expiresAt) {
		return 0, ErrEnrollTokenInvalid
	}
	return serverID, nil
}

// MarkEnrollTokenUsed, token'ın ilk kullanım zamanını (denetim amaçlı)
// best-effort işaretler; token'ı geçersiz KILMAZ (TTL boyunca kullanılabilir).
func MarkEnrollTokenUsed(db *sql.DB, token string) {
	db.Exec(`UPDATE agent_enroll_tokens SET used_at = now() WHERE token = $1 AND used_at IS NULL`, token)
}

// PurgeExpiredEnrollTokens, süresi geçmiş token'ları temizler (scheduler).
func PurgeExpiredEnrollTokens(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM agent_enroll_tokens WHERE expires_at < now() - INTERVAL '1 day'`)
	return err
}
