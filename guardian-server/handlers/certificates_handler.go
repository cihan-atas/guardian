package handlers

import (
	"crypto/x509"
	"database/sql"
	"net/http"
	"sync"

	"guardian.com/server/services"
)

// AgentCertReader, agent'ın sunduğu sertifikayı okuyan minimal arayüz
// (agentclient.Client sağlar).
type AgentCertReader interface {
	PeerCertificate(ip string) (*x509.Certificate, error)
}

type agentCertInfo struct {
	ServerID  int                 `json:"server_id"`
	Hostname  string              `json:"hostname"`
	IPAddress string              `json:"ip_address"`
	Online    bool                `json:"online"`
	Cert      *services.CertInfo  `json:"cert,omitempty"`
}

// Certificates (admin): CA + Guardian server sertifikalarını (yerel dosyalar)
// ve her sunucunun agent sertifikasını (TLS el sıkışmasıyla) süre-sonu bilgisiyle
// döndürür. GET /api/certificates
func Certificates(db *sql.DB, ac AgentCertReader, caCertPath, serverCertPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{}

		if ca, err := services.ReadCertInfo(caCertPath); err == nil {
			resp["ca"] = ca
		} else {
			resp["ca_error"] = err.Error()
		}
		if sc, err := services.ReadCertInfo(serverCertPath); err == nil {
			resp["server"] = sc
		} else {
			resp["server_error"] = err.Error()
		}

		// Agent sertifikaları — her sunucuyu paralel yokla.
		rows, err := db.Query(`SELECT id, hostname, ip_address FROM servers ORDER BY id ASC`)
		if err == nil {
			defer rows.Close()
			var agents []agentCertInfo
			for rows.Next() {
				var a agentCertInfo
				if err := rows.Scan(&a.ServerID, &a.Hostname, &a.IPAddress); err == nil {
					agents = append(agents, a)
				}
			}

			var wg sync.WaitGroup
			for i := range agents {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					cert, err := ac.PeerCertificate(agents[idx].IPAddress)
					if err == nil && cert != nil {
						agents[idx].Online = true
						agents[idx].Cert = services.CertInfoFromX509(cert)
					}
				}(i)
			}
			wg.Wait()

			if agents == nil {
				agents = []agentCertInfo{}
			}
			resp["agents"] = agents
		}

		writeJSON(w, http.StatusOK, resp)
	}
}
