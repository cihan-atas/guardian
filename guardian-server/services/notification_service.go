// guardian/guardian-server/services/notification_service.go
//
// Dış bildirim katmanı: webhook (Slack/Discord/Teams uyumlu JSON) ve/veya
// SMTP e-posta. Tümü opsiyoneldir ve ortam değişkenleriyle yapılandırılır;
// hiçbiri ayarlı değilse bildirimler sessizce atlanır. Gönderim fire-and-forget
// (goroutine) yapılır ki olay akışını (WS/DB) hiçbir şekilde bloke etmesin.

package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

// NotifyConfig, bildirim kanallarının yapılandırması.
type NotifyConfig struct {
	WebhookURL string

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string
	EmailTo  string
}

// NotifyEvent, tek bir bildirilebilir olay.
type NotifyEvent struct {
	Kind    string // "risky_command" | "session_start" | "session_end" | "key_ban"
	Title   string
	Text    string
	Level   string // "info" | "warning" | "critical"
	Session int    // ilgili oturum (varsa)
}

var notifyCfg NotifyConfig

// ConfigureNotifier, başlangıçta bir kez çağrılır.
func ConfigureNotifier(cfg NotifyConfig) {
	notifyCfg = cfg
	var on []string
	if cfg.WebhookURL != "" {
		on = append(on, "webhook")
	}
	if cfg.SMTPHost != "" && cfg.EmailTo != "" {
		on = append(on, "e-posta")
	}
	if len(on) == 0 {
		log.Println("[notify] Dış bildirim kanalı yapılandırılmadı (yalnızca uygulama içi uyarı).")
	} else {
		log.Printf("[notify] Etkin bildirim kanalları: %s", strings.Join(on, ", "))
	}
}

// Notify, olayı yapılandırılmış tüm kanallara (varsa) asenkron gönderir.
func Notify(ev NotifyEvent) {
	if notifyCfg.WebhookURL != "" {
		go sendWebhook(ev)
	}
	if notifyCfg.SMTPHost != "" && notifyCfg.EmailTo != "" {
		go sendEmail(ev)
	}
}

func sendWebhook(ev NotifyEvent) {
	// Slack/Discord/Teams "text" alanını ortak kabul eder; ek alanlar da
	// gönderilir ki kendi servisini kullananlar yapılandırılmış veriyi görsün.
	emoji := map[string]string{"critical": "🚨", "warning": "⚠️", "info": "ℹ️"}[ev.Level]
	payload := map[string]interface{}{
		"text":      fmt.Sprintf("%s *Guardian — %s*\n%s", emoji, ev.Title, ev.Text),
		"kind":      ev.Kind,
		"level":     ev.Level,
		"sessionId": ev.Session,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodPost, notifyCfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[notify] Webhook isteği oluşturulamadı: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[notify] Webhook gönderilemedi: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("[notify] Webhook beklenmedik durum kodu: %d", resp.StatusCode)
	}
}

func sendEmail(ev NotifyEvent) {
	from := notifyCfg.SMTPFrom
	if from == "" {
		from = notifyCfg.SMTPUser
	}
	to := strings.Split(notifyCfg.EmailTo, ",")
	for i := range to {
		to[i] = strings.TrimSpace(to[i])
	}

	subject := fmt.Sprintf("[Guardian] %s", ev.Title)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		from, strings.Join(to, ", "), subject, ev.Text)

	addr := notifyCfg.SMTPHost + ":" + notifyCfg.SMTPPort
	var auth smtp.Auth
	if notifyCfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", notifyCfg.SMTPUser, notifyCfg.SMTPPass, notifyCfg.SMTPHost)
	}
	if err := smtp.SendMail(addr, auth, from, to, []byte(msg)); err != nil {
		log.Printf("[notify] E-posta gönderilemedi: %v", err)
	}
}
