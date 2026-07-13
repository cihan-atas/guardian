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

func New(agentPort, secretToken, caCertFile, certFile, keyFile string) *Client {
	caCert, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		log.Fatalf("FATAL: CA sertifikası okunamadı (%s): %v", caCertFile, err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	// Sunucu→ajan mTLS: sunucu, ajana yaptığı isteklerde kendi sertifikasını
	// (server.crt/server.key) istemci sertifikası olarak sunar; ajan bunu CA
	// ile doğrular. Sertifika yüklenemezse token yedeğiyle devam edilir.
	if certFile != "" && keyFile != "" {
		if cert, certErr := tls.LoadX509KeyPair(certFile, keyFile); certErr == nil {
			tlsConfig.Certificates = []tls.Certificate{cert}
		} else {
			log.Printf("UYARI: Sunucu istemci sertifikası yüklenemedi (%s/%s): %v — ajan çağrılarında mTLS devre dışı, token yedeği kullanılacak.", certFile, keyFile, certErr)
		}
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

// maxCommandAttempts, ajan geçici olarak ulaşılamaz olduğunda komutun kaç kez
// denenceğidir. Yalnızca taşıma (transport) hatalarında yeniden denenir —
// yani ajandan hiç yanıt alınamadığında; bu durumda komut kesinlikle
// uygulanmamıştır, dolayısıyla add-key/remove-key gibi işlemleri tekrar etmek
// güvenlidir (çift uygulama riski yok). HTTP yanıtı (200 dışı dahil) alınırsa
// komut ajana ulaşmış demektir; belirsiz durumu tekrarlamamak için denenmez.
const maxCommandAttempts = 3

// commandBackoff, denemeler arası bekleme süresini üstel olarak hesaplar.
func commandBackoff(attempt int) time.Duration {
	return time.Duration(300*(1<<uint(attempt))) * time.Millisecond // 300ms, 600ms, 1.2s...
}

func (c *Client) sendCommand(ip, action string, payload interface{}) error {
	endpoint := fmt.Sprintf("https://%s:%s/actions/%s", ip, c.agentPort, action)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("payload JSON'a çevrilemedi: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < maxCommandAttempts; attempt++ {
		if attempt > 0 {
			wait := commandBackoff(attempt - 1)
			log.Printf("    ↻ Agent'a komut yeniden denenecek (Host: %s, Aksiyon: %s, deneme %d/%d, %s sonra): %v",
				ip, action, attempt+1, maxCommandAttempts, wait, lastErr)
			time.Sleep(wait)
		}

		// İstek gövdesi her denemede yeniden oluşturulur (Body okunup tükenmiş
		// olabilir).
		req, reqErr := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonPayload))
		if reqErr != nil {
			return fmt.Errorf("istek oluşturulamadı: %w", reqErr)
		}
		req.Header.Set("Content-Type", "application/json")
		if c.secretToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.secretToken)
		}

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			// Taşıma hatası: yanıt yok → komut uygulanmadı, yeniden denenebilir.
			lastErr = fmt.Errorf("agent'a istek gönderilemedi: %w", doErr)
			continue
		}

		// Yanıt alındı; komut ajana ulaştı. 200 dışı durumlar tekrarlanmaz.
		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("agent beklenmeyen durum kodu döndü: %s", resp.Status)
			} else {
				lastErr = nil
			}
		}()
		if lastErr != nil {
			return lastErr
		}
		log.Printf("✅ Agent'a komut başarıyla gönderildi: Host: %s, Aksiyon: %s%s", ip, action, attemptSuffix(attempt))
		return nil
	}

	return fmt.Errorf("agent %s %d denemede yanıt vermedi: %w", ip, maxCommandAttempts, lastErr)
}

func attemptSuffix(attempt int) string {
	if attempt == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d. denemede)", attempt+1)
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
