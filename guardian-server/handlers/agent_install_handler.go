package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"guardian.com/server/services"
)

// AgentInstaller, UI'dan agent kurulumu için gereken yapılandırmayı tutar ve
// ilgili HTTP handler'larını üretir.
type AgentInstaller struct {
	DB          *sql.DB
	CA          *services.CA // nil ise enrollment devre dışı (ca.key yok)
	BinaryPath  string       // servis edilecek guardian-agent binary yolu
	SecretToken string       // GUARDIAN_SECRET_TOKEN (agent.conf'a gömülür)
	ServerPort  string       // GUARDIAN_SERVER_PORT (agent'ın bağlanacağı)
	AgentPort   string       // GUARDIAN_AGENT_PORT
	PublicURL   string       // GUARDIAN_PUBLIC_URL: agent'ların sunucuya eriştiği taban URL
}

// baseURL, script/komutlarda kullanılacak taban URL'yi döndürür. Öncelik
// GUARDIAN_PUBLIC_URL; yoksa istek Host'undan türetilir (best-effort).
func (a *AgentInstaller) baseURL(r *http.Request) string {
	if a.PublicURL != "" {
		return strings.TrimRight(a.PublicURL, "/")
	}
	return "https://" + r.Host
}

// enrollTokenFromRequest, token'ı header (X-Enroll-Token) veya ?token= query'den okur.
func enrollTokenFromRequest(r *http.Request) string {
	if t := strings.TrimSpace(r.Header.Get("X-Enroll-Token")); t != "" {
		return t
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

// GenerateEnrollToken (admin): bir sunucu için kayıt token'ı + kurulum komutu üretir.
// POST /api/servers/{serverID}/enroll-token
func (a *AgentInstaller) GenerateEnrollToken() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serverID, err := strconv.Atoi(chi.URLParam(r, "serverID"))
		if err != nil {
			http.Error(w, "Geçersiz sunucu ID.", http.StatusBadRequest)
			return
		}
		var hostname, ip string
		if err := a.DB.QueryRow(`SELECT hostname, ip_address FROM servers WHERE id = $1`, serverID).
			Scan(&hostname, &ip); err != nil {
			http.Error(w, "Sunucu bulunamadı.", http.StatusNotFound)
			return
		}
		if a.CA == nil {
			http.Error(w, "Sunucuda CA anahtarı (ca.key) yapılandırılmadığı için otomatik kurulum devre dışı. TLS_CA_KEY_FILE ayarlayın.", http.StatusServiceUnavailable)
			return
		}

		token, expiresAt, err := services.CreateEnrollToken(a.DB, serverID)
		if err != nil {
			http.Error(w, "Kayıt token'ı oluşturulamadı.", http.StatusInternalServerError)
			return
		}
		services.Record(a.DB, r, services.AuditLog{Action: services.ActionRequestAccess, TargetType: "agent_enroll", TargetID: serverID, Status: "SUCCESS"})

		base := a.baseURL(r)
		installCmd := fmt.Sprintf(`curl -fsSLk -H "X-Enroll-Token: %s" %s/api/agent/install.sh | sudo bash`, token, base)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"token":           token,
			"expires_at":      expiresAt,
			"server_id":       serverID,
			"server_hostname": hostname,
			"server_ip":       ip,
			"base_url":        base,
			"install_command": installCmd,
			"binary_available": a.binaryAvailable(),
		})
	}
}

// ServeInstallScript (token): kurulum script'ini üretip döndürür.
// GET /api/agent/install.sh   (X-Enroll-Token veya ?token=)
func (a *AgentInstaller) ServeInstallScript() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := enrollTokenFromRequest(r)
		serverID, err := services.ValidateEnrollToken(a.DB, token)
		if err != nil {
			http.Error(w, "# Kayıt token'ı geçersiz veya süresi dolmuş.\n", http.StatusUnauthorized)
			return
		}
		serverHost := a.agentServerHost(r)
		script := renderInstallScript(a.baseURL(r), serverHost, a.ServerPort, a.AgentPort, a.SecretToken, token, serverID)
		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		w.Write([]byte(script))
	}
}

// EnrollAgent (token): agent CSR'ını imzalar, sertifikayı PEM olarak döndürür.
// POST /api/agent/enroll
func (a *AgentInstaller) EnrollAgent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.CA == nil {
			http.Error(w, "Enrollment devre dışı (ca.key yok).", http.StatusServiceUnavailable)
			return
		}
		token := enrollTokenFromRequest(r)
		serverID, err := services.ValidateEnrollToken(a.DB, token)
		if err != nil {
			http.Error(w, "Kayıt token'ı geçersiz veya süresi dolmuş.", http.StatusUnauthorized)
			return
		}

		var ip string
		if err := a.DB.QueryRow(`SELECT ip_address FROM servers WHERE id = $1`, serverID).Scan(&ip); err != nil {
			http.Error(w, "Sunucu bulunamadı.", http.StatusNotFound)
			return
		}

		csrPEM, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		if err != nil || len(csrPEM) == 0 {
			http.Error(w, "CSR okunamadı.", http.StatusBadRequest)
			return
		}

		var ips []net.IP
		if parsed := net.ParseIP(ip); parsed != nil {
			ips = append(ips, parsed)
		}
		certPEM, err := a.CA.SignAgentCSR(csrPEM, ips, nil)
		if err != nil {
			http.Error(w, "Sertifika imzalanamadı: "+err.Error(), http.StatusBadRequest)
			return
		}

		// İlk kullanımı denetim için işaretle (token TTL boyunca ca.crt + binary
		// indirmeleri için geçerli kalmalı; bu yüzden geçersiz kılmıyoruz).
		services.MarkEnrollTokenUsed(a.DB, token)
		services.Record(a.DB, r, services.AuditLog{Action: services.ActionCreateServer, TargetType: "agent_enroll", TargetID: serverID, Status: "SUCCESS"})

		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Write(certPEM)
	}
}

// ServeCACert (token): CA sertifikasını döndürür.
// GET /api/agent/ca.crt
func (a *AgentInstaller) ServeCACert() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.CA == nil {
			http.Error(w, "CA yapılandırılmadı.", http.StatusServiceUnavailable)
			return
		}
		if _, err := services.ValidateEnrollToken(a.DB, enrollTokenFromRequest(r)); err != nil {
			http.Error(w, "Kayıt token'ı geçersiz.", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Write(a.CA.CACertPEM())
	}
}

// ServeBinary (token): guardian-agent binary'sini döndürür.
// GET /api/agent/binary
func (a *AgentInstaller) ServeBinary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := services.ValidateEnrollToken(a.DB, enrollTokenFromRequest(r)); err != nil {
			http.Error(w, "Kayıt token'ı geçersiz.", http.StatusUnauthorized)
			return
		}
		if !a.binaryAvailable() {
			http.Error(w, "Agent binary sunucuda bulunamadı. GUARDIAN_AGENT_BINARY_PATH ayarlayın.", http.StatusServiceUnavailable)
			return
		}
		f, err := os.Open(a.BinaryPath)
		if err != nil {
			http.Error(w, "Binary açılamadı.", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "application/octet-stream")
		io.Copy(w, f)
	}
}

func (a *AgentInstaller) binaryAvailable() bool {
	if a.BinaryPath == "" {
		return false
	}
	info, err := os.Stat(a.BinaryPath)
	return err == nil && !info.IsDir()
}

// agentServerHost, agent.conf'a yazılacak GUARDIAN_SERVER_HOST'u (şema + host,
// port hariç) baseURL'den türetir.
func (a *AgentInstaller) agentServerHost(r *http.Request) string {
	base := a.baseURL(r)
	// Portu at (agent.conf ayrı GUARDIAN_SERVER_PORT kullanır).
	if i := strings.Index(base, "://"); i != -1 {
		scheme := base[:i]
		rest := base[i+3:]
		if h, _, err := net.SplitHostPort(rest); err == nil {
			return scheme + "://" + h
		}
		return base
	}
	return base
}
