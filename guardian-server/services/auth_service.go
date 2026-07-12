package services

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Roller (RBAC). Rütbe sırası: viewer < operator < admin.
const (
	RoleViewer   = "viewer"
	RoleOperator = "operator"
	RoleAdmin    = "admin"
)

// sessionTTL, bir giriş oturumunun geçerlilik süresi.
const sessionTTL = 12 * time.Hour

// roleRank rol rütbesini döndürür; bilinmeyen rol 0 (yetkisiz).
func roleRank(role string) int {
	switch role {
	case RoleViewer:
		return 1
	case RoleOperator:
		return 2
	case RoleAdmin:
		return 3
	default:
		return 0
	}
}

// RoleAtLeast, verilen rolün gereken minimum role sahip olup olmadığını söyler.
func RoleAtLeast(role, min string) bool {
	return roleRank(role) >= roleRank(min) && roleRank(role) > 0
}

// ValidRole rolün tanımlı rollerden biri olup olmadığını doğrular.
func ValidRole(role string) bool {
	return role == RoleViewer || role == RoleOperator || role == RoleAdmin
}

// AdminIdentity, kimliği doğrulanmış bir yöneticinin bağlam (context) bilgisi.
type AdminIdentity struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
	TOTPEnabled bool   `json:"totp_enabled"`
}

type adminIdentityKey string

// AdminIdentityContextKey, request context'inde AdminIdentity'nin saklandığı anahtar.
const AdminIdentityContextKey = adminIdentityKey("adminIdentity")

// IdentityFromContext, context'teki kimliği (varsa) döndürür.
func IdentityFromContext(ctx context.Context) (*AdminIdentity, bool) {
	id, ok := ctx.Value(AdminIdentityContextKey).(*AdminIdentity)
	return id, ok
}

// EnsureAuthTables, RBAC için gereken tabloları (yoksa) oluşturur. Daha önce
// deploy edilmiş veritabanlarında bu tablolar bulunmayabileceğinden açılışta
// otomatik migration olarak çalıştırılır.
func EnsureAuthTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS admin_users (
			id serial PRIMARY KEY,
			username varchar(100) UNIQUE NOT NULL,
			password_hash text NOT NULL,
			role varchar(20) NOT NULL DEFAULT 'viewer',
			display_name varchar(150),
			disabled boolean NOT NULL DEFAULT false,
			created_at timestamptz NOT NULL DEFAULT now(),
			last_login timestamptz
		)`,
		`CREATE TABLE IF NOT EXISTS admin_sessions (
			token varchar(64) PRIMARY KEY,
			admin_user_id integer NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
			created_at timestamptz NOT NULL DEFAULT now(),
			expires_at timestamptz NOT NULL,
			last_seen timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_admin_sessions_user ON admin_sessions(admin_user_id)`,
		// 2FA (TOTP) — eski kurulumlarda kolonlar yoksa ekle.
		`ALTER TABLE admin_users ADD COLUMN IF NOT EXISTS totp_secret text`,
		`ALTER TABLE admin_users ADD COLUMN IF NOT EXISTS totp_enabled boolean NOT NULL DEFAULT false`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("auth tablosu oluşturulamadı: %w", err)
		}
	}
	return nil
}

// BootstrapAdmin, hiç admin kullanıcısı yoksa ilk yöneticiyi oluşturur.
// Parola env'den (GUARDIAN_ADMIN_PASSWORD) okunur; boşsa rastgele üretilip
// log'a yazılır (operatör ilk girişte değiştirmeli).
func BootstrapAdmin(db *sql.DB, username, password string) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM admin_users`).Scan(&count); err != nil {
		return fmt.Errorf("admin sayısı okunamadı: %w", err)
	}
	if count > 0 {
		return nil
	}

	username = strings.TrimSpace(username)
	if username == "" {
		username = "admin"
	}

	generated := false
	if password == "" {
		password = randomToken(9) // 18 hex karakter
		generated = true
	}

	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("parola hash'lenemedi: %w", err)
	}

	if _, err := db.Exec(
		`INSERT INTO admin_users (username, password_hash, role, display_name) VALUES ($1, $2, $3, $4)`,
		username, hash, RoleAdmin, "Yönetici",
	); err != nil {
		return fmt.Errorf("ilk admin oluşturulamadı: %w", err)
	}

	if generated {
		log.Printf("\n============================================================\n"+
			"  İLK YÖNETİCİ HESABI OLUŞTURULDU\n"+
			"  Kullanıcı adı: %s\n"+
			"  Geçici parola: %s\n"+
			"  Lütfen giriş yaptıktan sonra parolayı değiştirin.\n"+
			"============================================================\n",
			username, password)
	} else {
		log.Printf("İlk yönetici hesabı '%s' env parolasıyla oluşturuldu.", username)
	}
	return nil
}

// HashPassword bcrypt hash üretir.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ErrInvalidCredentials, kullanıcı adı/parola eşleşmediğinde döner.
var ErrInvalidCredentials = errors.New("kullanıcı adı veya parola hatalı")

// ErrTOTPRequired, parola doğru ama hesapta 2FA açık ve kod verilmemiş/eksikse döner.
var ErrTOTPRequired = errors.New("iki adımlı doğrulama kodu gerekli")

// ErrInvalidTOTP, verilen 2FA kodu hatalıysa döner.
var ErrInvalidTOTP = errors.New("iki adımlı doğrulama kodu hatalı")

// Authenticate kullanıcı adı+parola (+ 2FA açıksa TOTP kodu) doğrular ve yeni
// bir oturum token'ı üretir. 2FA açık olup kod boşsa ErrTOTPRequired döner
// (UI ikinci adımda kodu ister); kod hatalıysa ErrInvalidTOTP.
func Authenticate(db *sql.DB, username, password, totpCode string) (string, *AdminIdentity, error) {
	var (
		id          int
		hash, role  string
		displayName sql.NullString
		disabled    bool
		totpSecret  sql.NullString
		totpEnabled bool
	)
	err := db.QueryRow(
		`SELECT id, password_hash, role, display_name, disabled, totp_secret, totp_enabled FROM admin_users WHERE username = $1`,
		strings.TrimSpace(username),
	).Scan(&id, &hash, &role, &displayName, &disabled, &totpSecret, &totpEnabled)
	if err == sql.ErrNoRows {
		// Zamanlama saldırısını zorlaştırmak için yine de bir bcrypt karşılaştırması yap.
		bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinva"), []byte(password))
		return "", nil, ErrInvalidCredentials
	}
	if err != nil {
		return "", nil, err
	}
	if disabled {
		return "", nil, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", nil, ErrInvalidCredentials
	}

	// 2FA açıksa TOTP kodunu doğrula (parola doğrulandıktan sonra).
	if totpEnabled {
		if strings.TrimSpace(totpCode) == "" {
			return "", nil, ErrTOTPRequired
		}
		if !ValidateTOTP(totpSecret.String, totpCode) {
			return "", nil, ErrInvalidTOTP
		}
	}

	token := randomToken(32)
	expiresAt := time.Now().UTC().Add(sessionTTL)
	if _, err := db.Exec(
		`INSERT INTO admin_sessions (token, admin_user_id, expires_at) VALUES ($1, $2, $3)`,
		token, id, expiresAt,
	); err != nil {
		return "", nil, fmt.Errorf("oturum oluşturulamadı: %w", err)
	}
	db.Exec(`UPDATE admin_users SET last_login = now() WHERE id = $1`, id)

	ident := &AdminIdentity{ID: id, Username: username, Role: role, DisplayName: displayName.String, TOTPEnabled: totpEnabled}
	return token, ident, nil
}

// ValidateSession, verilen oturum token'ını doğrular ve süresi geçmemiş +
// devre dışı olmayan bir kullanıcıya bağlıysa kimliği döndürür.
func ValidateSession(db *sql.DB, token string) (*AdminIdentity, error) {
	if token == "" {
		return nil, ErrInvalidCredentials
	}
	var (
		id          int
		username    string
		role        string
		displayName sql.NullString
		expiresAt   time.Time
		disabled    bool
		totpEnabled bool
	)
	err := db.QueryRow(`
		SELECT u.id, u.username, u.role, u.display_name, s.expires_at, u.disabled, u.totp_enabled
		FROM admin_sessions s
		JOIN admin_users u ON u.id = s.admin_user_id
		WHERE s.token = $1`, token,
	).Scan(&id, &username, &role, &displayName, &expiresAt, &disabled, &totpEnabled)
	if err == sql.ErrNoRows {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if disabled || time.Now().UTC().After(expiresAt) {
		return nil, ErrInvalidCredentials
	}

	// last_seen'i asenkron güncelle (best-effort).
	go func() { db.Exec(`UPDATE admin_sessions SET last_seen = now() WHERE token = $1`, token) }()

	return &AdminIdentity{ID: id, Username: username, Role: role, DisplayName: displayName.String, TOTPEnabled: totpEnabled}, nil
}

// InvalidateSession bir oturum token'ını (logout) siler.
func InvalidateSession(db *sql.DB, token string) error {
	_, err := db.Exec(`DELETE FROM admin_sessions WHERE token = $1`, token)
	return err
}

// InvalidateUserSessions, bir kullanıcının tüm oturumlarını sonlandırır
// (parola değişimi / devre dışı bırakma sonrası).
func InvalidateUserSessions(db *sql.DB, userID int) error {
	_, err := db.Exec(`DELETE FROM admin_sessions WHERE admin_user_id = $1`, userID)
	return err
}

// PurgeExpiredSessions süresi dolmuş oturumları temizler (scheduler çağırır).
func PurgeExpiredSessions(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM admin_sessions WHERE expires_at < now()`)
	return err
}

// VerifyPassword, bir kullanıcının parolasını 2FA akışını tetiklemeden
// doğrular (parola değiştirme / 2FA kapatma gibi hassas işlemler için).
func VerifyPassword(db *sql.DB, userID int, password string) bool {
	var hash string
	if err := db.QueryRow(`SELECT password_hash FROM admin_users WHERE id = $1`, userID).Scan(&hash); err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// totpIssuer, authenticator uygulamasında görünecek yayıncı adı.
const totpIssuer = "Guardian"

// SetupTOTP, kullanıcı için yeni bir TOTP gizli anahtarı üretip kaydeder
// (henüz etkinleştirmez — enabled=false) ve QR için provisioning URI'sini döner.
func SetupTOTP(db *sql.DB, userID int) (secret, uri string, err error) {
	var username string
	if err = db.QueryRow(`SELECT username FROM admin_users WHERE id = $1`, userID).Scan(&username); err != nil {
		return "", "", err
	}
	secret, err = GenerateTOTPSecret()
	if err != nil {
		return "", "", err
	}
	// Yeni gizli anahtarı kaydet; doğrulanana kadar enabled=false kalır.
	if _, err = db.Exec(
		`UPDATE admin_users SET totp_secret = $1, totp_enabled = false WHERE id = $2`,
		secret, userID,
	); err != nil {
		return "", "", err
	}
	uri = TOTPProvisioningURI(secret, username, totpIssuer)
	return secret, uri, nil
}

// EnableTOTP, kayıtlı gizli anahtara karşı verilen kodu doğrular ve 2FA'yı
// etkinleştirir. Anahtar yoksa veya kod hatalıysa hata döner.
func EnableTOTP(db *sql.DB, userID int, code string) error {
	var secret sql.NullString
	if err := db.QueryRow(`SELECT totp_secret FROM admin_users WHERE id = $1`, userID).Scan(&secret); err != nil {
		return err
	}
	if !secret.Valid || secret.String == "" {
		return errors.New("önce 2FA kurulumu başlatılmalı")
	}
	if !ValidateTOTP(secret.String, code) {
		return ErrInvalidTOTP
	}
	_, err := db.Exec(`UPDATE admin_users SET totp_enabled = true WHERE id = $1`, userID)
	return err
}

// DisableTOTP, kullanıcının 2FA'sını kapatır ve gizli anahtarı temizler.
func DisableTOTP(db *sql.DB, userID int) error {
	_, err := db.Exec(`UPDATE admin_users SET totp_secret = NULL, totp_enabled = false WHERE id = $1`, userID)
	return err
}

// SecureEquals sabit zamanlı string karşılaştırması.
func SecureEquals(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func randomToken(nbytes int) string {
	b := make([]byte, nbytes)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand başarısız olursa panik makul: token güvenliği buna dayanıyor.
		panic("rastgele token üretilemedi: " + err.Error())
	}
	return hex.EncodeToString(b)
}
