package services

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"guardian.com/server/agentclient"
)

// EnsureSessionColumns, eski kurulumlarda sessions tablosunda bulunmayabilecek
// terminal boyutu kolonlarını (cols/rows) idempotent şekilde ekler.
func EnsureSessionColumns(db *sql.DB) error {
	stmts := []string{
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS cols integer`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS rows integer`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("sessions kolonları eklenemedi: %w", err)
		}
	}
	return nil
}

func UpdateAndTerminateSession(db *sql.DB, ac agentclient.AgentCommunicator, sessionID int, newStatus string, r *http.Request) error {
	var agentIP string
	var currentStatus string
	query := `SELECT sv.ip_address, s.status FROM sessions s JOIN servers sv ON s.server_id = sv.id WHERE s.id = $1`

	err := db.QueryRow(query, sessionID).Scan(&agentIP, &currentStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("oturum ID %d bulunamadı", sessionID)
		}
		return fmt.Errorf("agent IP alınamadı: %w", err)
	}

	if currentStatus != "active" {
		log.Printf("Oturum %d zaten aktif değil (durumu: %s), sonlandırma işlemi atlanıyor.", sessionID, currentStatus)
		return nil
	}

	updateQuery := `UPDATE sessions SET status = $1, end_time = NOW() AT TIME ZONE 'utc' WHERE id = $2`
	_, err = db.Exec(updateQuery, newStatus, sessionID)
	if err != nil {
		return fmt.Errorf("oturum durumu güncellenemedi: %w", err)
	}
	log.Printf("✅ DB güncellendi: Oturum %d durumu '%s' olarak ayarlandı.", sessionID, newStatus)

	if r != nil {
		Record(db, r, AuditLog{
			Action:     ActionTerminateSess,
			TargetType: "session",
			TargetID:   sessionID,
			Status:     "SUCCESS",
		})
	}

	go func() {
		if err := ac.TerminateSession(agentIP, sessionID); err != nil {
			log.Printf("[ERROR] Agent'a terminate komutu gönderilemedi (SessionID %d): %v", sessionID, err)
		}
	}()

	return nil
}
