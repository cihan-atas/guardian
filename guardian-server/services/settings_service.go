// guardian/guardian-server/services/settings_service.go
//
// Bildirim/alarm ayarları artık env yerine (veya env'e ek olarak) UI'dan
// yönetilebilir. Ayarlar basit bir key-value tablosunda tutulur; env değerleri
// yalnızca ilk açılışta (tablo boşsa) tohum olarak yazılır. Kayıttan sonra
// notifier + alerting katmanı yeniden yapılandırılır (servis restartı gerekmez).

package services

import (
	"database/sql"
	"fmt"
)

// Ayar anahtarları (settings tablosundaki key değerleri).
const (
	SettingWebhookURL      = "webhook_url"
	SettingSMTPHost        = "smtp_host"
	SettingSMTPPort        = "smtp_port"
	SettingSMTPUser        = "smtp_user"
	SettingSMTPPass        = "smtp_pass"
	SettingSMTPFrom        = "smtp_from"
	SettingAlertEmailTo    = "alert_email_to"
	SettingRiskyAutoAction = "risky_autoaction"
)

// settingsKeys, yönetilen tüm ayar anahtarları (parola dahil).
var settingsKeys = []string{
	SettingWebhookURL, SettingSMTPHost, SettingSMTPPort, SettingSMTPUser,
	SettingSMTPPass, SettingSMTPFrom, SettingAlertEmailTo, SettingRiskyAutoAction,
}

// EnsureSettingsTable, settings tablosunu yoksa oluşturur.
func EnsureSettingsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS settings (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
		)`)
	return err
}

// LoadSettings, tüm ayarları map olarak döner (eksik anahtarlar boş string).
func LoadSettings(db *sql.DB) (map[string]string, error) {
	out := map[string]string{}
	for _, k := range settingsKeys {
		out[k] = ""
	}
	rows, err := db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			out[k] = v
		}
	}
	return out, rows.Err()
}

// SaveSetting, tek bir ayarı upsert eder.
func SaveSetting(db *sql.DB, key, value string) error {
	_, err := db.Exec(`
		INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW() AT TIME ZONE 'utc')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`,
		key, value)
	return err
}

// SeedSettingsFromEnv, tablo tamamen boşsa (ilk kurulum) env'den gelen
// başlangıç değerlerini bir kez yazar. Var olan kurulumlarda hiçbir şeyi ezmez.
func SeedSettingsFromEnv(db *sql.DB, envDefaults map[string]string) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	for k, v := range envDefaults {
		if v == "" {
			continue
		}
		if err := SaveSetting(db, k, v); err != nil {
			return err
		}
	}
	return nil
}

// ApplySettings, verilen ayar map'ini canlı notifier + alerting yapılandırmasına
// uygular. Kayıt sonrası ve başlangıçta çağrılır.
func ApplySettings(s map[string]string) {
	port := s[SettingSMTPPort]
	if port == "" {
		port = "587"
	}
	ConfigureNotifier(NotifyConfig{
		WebhookURL: s[SettingWebhookURL],
		SMTPHost:   s[SettingSMTPHost],
		SMTPPort:   port,
		SMTPUser:   s[SettingSMTPUser],
		SMTPPass:   s[SettingSMTPPass],
		SMTPFrom:   s[SettingSMTPFrom],
		EmailTo:    s[SettingAlertEmailTo],
	})
	ConfigureAlerting(AutoAction(s[SettingRiskyAutoAction]), 60)
}

// InitSettings, başlangıç akışı: tablo + env tohumu + yükle + uygula.
func InitSettings(db *sql.DB, envDefaults map[string]string) error {
	if err := EnsureSettingsTable(db); err != nil {
		return fmt.Errorf("settings tablosu: %w", err)
	}
	if err := SeedSettingsFromEnv(db, envDefaults); err != nil {
		return fmt.Errorf("settings tohumlama: %w", err)
	}
	s, err := LoadSettings(db)
	if err != nil {
		return fmt.Errorf("settings yükleme: %w", err)
	}
	ApplySettings(s)
	return nil
}
