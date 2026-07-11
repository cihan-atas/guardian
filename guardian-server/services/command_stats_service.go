// guardian/guardian-server/services/command_stats_service.go

package services

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

type CommandCount struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// CommandStat, TAM komut satırı bazında (örn. "ls -la") kullanım istatistiği.
// Servers, komutun hangi sunucuda kaç kez çalıştığını tutar → UI'da sunucu
// filtresi bu harita üzerinden anlık (yeniden sorgu olmadan) uygulanır.
type CommandStat struct {
	Command string         `json:"command"` // tam komut satırı
	Base    string         `json:"base"`    // ilk kelime (örn. "ls")
	Count   int            `json:"count"`   // toplam çalıştırma sayısı
	Servers map[string]int `json:"servers"` // hostname -> sayı
}

// isMeaningfulCommand, escape temizliğinden arta kalan çöp parçalarını eler:
// içinde en az bir harf olmayan girdileri (örn. tek işaret, sayı-only artıklar)
// komut saymayız.
func isMeaningfulCommand(cmd string) bool {
	for _, r := range cmd {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// extractCommandsForSession, bir oturumun "input" olaylarından çalıştırılan
// komutları çıkarır. ParseSessionEvents ile aynı ayrıştırma mantığını
// kullanır ama metaveri sorgusu yapmadan sadece komut listesini döner
// (toplu istatistik hesaplarken gereksiz JOIN'lerden kaçınmak için).
func extractCommandsForSession(db *sql.DB, sessionID int) ([]string, error) {
	query := squash(`SELECT event_type, data FROM session_events WHERE session_id = $1 AND event_type = 'input' ORDER BY event_time ASC, id ASC`)
	rows, err := db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("oturum olayları alınamadı: %w", err)
	}
	defer rows.Close()

	var commands []string
	var currentInput []rune

	for rows.Next() {
		var eventType string
		var data []byte
		if err := rows.Scan(&eventType, &data); err != nil {
			continue
		}
		for _, r := range string(data) {
			switch r {
			case '\r', '\n':
				command := cleanString(string(currentInput))
				if len(command) > 0 {
					commands = append(commands, command)
				}
				currentInput = currentInput[:0]
			case 0x7F, 0x08:
				if len(currentInput) > 0 {
					currentInput = currentInput[:len(currentInput)-1]
				}
			default:
				currentInput = append(currentInput, r)
			}
		}
	}
	return commands, rows.Err()
}

// GetTopCommands, en son çalıştırılmış `sessionLimit` oturuma bakarak en sık
// kullanılan komutları (ilk kelimeye göre, örn. "ls -la" -> "ls") döner.
// Tüm geçmiş oturumları taramak yerine son N oturumla sınırlamak, çok sayıda
// kayıt biriktiğinde N+1 sorgu maliyetinin sınırsız büyümesini önler.
func GetTopCommands(db *sql.DB, sessionLimit, topN int) ([]CommandCount, error) {
	idsQuery := `SELECT id FROM sessions ORDER BY start_time DESC LIMIT $1`
	rows, err := db.Query(idsQuery, sessionLimit)
	if err != nil {
		return nil, fmt.Errorf("oturum ID'leri alınamadı: %w", err)
	}
	var sessionIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil {
			sessionIDs = append(sessionIDs, id)
		}
	}
	rows.Close()

	counts := make(map[string]int)
	for _, sessionID := range sessionIDs {
		commands, err := extractCommandsForSession(db, sessionID)
		if err != nil {
			continue
		}
		for _, cmd := range commands {
			fields := strings.Fields(cmd)
			if len(fields) == 0 {
				continue
			}
			base := fields[0]
			counts[base]++
		}
	}

	result := make([]CommandCount, 0, len(counts))
	for name, value := range counts {
		result = append(result, CommandCount{Name: name, Value: value})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Value > result[j].Value })
	if len(result) > topN {
		result = result[:topN]
	}
	return result, nil
}

// GetCommandStats, son `sessionLimit` oturuma bakarak TAM komut bazında
// kullanım istatistiğini sunucu kırılımıyla döner. Frontend bunu hem baz
// komuta göre gruplayıp drill-down yapar hem de sunucu filtresini istemci
// tarafında (yeniden sorgu olmadan) uygular.
func GetCommandStats(db *sql.DB, sessionLimit int) ([]CommandStat, error) {
	rows, err := db.Query(`
		SELECT s.id, COALESCE(sv.hostname, 'bilinmiyor')
		FROM sessions s
		LEFT JOIN servers sv ON s.server_id = sv.id
		ORDER BY s.start_time DESC
		LIMIT $1`, sessionLimit)
	if err != nil {
		return nil, fmt.Errorf("oturumlar alınamadı: %w", err)
	}
	type sessRef struct {
		id   int
		host string
	}
	var sessions []sessRef
	for rows.Next() {
		var s sessRef
		if err := rows.Scan(&s.id, &s.host); err == nil {
			sessions = append(sessions, s)
		}
	}
	rows.Close()

	agg := make(map[string]*CommandStat)
	for _, s := range sessions {
		commands, err := extractCommandsForSession(db, s.id)
		if err != nil {
			continue
		}
		for _, cmd := range commands {
			if !isMeaningfulCommand(cmd) {
				continue
			}
			fields := strings.Fields(cmd)
			if len(fields) == 0 {
				continue
			}
			st, ok := agg[cmd]
			if !ok {
				st = &CommandStat{Command: cmd, Base: fields[0], Servers: map[string]int{}}
				agg[cmd] = st
			}
			st.Count++
			st.Servers[s.host]++
		}
	}

	result := make([]CommandStat, 0, len(agg))
	for _, st := range agg {
		result = append(result, *st)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	return result, nil
}
