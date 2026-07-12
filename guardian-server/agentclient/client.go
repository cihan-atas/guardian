package agentclient

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"guardian.com/server/models"
)

type Client struct {
	httpClient  *http.Client
	agentPort   string
	secretToken string
}

func New(agentPort, secretToken, caCertFile string) *Client {
	caCert, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		log.Fatalf("FATAL: CA sertifikası okunamadı (%s): %v", caCertFile, err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}

	return &Client{
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
		agentPort:   agentPort,
		secretToken: secretToken,
	}
}

func (c *Client) sendCommand(ip, action string, payload interface{}) error {
	endpoint := fmt.Sprintf("https://%s:%s/actions/%s", ip, c.agentPort, action)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("payload JSON'a çevrilemedi: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("istek oluşturulamadı: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.secretToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("agent'a istek gönderilemedi: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent beklenmeyen durum kodu döndü: %s", resp.Status)
	}

	log.Printf("✅ Agent'a komut başarıyla gönderildi: Host: %s, Aksiyon: %s", ip, action)
	return nil
}

func (c *Client) SendKeyCommand(ip, action string, payload models.KeyPayload) error {
	actionEndpoint := fmt.Sprintf("%s-key", action)
	log.Printf("    => Agent'a anahtar komutu gönderiliyor: Host: %s, Aksiyon: %s, Kullanıcı: %s", ip, action, payload.Username)
	return c.sendCommand(ip, actionEndpoint, payload)
}

func (c *Client) TerminateSession(ip string, sessionID int) error {
	payload := map[string]int{"session_id": sessionID}
	log.Printf("    => Agent'a oturum sonlandırma komutu gönderiliyor: Host: %s, SessionID: %d", ip, sessionID)
	return c.sendCommand(ip, "terminate-session", payload)
}

// Ping, agent'ın kimlik doğrulamasız `/status` ucuna kısa timeout'lu bir GET
// isteği atar; agent çevrimiçi ve TLS el sıkışması başarılıysa nil döner.
// Sağlık kontrolü için ana httpClient'tan bağımsız kısa bir timeout kullanır.
func (c *Client) Ping(ip string) error {
	endpoint := fmt.Sprintf("https://%s:%s/status", ip, c.agentPort)

	client := &http.Client{
		Timeout:   3 * time.Second,
		Transport: c.httpClient.Transport,
	}
	resp, err := client.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent %s beklenmeyen durum: %s", ip, resp.Status)
	}
	return nil
}

// PeerCertificate, agent'a TLS el sıkışması yapıp sunduğu yaprak (leaf)
// sertifikayı döndürür. Süresi dolmuş/geçersiz sertifikaları da okuyabilmek
// için doğrulama atlanır (InsecureSkipVerify) — amaç yalnızca süre-sonu bilgisi.
func (c *Client) PeerCertificate(ip string) (*x509.Certificate, error) {
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", ip+":"+c.agentPort, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("agent sertifika sunmadı")
	}
	return certs[0], nil
}

func (c *Client) ValidateUser(ip, username string) error {
	log.Printf("    => Agent'a kullanıcı doğrulama isteği gönderiliyor: Host: %s, Kullanıcı: %s", ip, username)
	payload := map[string]string{"username": username}
	return c.sendCommand(ip, "validate-user", payload)
}
