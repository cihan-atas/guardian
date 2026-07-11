package services

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// CommandMatch, global komut aramasında tek bir eşleşme.
// CommandIndex, oturum içindeki komutun sırasıdır ve ParseSessionEvents'in
// ürettiği Commands dizisiyle birebir hizalıdır → replay'e derin link
// (?cmd=<index>) bu indeksi kullanır.
type CommandMatch struct {
	SessionID      int       `json:"session_id"`
	CommandIndex   int       `json:"command_index"`
	Command        string    `json:"command"`
	Username       string    `json:"username"`
	ServerHostname string    `json:"server_hostname"`
	StartTime      time.Time `json:"start_time"`
	Status         string    `json:"status"`
}

// SearchCommands, en son `sessionLimit` oturumu tarayarak komut metninde
// `query` (büyük/küçük harf duyarsız alt dize) geçen çalıştırmaları döndürür.
// En fazla `maxResults` eşleşme döner (en yeni oturumdan başlayarak).
func SearchCommands(db *sql.DB, query string, sessionLimit, maxResults int) ([]CommandMatch, error) {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return []CommandMatch{}, nil
	}

	type candidate struct {
		id        int
		username  string
		hostname  string
		startTime time.Time
		status    string
	}

	rows, err := db.Query(squash(`
		SELECT s.id, s.username, sv.hostname, s.start_time, s.status
		FROM sessions s
		JOIN servers sv ON s.server_id = sv.id
		ORDER BY s.start_time DESC
		LIMIT $1`), sessionLimit)
	if err != nil {
		return nil, fmt.Errorf("oturumlar alınamadı: %w", err)
	}
	defer rows.Close()

	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.username, &c.hostname, &c.startTime, &c.status); err != nil {
			continue
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	matches := []CommandMatch{}
	for _, c := range candidates {
		cmds, err := extractCommandsForSession(db, c.id)
		if err != nil {
			continue
		}
		for idx, cmd := range cmds {
			if strings.Contains(strings.ToLower(cmd), needle) {
				matches = append(matches, CommandMatch{
					SessionID:      c.id,
					CommandIndex:   idx,
					Command:        cmd,
					Username:       c.username,
					ServerHostname: c.hostname,
					StartTime:      c.startTime,
					Status:         c.status,
				})
				if len(matches) >= maxResults {
					return matches, nil
				}
			}
		}
	}
	return matches, nil
}
