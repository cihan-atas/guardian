// guardian/guardian-server/services/alert_service.go
//
// Riskli komut tespiti bir "alert" üretir. Alert şunları yapar:
//   1. alerts tablosuna kalıcı kayıt,
//   2. o oturumu canlı izleyen tarayıcılara WebSocket üzerinden anlık uyarı,
//   3. yapılandırılmış dış kanallara (webhook/e-posta) bildirim,
//   4. (opsiyonel) kritik ciddiyette otomatik aksiyon: oturumu sonlandır veya
//      anahtarı yasakla.

package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"guardian.com/server/agentclient"
	"guardian.com/server/hub"
)

// AutoAction, kritik riskli komutta alınacak otomatik aksiyon.
type AutoAction string

const (
	AutoActionNone      AutoAction = "none"
	AutoActionTerminate AutoAction = "terminate"
	AutoActionBan       AutoAction = "ban"
)

// autoAction, yalnızca "critical" ciddiyetli eşleşmelerde uygulanır.
// Varsayılan: none ("sadece uyar").
var autoAction = AutoActionNone

// autoBanMinutes, oto-aksiyon "ban" olduğunda uygulanacak yasak süresi.
var autoBanMinutes = 60

// ConfigureAlerting, başlangıçta bir kez çağrılır.
func ConfigureAlerting(action AutoAction, banMinutes int) {
	switch action {
	case AutoActionTerminate, AutoActionBan:
		autoAction = action
	default:
		autoAction = AutoActionNone
	}
	if banMinutes > 0 {
		autoBanMinutes = banMinutes
	}
	log.Printf("[alert] Riskli komut oto-aksiyonu: %s", autoAction)
}

// Alert, kalıcı uyarı kaydı.
type Alert struct {
	ID          int       `json:"id"`
	SessionID   int       `json:"session_id"`
	Severity    string    `json:"severity"`
	RuleName    string    `json:"rule_name"`
	Command     string    `json:"command"`
	ActionTaken string    `json:"action_taken"`
	CreatedAt   time.Time `json:"created_at"`
	// Liste görünümü için oturum meta bilgisi (JOIN'den).
	Username       string `json:"username,omitempty"`
	ServerHostname string `json:"server_hostname,omitempty"`
}

// EnsureAlertsTable, alerts tablosunu yoksa oluşturur (var olan kurulumlarda
// elle migration gerektirmesin diye başlangıçta çağrılır).
func EnsureAlertsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS alerts (
			id           SERIAL PRIMARY KEY,
			session_id   INTEGER,
			severity     TEXT NOT NULL,
			rule_name    TEXT NOT NULL,
			command      TEXT NOT NULL,
			action_taken TEXT NOT NULL DEFAULT 'none',
			created_at   TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'utc')
		)`)
	return err
}

// RaiseAlert, tespit edilen riskli komut için tüm alert akışını yürütür.
func RaiseAlert(db *sql.DB, h *hub.Hub, ac agentclient.AgentCommunicator, sessionID int, match *RiskyMatch) {
	if match == nil {
		return
	}

	actionTaken := "none"
	if match.Severity == SeverityCritical {
		actionTaken = applyAutoAction(db, ac, sessionID)
	}

	// 1) Kalıcı kayıt.
	var alertID int
	var createdAt time.Time
	err := db.QueryRow(
		`INSERT INTO alerts (session_id, severity, rule_name, command, action_taken)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		sessionID, string(match.Severity), match.RuleName, match.Command, actionTaken,
	).Scan(&alertID, &createdAt)
	if err != nil {
		log.Printf("[alert] Uyarı kaydedilemedi (Session %d): %v", sessionID, err)
	}

	log.Printf("🚨 [alert] Riskli komut (Session %d, %s): %q [aksiyon: %s]",
		sessionID, match.Severity, match.Command, actionTaken)

	// 2) Canlı izleyicilere WS üzerinden anlık uyarı.
	if h != nil {
		payload, _ := json.Marshal(map[string]interface{}{
			"type":      "alert",
			"severity":  string(match.Severity),
			"rule":      match.RuleName,
			"command":   match.Command,
			"action":    actionTaken,
			"timestamp": createdAt.Format(time.RFC3339),
		})
		h.Broadcast <- &hub.BroadcastMessage{SessionID: sessionID, Data: payload}
	}

	// 3) Dış bildirim.
	level := "warning"
	if match.Severity == SeverityCritical {
		level = "critical"
	}
	text := fmt.Sprintf("Oturum #%d içinde riskli komut tespit edildi:\n  %s\nKural: %s (%s)",
		sessionID, match.Command, match.RuleName, match.Severity)
	if actionTaken != "none" {
		text += fmt.Sprintf("\nOtomatik aksiyon: %s", actionTaken)
	}
	Notify(NotifyEvent{
		Kind:    "risky_command",
		Title:   fmt.Sprintf("Riskli komut (#%d)", sessionID),
		Text:    text,
		Level:   level,
		Session: sessionID,
	})
}

// applyAutoAction, kritik komutta yapılandırılmış otomatik aksiyonu uygular
// ve alınan aksiyonun adını döner.
func applyAutoAction(db *sql.DB, ac agentclient.AgentCommunicator, sessionID int) string {
	switch autoAction {
	case AutoActionTerminate:
		if err := UpdateAndTerminateSession(db, ac, sessionID, "terminated_by_admin", nil); err != nil {
			log.Printf("[alert] Oto-sonlandırma başarısız (Session %d): %v", sessionID, err)
			return "none"
		}
		return "terminate"

	case AutoActionBan:
		var publicKeyID sql.NullInt64
		err := db.QueryRow(
			`SELECT ar.public_key_id FROM sessions s
			 JOIN access_rules ar ON s.rule_id = ar.id WHERE s.id = $1`, sessionID,
		).Scan(&publicKeyID)
		if err != nil || !publicKeyID.Valid {
			log.Printf("[alert] Oto-ban için anahtar bulunamadı (Session %d): %v", sessionID, err)
			// Anahtar bulunamasa da en azından oturumu kes.
			if terr := UpdateAndTerminateSession(db, ac, sessionID, "terminated_by_admin", nil); terr == nil {
				return "terminate"
			}
			return "none"
		}
		if _, err := BanKey(db, ac, int(publicKeyID.Int64),
			time.Duration(autoBanMinutes)*time.Minute, "otomatik: kritik riskli komut"); err != nil {
			log.Printf("[alert] Oto-ban başarısız (Session %d): %v", sessionID, err)
			return "none"
		}
		return "ban"
	}
	return "none"
}

// RecentAlerts, en son uyarıları oturum meta bilgisiyle döner.
func RecentAlerts(db *sql.DB, limit int) ([]Alert, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.Query(`
		SELECT a.id, COALESCE(a.session_id, 0), a.severity, a.rule_name, a.command, a.action_taken, a.created_at,
		       COALESCE(s.username, ''), COALESCE(sv.hostname, '')
		FROM alerts a
		LEFT JOIN sessions s ON a.session_id = s.id
		LEFT JOIN servers sv ON s.server_id = sv.id
		ORDER BY a.id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	alerts := make([]Alert, 0)
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.SessionID, &a.Severity, &a.RuleName, &a.Command,
			&a.ActionTaken, &a.CreatedAt, &a.Username, &a.ServerHostname); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}
