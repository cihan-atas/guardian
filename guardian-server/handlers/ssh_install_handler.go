package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/ssh"
	"guardian.com/server/services"
)

type sshInstallRequest struct {
	SSHHost    string `json:"ssh_host"`
	SSHPort    string `json:"ssh_port"`
	SSHUser    string `json:"ssh_user"`
	SSHPass    string `json:"ssh_password"`
	PrivateKey string `json:"ssh_private_key"`
}

// SSHInstall (admin): hedef sunucuya SSH ile bağlanıp kurulum script'ini
// uzaktan çalıştırır. Kimlik bilgileri saklanmaz; yalnızca bu istek için
// kullanılır. POST /api/servers/{serverID}/ssh-install
func (a *AgentInstaller) SSHInstall() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serverID, err := strconv.Atoi(chi.URLParam(r, "serverID"))
		if err != nil {
			http.Error(w, "Geçersiz sunucu ID.", http.StatusBadRequest)
			return
		}
		if a.CA == nil {
			http.Error(w, "Otomatik kurulum devre dışı (ca.key yok). TLS_CA_KEY_FILE ayarlayın.", http.StatusServiceUnavailable)
			return
		}
		if !a.binaryAvailable() {
			http.Error(w, "Agent binary sunucuda bulunamadı. GUARDIAN_AGENT_BINARY_PATH ayarlayın.", http.StatusServiceUnavailable)
			return
		}

		var req sshInstallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Geçersiz istek gövdesi.", http.StatusBadRequest)
			return
		}

		var serverIP string
		if err := a.DB.QueryRow(`SELECT ip_address FROM servers WHERE id = $1`, serverID).Scan(&serverIP); err != nil {
			http.Error(w, "Sunucu bulunamadı.", http.StatusNotFound)
			return
		}

		host := strings.TrimSpace(req.SSHHost)
		if host == "" {
			host = serverIP
		}
		port := strings.TrimSpace(req.SSHPort)
		if port == "" {
			port = "22"
		}
		user := strings.TrimSpace(req.SSHUser)
		if user == "" {
			http.Error(w, "SSH kullanıcı adı gereklidir.", http.StatusBadRequest)
			return
		}

		authMethods := []ssh.AuthMethod{}
		if strings.TrimSpace(req.PrivateKey) != "" {
			signer, err := ssh.ParsePrivateKey([]byte(req.PrivateKey))
			if err != nil {
				http.Error(w, "Özel anahtar çözümlenemedi: "+err.Error(), http.StatusBadRequest)
				return
			}
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
		if req.SSHPass != "" {
			authMethods = append(authMethods, ssh.Password(req.SSHPass))
		}
		if len(authMethods) == 0 {
			http.Error(w, "SSH parolası veya özel anahtar gereklidir.", http.StatusBadRequest)
			return
		}

		cfg := &ssh.ClientConfig{
			User:            user,
			Auth:            authMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(), // hedef admin tarafından belirtiliyor
			Timeout:         15 * time.Second,
		}

		client, err := ssh.Dial("tcp", host+":"+port, cfg)
		if err != nil {
			http.Error(w, "SSH bağlantısı kurulamadı: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer client.Close()

		// Kayıt token'ı üret ve tek satırlık kurulum komutunu oluştur.
		token, _, err := services.CreateEnrollToken(a.DB, serverID)
		if err != nil {
			http.Error(w, "Kayıt token'ı oluşturulamadı.", http.StatusInternalServerError)
			return
		}
		base := a.baseURL(r)
		sudo := "sudo "
		if user == "root" {
			sudo = ""
		}
		cmd := fmt.Sprintf(`curl -fsSLk -H "X-Enroll-Token: %s" %s/api/agent/install.sh | %sbash`, token, base, sudo)

		output, runErr := runSSHCommand(client, cmd, 150*time.Second)
		status := "SUCCESS"
		if runErr != nil {
			status = "FAILURE"
		}
		services.Record(a.DB, r, services.AuditLog{Action: services.ActionCreateServer, TargetType: "agent_ssh_install", TargetID: serverID, Status: status})

		resp := map[string]interface{}{
			"success": runErr == nil,
			"output":  output,
		}
		if runErr != nil {
			resp["error"] = runErr.Error()
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// runSSHCommand, komutu çalıştırır ve birleşik çıktıyı (stdout+stderr) döndürür;
// timeout aşılırsa hata döner.
func runSSHCommand(client *ssh.Client, cmd string, timeout time.Duration) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("SSH oturumu açılamadı: %w", err)
	}
	defer session.Close()

	type result struct {
		out []byte
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := session.CombinedOutput(cmd)
		done <- result{out, err}
	}()

	select {
	case res := <-done:
		return string(res.out), res.err
	case <-time.After(timeout):
		session.Signal(ssh.SIGKILL)
		return "", fmt.Errorf("kurulum zaman aşımına uğradı (%s)", timeout)
	}
}
