package handlers

import (
	"database/sql"
	"net/http"
	"sync"
	"time"
)

// AgentPinger, sunucu sağlık kontrolü için ihtiyaç duyulan minimal arayüz.
// agentclient.Client bunu sağlar; AgentCommunicator arayüzünü (ve onun
// testlerdeki mock'larını) değiştirmemek için ayrı tutulur.
type AgentPinger interface {
	Ping(ip string) error
}

type serverHealth struct {
	ServerID  int    `json:"server_id"`
	Hostname  string `json:"hostname"`
	IPAddress string `json:"ip_address"`
	Online    bool   `json:"online"`
	LatencyMs int64  `json:"latency_ms"`
}

// GetServersHealth, tüm sunucuların agent'larını paralel olarak pingler ve
// çevrimiçi/çevrimdışı durumunu + gecikmeyi döndürür. Örnek (example-) sunucu
// da dahildir; UI istatistiği bozmadan gösterir.
func GetServersHealth(db *sql.DB, pinger AgentPinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`SELECT id, hostname, ip_address FROM servers ORDER BY id ASC`)
		if err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var results []serverHealth
		for rows.Next() {
			var h serverHealth
			if err := rows.Scan(&h.ServerID, &h.Hostname, &h.IPAddress); err != nil {
				http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
				return
			}
			results = append(results, h)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			return
		}

		var wg sync.WaitGroup
		for i := range results {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				start := time.Now()
				if err := pinger.Ping(results[idx].IPAddress); err == nil {
					results[idx].Online = true
					results[idx].LatencyMs = time.Since(start).Milliseconds()
				}
			}(i)
		}
		wg.Wait()

		if results == nil {
			results = []serverHealth{}
		}
		writeJSON(w, http.StatusOK, results)
	}
}
