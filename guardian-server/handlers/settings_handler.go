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

		updates := map[string]string{
			services.SettingWebhookURL:      in.WebhookURL,
			services.SettingSMTPHost:        in.SMTPHost,
			services.SettingSMTPPort:        in.SMTPPort,
			services.SettingSMTPUser:        in.SMTPUser,
			services.SettingSMTPFrom:        in.SMTPFrom,
			services.SettingAlertEmailTo:    in.AlertEmailTo,
			services.SettingRiskyAutoAction: action,
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

		services.Record(db, r, services.AuditLog{
			Action:     "UPDATE_SETTINGS",
			TargetType: "settings",
			Status:     "SUCCESS",
		})

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
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
