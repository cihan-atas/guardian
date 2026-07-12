// guardian/guardian-server/services/retention_service.go
//
// Kayıt saklama politikası: bitmiş oturumların hacimli olay akışını
// (session_events — replay/komut verisi) yapılandırılabilir bir süre sonra
// temizler. Oturum özet satırları (sessions: kullanıcı, sunucu, süre, durum)
// audit için KORUNUR; yalnızca ağır olay kayıtları silinir.
//
// Süre "retention_days" ayarında tutulur (0 = sınırsız, temizlik kapalı).
// Son çalışma zamanı ve silinen kayıt sayısı UI'da göstermek için
// "retention_last_run" / "retention_last_deleted" ayarlarına yazılır.

package services

import (
	"database/sql"
	"strconv"
	"time"
)

const (
	SettingRetentionDays    = "retention_days"
	SettingRetentionLastRun = "retention_last_run"
	SettingRetentionLastDel = "retention_last_deleted"
)

// RetentionDays, kayıtlı saklama süresini gün olarak döner (geçersiz/eksikse 0).
func RetentionDays(db *sql.DB) int {
	s, err := LoadSettings(db)
	if err != nil {
		return 0
	}
	return parseRetentionDays(s[SettingRetentionDays])
}

// parseRetentionDays, negatif/geçersiz değerleri 0 (sınırsız) sayar.
func parseRetentionDays(v string) int {
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// CountPurgeableEvents, verilen süreye göre silinmeye aday session_events
// sayısını döner (days <= 0 ise 0). Önizleme için kullanılır.
func CountPurgeableEvents(db *sql.DB, days int) (int64, error) {
	if days <= 0 {
		return 0, nil
	}
	var count int64
	err := db.QueryRow(`
		SELECT COUNT(*) FROM session_events se
		JOIN sessions s ON s.id = se.session_id
		WHERE s.end_time IS NOT NULL
		  AND s.end_time < (NOW() AT TIME ZONE 'utc') - ($1 * INTERVAL '1 day')`,
		days).Scan(&count)
	return count, err
}

// PurgeOldSessionEvents, bitmiş ve süresi geçmiş oturumların olay kayıtlarını
// siler ve silinen satır sayısını döner. days <= 0 ise hiçbir şey yapmaz.
func PurgeOldSessionEvents(db *sql.DB, days int) (int64, error) {
	if days <= 0 {
		return 0, nil
	}
	res, err := db.Exec(`
		DELETE FROM session_events se
		USING sessions s
		WHERE se.session_id = s.id
		  AND s.end_time IS NOT NULL
		  AND s.end_time < (NOW() AT TIME ZONE 'utc') - ($1 * INTERVAL '1 day')`,
		days)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// RunRetention, kayıtlı süreye göre temizliği çalıştırır ve son-çalışma
// bilgisini ayarlara yazar. Zamanlayıcıdan periyodik olarak çağrılır.
// Süre 0 (sınırsız) ise sessizce çıkar (son-çalışma bilgisi güncellenmez).
func RunRetention(db *sql.DB) (int64, error) {
	days := RetentionDays(db)
	if days <= 0 {
		return 0, nil
	}
	deleted, err := PurgeOldSessionEvents(db, days)
	if err != nil {
		return 0, err
	}
	// Son çalışma bilgisini kaydet (best-effort; hata olsa da temizlik geçerli).
	_ = SaveSetting(db, SettingRetentionLastRun, time.Now().UTC().Format(time.RFC3339))
	_ = SaveSetting(db, SettingRetentionLastDel, strconv.FormatInt(deleted, 10))
	return deleted, nil
}
