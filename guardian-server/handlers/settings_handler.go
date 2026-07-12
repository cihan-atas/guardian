// guardian/guardian-server/handlers/settings_handler.go
//
// Bildirim/alarm ayarlarının UI'dan yönetimi. Parola (smtp_pass) write-only'dir:
// GET yanıtında değeri dönmez, yalnızca "ayarlı mı" bilgisini verir; PUT'te boş
// gelirse mevcut değer korunur.

package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"guardian.com/server/services"
)

// settingsDTO, UI ile alışverişte kullanılan ayar gövdesi.
type settingsDTO struct {
	WebhookURL      string `json:"webhook_url"`
	SMTPHost        string `json:"smtp_host"`
	SMTPPort        string `json:"smtp_port"`
	SMTPUser        string `json:"smtp_user"`
	SMTPPass        string `json:"smtp_pass"`     // PUT'te opsiyonel (boşsa korunur)
	SMTPPassSet     bool   `json:"smtp_pass_set"` // GET'te: parola ayarlı mı
	SMTPFrom        string `json:"smtp_from"`
	AlertEmailTo    string `json:"alert_email_to"`
	RiskyAutoAction string `json:"risky_autoaction"`
	// Kayıt saklama politikası. RetentionDays 0 = sınırsız (temizlik kapalı).
	// LastRun/LastDeleted GET'te bilgilendirme amaçlı döner (read-only).
	RetentionDays        int    `json:"retention_days"`
	RetentionLastRun     string `json:"retention_last_run,omitempty"`
	RetentionLastDeleted int64  `json:"retention_last_deleted,omitempty"`
}

// GetSettings, mevcut ayarları döner (parola maskeli).
func GetSettings(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := services.LoadSettings(db)
		if err != nil {
			log.Printf("Ayarlar okunamadı: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		dto := settingsDTO{
			WebhookURL:      s[services.SettingWebhookURL],
			SMTPHost:        s[services.SettingSMTPHost],
			SMTPPort:        s[services.SettingSMTPPort],
			SMTPUser:        s[services.SettingSMTPUser],
			SMTPPassSet:     s[services.SettingSMTPPass] != "",
			SMTPFrom:        s[services.SettingSMTPFrom],
			AlertEmailTo:    s[services.SettingAlertEmailTo],
			RiskyAutoAction: s[services.SettingRiskyAutoAction],
		}
		if dto.RiskyAutoAction == "" {
			dto.RiskyAutoAction = "none"
		}
		if n, err := strconv.Atoi(s[services.SettingRetentionDays]); err == nil && n > 0 {
			dto.RetentionDays = n
		}
		dto.RetentionLastRun = s[services.SettingRetentionLastRun]
		if n, err := strconv.ParseInt(s[services.SettingRetentionLastDel], 10, 64); err == nil {
			dto.RetentionLastDeleted = n
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dto)
	}
}

// UpdateSettings, ayarları kaydeder ve canlı yapılandırmayı yeniden uygular.
func UpdateSettings(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in settingsDTO
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
			return
		}

		action := in.RiskyAutoAction
		if action != "terminate" && action != "ban" {
			action = "none"
		}

		retention := in.RetentionDays
		if retention < 0 {
			retention = 0
		}

		updates := map[string]string{
			services.SettingWebhookURL:      in.WebhookURL,
			services.SettingSMTPHost:        in.SMTPHost,
			services.SettingSMTPPort:        in.SMTPPort,
			services.SettingSMTPUser:        in.SMTPUser,
			services.SettingSMTPFrom:        in.SMTPFrom,
			services.SettingAlertEmailTo:    in.AlertEmailTo,
			services.SettingRiskyAutoAction: action,
			services.SettingRetentionDays:   strconv.Itoa(retention),
		}
		// Parola yalnızca yeni bir değer girildiyse güncellenir (write-only).
		if in.SMTPPass != "" {
			updates[services.SettingSMTPPass] = in.SMTPPass
		}

		for k, v := range updates {
			if err := services.SaveSetting(db, k, v); err != nil {
				log.Printf("Ayar kaydedilemedi (%s): %v", k, err)
				http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
				return
			}
		}

		// Kaydettikten sonra canlı yapılandırmayı yeniden yükle.
		s, err := services.LoadSettings(db)
		if err == nil {
			services.ApplySettings(s)
		}

		// Saklama süresi güncellendiğinde temizliği hemen tetikle (best-effort);
		// böylece kullanıcı 12 saatlik döngüyü beklemeden etkisini görür.
		go func() {
			if _, err := services.RunRetention(db); err != nil {
				log.Printf("[WARN] Kayıt sonrası saklama temizliği başarısız: %v", err)
			}
		}()

		services.Record(db, r, services.AuditLog{
			Action:     "UPDATE_SETTINGS",
			TargetType: "settings",
			Status:     "SUCCESS",
		})

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}
}

// RetentionPreview, verilen gün değerine göre silinmeye aday olay kaydı
// sayısını döner (?days=N). Ayarlar sayfasında "kaç kayıt etkilenecek"
// önizlemesi için kullanılır; hiçbir şey silmez.
func RetentionPreview(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		days := 0
		if v := r.URL.Query().Get("days"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				days = n
			}
		}
		count, err := services.CountPurgeableEvents(db, days)
		if err != nil {
			log.Printf("Saklama önizlemesi hesaplanamadı: %v", err)
			http.Error(w, "Sunucu hatası", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int64{"count": count})
	}
}

// TestNotification, kayıtlı ayarlarla bir test bildirimi gönderir.
func TestNotification(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		services.Notify(services.NotifyEvent{
			Kind:  "test",
			Title: "Test bildirimi",
			Text:  "Bu bir Guardian test bildirimidir. Bu mesajı aldıysanız bildirim kanalınız çalışıyor. ✅",
			Level: "info",
		})
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"sent"}`))
	}
}
