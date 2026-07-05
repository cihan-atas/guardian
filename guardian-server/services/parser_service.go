// guardian/guardian-server/services/parser_service.go

package services

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode"
)

type SessionDetails struct {
	SessionInfo SessionMetadata `json:"session_info"`
	Commands    []ParsedCommand `json:"commands"`
}

type SessionMetadata struct {
	ID             int        `json:"id"`
	Username       string     `json:"username"`
	ServerHostname string     `json:"server_hostname"`
	ServerIP       string     `json:"server_ip"`
	StartTime      time.Time  `json:"start_time"`
	EndTime        *time.Time `json:"end_time,omitempty"`
	Status         string     `json:"status"`
}

type ParsedCommand struct {
	Timestamp time.Time `json:"timestamp"`
	Command   string    `json:"command"`
	Output    string    `json:"output"`
}

// squash, bir SQL sorgusundaki tüm satır sonlarını ve
// ardışık boşlukları tek bir boşluğa indirger.
func squash(text string) string {
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(text, " "))
}

func ParseSessionEvents(db *sql.DB, sessionID int) (*SessionDetails, error) {
	var meta SessionMetadata
	metaQuery := squash(`
		SELECT 
			s.id, s.username, s.start_time, s.end_time, s.status, 
			sv.hostname, sv.ip_address
		FROM sessions s
		JOIN servers sv ON s.server_id = sv.id
		WHERE s.id = $1`)

	err := db.QueryRow(metaQuery, sessionID).Scan(
		&meta.ID, &meta.Username, &meta.StartTime, &meta.EndTime, &meta.Status,
		&meta.ServerHostname, &meta.ServerIP,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session_id %d bulunamadı", sessionID)
		}
		return nil, fmt.Errorf("oturum metaverisi alınamadı: %w", err)
	}

	query := squash(`SELECT event_type, data, event_time FROM session_events WHERE session_id = $1 ORDER BY event_time ASC, id ASC`)
	rows, err := db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("veritabanından olaylar çekilemedi: %w", err)
	}
	defer rows.Close()

	var commands []ParsedCommand
	var currentInput []rune
	var currentOutput strings.Builder
	var pendingTimestamp time.Time

	for rows.Next() {
		var eventType string
		var data []byte
		var eventTime time.Time
		if err := rows.Scan(&eventType, &data, &eventTime); err != nil {
			log.Printf("Olay verisi okunurken hata (atlanıyor): %v", err)
			continue
		}

		if eventType == "input" {
			if len(currentInput) == 0 {
				pendingTimestamp = eventTime
			}
			for _, r := range string(data) {
				switch r {
				case '\r', '\n':
					command := cleanString(string(currentInput))
					if len(command) > 0 {
						if len(commands) > 0 {
							// Bir önceki komutun çıktısını tamamla
							commands[len(commands)-1].Output = cleanString(currentOutput.String())
						}
						// Yeni komutu ekle
						commands = append(commands, ParsedCommand{
							Timestamp: pendingTimestamp,
							Command:   command,
						})
						currentOutput.Reset()
					}
					currentInput = currentInput[:0]
				case 0x7F, 0x08: // Backspace/Delete: son karakteri geri al
					if len(currentInput) > 0 {
						currentInput = currentInput[:len(currentInput)-1]
					}
				default:
					currentInput = append(currentInput, r)
				}
			}
			continue
		}

		if eventType == "output" {
			currentOutput.Write(data)
		}
	}

	if len(commands) > 0 {
		commands[len(commands)-1].Output = cleanString(currentOutput.String())
	}

	details := &SessionDetails{
		SessionInfo: meta,
		Commands:    commands,
	}

	return details, nil
}

func cleanString(s string) string {
	var result strings.Builder
	inEscapeCode := false
	for i, r := range s {
		if r == 0x1B && i+1 < len(s) && s[i+1] == '[' {
			inEscapeCode = true
			continue
		}
		if inEscapeCode {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscapeCode = false
			}
			continue
		}
		if unicode.IsPrint(r) {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}
