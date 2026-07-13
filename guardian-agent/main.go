package main

import (
	"bytes"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type Config struct {
	AgentPort          string
	CertFile           string
	KeyFile            string
	ServerHost         string
	ServerPort         string
	AgentServerID      int
	AgentSshKeyPath    string
	SecretToken        string
	TrustedHostKeyPath string
}

func loadConfig() (*Config, error) {
	// Windows'ta (ve systemd EnvironmentFile kullanılmayan durumlarda) config'i
	// dosyadan env'e yükle; zaten set edilmiş env değerleri korunur.
	loadConfigFileIntoEnv()

	serverIDStr := os.Getenv("GUARDIAN_AGENT_SERVER_ID")
	serverID, _ := strconv.Atoi(serverIDStr)

	cfg := &Config{
		AgentPort:          getEnv("GUARDIAN_AGENT_PORT", "6666"),
		CertFile:           getEnv("AGENT_TLS_CERT_FILE", "../certs/agent.crt"),
		KeyFile:            getEnv("AGENT_TLS_KEY_FILE", "../certs/agent.key"),
		ServerHost:         getEnv("GUARDIAN_SERVER_HOST", "https://localhost"),
		ServerPort:         getEnv("GUARDIAN_SERVER_PORT", "5555"),
		AgentSshKeyPath:    getEnv("GUARDIAN_AGENT_SSH_KEY_PATH", "/etc/guardian/agent_service_key"),
		AgentServerID:      serverID,
		SecretToken:        os.Getenv("GUARDIAN_SECRET_TOKEN"),
		TrustedHostKeyPath: getEnv("GUARDIAN_AGENT_TRUSTED_HOST_KEY", "/etc/ssh/ssh_host_ed25519_key.pub"),
	}

	// Kimlik doğrulama artık öncelikli olarak mTLS (agent.crt/agent.key) ile
	// yapılıyor; GUARDIAN_SECRET_TOKEN yalnızca eski sunucularla uyumluluk için
	// opsiyonel bir yedek. Bu yüzden token zorunlu değil.
	if cfg.SecretToken == "" {
		log.Println("Bilgi: GUARDIAN_SECRET_TOKEN ayarlı değil; kimlik doğrulama mTLS ile yapılacak.")
	}
	return cfg, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Kullanım: guardian-agent [serve|proxy]")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "proxy":
		handleProxy()
	case "serve":
		startHttpServer()
	default:
		fmt.Printf("Bilinmeyen komut: '%s'.\n", os.Args[1])
		os.Exit(1)
	}
}

func startHttpServer() {
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Yapılandırma yüklenemedi: %v", err)
	}

	mux := http.NewServeMux()

	mux.Handle("/actions/validate-user", authMiddleware(http.HandlerFunc(handleValidateUser)))
	mux.Handle("/actions/add-key", authMiddleware(http.HandlerFunc(handleAddKey)))
	mux.Handle("/actions/remove-key", authMiddleware(http.HandlerFunc(handleRemoveKey)))
	// -------------------------

	mux.Handle("/actions/terminate-session", authMiddleware(http.HandlerFunc(handleTerminateSession)))
	mux.HandleFunc("/status", handleStatus)

	// Sunucu→ajan mTLS: sunucu, istemci sertifikası sunduğunda TLS katmanı
	// zinciri CA'ya karşı doğrular. Sağlık kontrolü (/status) sertifikasız da
	// erişilebilir olduğundan VerifyClientCertIfGiven kullanılır; asıl kimlik
	// doğrulama authMiddleware'de yapılır.
	caCertFile := getEnv("TLS_CA_FILE", "../certs/ca.crt")
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if caPEM, caErr := os.ReadFile(caCertFile); caErr == nil {
		pool := x509.NewCertPool()
		if pool.AppendCertsFromPEM(caPEM) {
			tlsConfig.ClientCAs = pool
			tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
		} else {
			log.Printf("UYARI: CA sertifikası ayrıştırılamadı (%s) — sunucu istemci sertifikası doğrulanamayacak (token yedeği geçerli).", caCertFile)
		}
	} else {
		log.Printf("UYARI: CA sertifikası okunamadı (%s): %v — sunucu istemci sertifikası doğrulanamayacak (token yedeği geçerli).", caCertFile, caErr)
	}

	srv := &http.Server{
		Addr:      ":" + config.AgentPort,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	log.Printf("🛡️ Guardian Agent API https://localhost:%s adresinde GÜVENLİ modda başlatılıyor...", config.AgentPort)
	if err := srv.ListenAndServeTLS(config.CertFile, config.KeyFile); err != nil {
		log.Fatalf("Güvenli (TLS) agent sunucusu başlatılamadı: %v", err)
	}
}

func handleProxy() {
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Proxy modu için yapılandırma yüklenemedi: %v", err)
	}
	if config.AgentServerID == 0 {
		log.Fatal("FATAL: GUARDIAN_AGENT_SERVER_ID ortam değişkeni proxy modu için ayarlanmalıdır.")
	}

	width, height := getTerminalSize()
	_, sessionID, validUntil := parseFlagsAndStartSession(config, width, height)
	log.Printf("✅ Sunucuda oturum başarıyla başlatıldı. Session ID: %d", sessionID)

	if err := createPidFile(sessionID); err != nil {
		log.Printf("[FATAL] PID dosyası oluşturulamadı: %v", err)
		endSessionOnServer(sessionID, "error_pid_creation", config)
		return
	}
	defer removePidFile(sessionID)
	defer endSessionOnServer(sessionID, "ended", config)

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			log.Printf("[WARN] Terminal raw mode'a alınamadı (sinyal yönetimi etkilenebilir): %v", err)
		} else {
			defer term.Restore(fd, oldState)
			log.Println("✅ Terminal raw mode'a alındı.")
		}
	}

	ws := connectWebSocket(sessionID, config)
	defer ws.Close()
	log.Printf("✅ Kayıt için WebSocket bağlantısı sunucuya kuruldu.")

	go sendHeartbeats(ws, sessionID)

	client := connectSSH(config)
	defer client.Close()
	log.Println("✅ Arka plan SSH sunucusuna GÜVENLİ bir şekilde bağlanıldı.")

	session, err := client.NewSession()
	if err != nil {
		endSessionOnServer(sessionID, "error_ssh_session", config)
		log.Fatalf("Arka plan SSH oturumu oluşturulamadı: %v", err)
	}
	defer session.Close()

	// Admin "zorla sonlandır" dediğinde bu prosese SIGTERM gönderilir (bkz.
	// terminateProcessBySessionID). SIGKILL yerine burada nazikçe sadece SSH
	// oturumunu kapatıyoruz: uzaktaki shell normal şekilde SIGHUP alıp kendi
	// temizliğini (terminal modlarını sıfırlama escape kodları dahil) yapma
	// şansı buluyor, ayrıca session.Wait() normal döner ve defer'lar
	// (removePidFile, endSessionOnServer) düzgün çalışır. Ham SIGKILL, hem
	// bu defer'ları atlıyordu hem de istemci tarafındaki terminali "uygulama
	// imleç modu" gibi bir ara durumda bırakabiliyordu.
	go func() {
		sigCh := make(chan os.Signal, 1)
		// SIGTERM: admin "zorla sonlandır" dediğinde bize gönderilir.
		// SIGHUP: kullanıcı terminali exit demeden kapatınca (pencereyi çarpı ile
		// kapatma / kısayol) sshd, forced-command olan bu prosese SIGHUP yollar.
		// İkisini de yakalayıp oturumu nazikçe kapatıyoruz ki session.Wait()
		// dönsün ve defer'lar (endSessionOnServer "ended") çalışsın; aksi halde
		// oturum sonsuza dek "active" kalıyordu.
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP)
		sig := <-sigCh
		log.Printf("⚠️ Sonlandırma sinyali (%v) alındı, SSH oturumu nazikçe kapatılıyor...", sig)
		session.Close()
	}()

	// Sunucu tarafındaki scheduler'ın "terminate" komutuna ek olarak, ajan
	// kendi tarafında da süre dolumunu zorlar (ikinci savunma katmanı).
	// valid_until, oturum başlatılırken sunucudan alınır (erişim kuralının
	// valid_until'i); yoksa (süresiz kural) client-side zamanlayıcı kurulmaz.
	if validUntil != nil {
		go enforceSessionTimeout(session, sessionID, *validUntil, config)
	}

	setupPipes(session, ws, width, height)

	if err := session.Shell(); err != nil {
		endSessionOnServer(sessionID, "error_shell", config)
		log.Fatalf("Uzakta shell başlatılamadı: %v", err)
	}
	log.Println("✅ Uzakta shell başarıyla başlatıldı. Oturum aktif.")

	if err := session.Wait(); err != nil {
		log.Printf("Oturum sonlandı. Detay: %v", err)
	} else {
		log.Println("Oturum normal bir şekilde sonlandırıldı.")
	}
}

func parseFlagsAndStartSession(config *Config, width, height int) (int, int, *time.Time) {
	proxyFlags := flag.NewFlagSet("proxy", flag.ExitOnError)
	ruleID := proxyFlags.Int("rule-id", 0, "Oturumla ilişkili erişim kuralı ID'si")
	if err := proxyFlags.Parse(os.Args[2:]); err != nil {
		log.Fatalf("Proxy argümanları parse edilemedi: %v", err)
	}
	if *ruleID == 0 {
		log.Fatal("HATA: --rule-id argümanı olmadan proxy başlatılamaz.")
	}
	log.Printf("🚀 SSH Proxy modu başlatıldı. Kural ID: %d", *ruleID)

	sessionID, validUntil, err := startSessionOnServer(*ruleID, config, width, height)
	if err != nil {
		log.Fatalf("Sunucuda oturum başlatılamadı: %v", err)
	}
	return *ruleID, sessionID, validUntil
}

func connectWebSocket(sessionID int, config *Config) *websocket.Conn {
	wsURL := fmt.Sprintf("%s:%s/api/agent/ws/sessions/%d", strings.Replace(config.ServerHost, "https", "wss", 1), config.ServerPort, sessionID)
	dialer := websocket.Dialer{TLSClientConfig: createApiClient().Transport.(*http.Transport).TLSClientConfig}
	ws, _, err := dialer.Dial(wsURL, http.Header{"Authorization": {"Bearer " + config.SecretToken}})
	if err != nil {
		endSessionOnServer(sessionID, "error_ws_connect", config)
		log.Fatalf("Kayıt için WebSocket bağlantısı kurulamadı: %v", err)
	}
	return ws
}

func connectSSH(config *Config) *ssh.Client {
	log.Printf("[DEBUG] TrustedHostKeyPath: %s", config.TrustedHostKeyPath)
	keyBytes, err := os.ReadFile(config.TrustedHostKeyPath)
	if err != nil {
		log.Fatalf("Agent servis anahtarı okunamadı (%s): %v", config.TrustedHostKeyPath, err)
	}
	log.Printf("[DEBUG] Trusted host key (raw): %s", strings.TrimSpace(string(keyBytes)))
	//
	key, err := os.ReadFile(config.AgentSshKeyPath)
	if err != nil {
		log.Fatalf("Agent servis anahtarı okunamadı (%s): %v", config.AgentSshKeyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("Agent servis anahtarı parse edilemedi: %v", err)
	}

	hostKey, err := getHostKey(config.TrustedHostKeyPath)
	if err != nil {
		log.Fatalf("Güvenilir host anahtarı okunamadı veya parse edilemedi (%s): %v", config.TrustedHostKeyPath, err)
	}

	sshUser := os.Getenv("USER")
	if sshUser == "" {
		log.Fatal("SSH hedef kullanıcısı 'USER' ortam değişkeninden okunamadı.")
	}

	sshConfig := &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.FixedHostKey(hostKey),
	}

	client, err := ssh.Dial("tcp", "localhost:22", sshConfig)
	if err != nil {
		log.Fatalf("Yerel SSH sunucusuna GÜVENLİ bağlantı kurulamadı: %v", err)
	}
	return client
}

func getHostKey(file string) (ssh.PublicKey, error) {
	keyBytes, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("dosya okunamadı: %w", err)
	}
	pk, _, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("anahtar parse edilemedi: %w", err)
	}
	return pk, nil
}

func enforceSessionTimeout(session *ssh.Session, sessionID int, validUntil time.Time, config *Config) {
	durationLeft := time.Until(validUntil)
	if durationLeft <= 0 {
		log.Println("⏰ Oturum süresi zaten dolmuş. Bağlantı hemen kapatılıyor.")
		endSessionOnServer(sessionID, "timed_out", config)
		session.Close()
		return
	}
	log.Printf("⏰ Bu oturumun süresi %s sonra dolacak.", durationLeft.Round(time.Second))
	<-time.After(durationLeft)
	log.Println("⏰ OTURUM SÜRESİ DOLDU! Bağlantı zorla sonlandırılıyor.")
	endSessionOnServer(sessionID, "timed_out", config)
	session.Close()
}

// getTerminalSize, kaydın yapıldığı PTY boyutunu belirler. Bu boyut sunucuya
// bildirilip DB'ye kaydedilir; tekrar oynatma/canlı izleme tarafındaki
// terminal de aynı boyutla oluşturulmalıdır. Aksi halde mutlak imleç
// konumlandırma ve scroll-region ANSI kodları yanlış yorumlanır (ekran bir
// ekrandan fazla dolduğunda imlecin en üste sıçraması bu uyumsuzluktandır).
func getTerminalSize() (width, height int) {
	width, height, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		width, height = 120, 30
		log.Printf("Terminal boyutu alınamadı, varsayılan kullanılıyor: %dx%d", width, height)
	}
	return width, height
}

func setupPipes(session *ssh.Session, ws *websocket.Conn, width, height int) {
	wsOutputWriter := &websocketWriter{conn: ws, eventType: "output"}
	session.Stdout = io.MultiWriter(os.Stdout, wsOutputWriter)
	session.Stderr = io.MultiWriter(os.Stderr, wsOutputWriter)

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		log.Fatalf("Stdin pipe oluşturulamadı: %v", err)
	}
	go func() {
		defer stdinPipe.Close()
		wsInputWriter := &websocketWriter{conn: ws, eventType: "input"}
		io.Copy(io.MultiWriter(stdinPipe, wsInputWriter), os.Stdin)
		// os.Stdin EOF verdi = istemcinin giriş akışı kapandı (kullanıcı bağlantıyı
		// kesti). Uzak interaktif shell kendi PTY'sinde takılı kalıp session.Wait()'i
		// süresiz bloke etmesin diye oturumu burada kapatıyoruz; böylece temizlik
		// defer'ları çalışır ve oturum "active" takılı kalmaz.
		log.Println("İstemci girişi (stdin) kapandı, uzak oturum sonlandırılıyor...")
		session.Close()
	}()

	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	termType := os.Getenv("TERM")
	if termType == "" {
		termType = "xterm-256color"
	}
	if err := session.RequestPty(termType, height, width, modes); err != nil {
		log.Fatalf("Arka plan sunucusundan PTY istenemedi: %v", err)
	}
	log.Println("✅ I/O ve PTY altyapısı kuruldu.")
}

func startSessionOnServer(ruleID int, config *Config, width, height int) (int, *time.Time, error) {
	serverURL := fmt.Sprintf("%s:%s/api/agent/sessions", config.ServerHost, config.ServerPort)
	sshUser := os.Getenv("USER")
	reqBody := map[string]interface{}{
		"rule_id": ruleID, "server_id": config.AgentServerID, "username": sshUser,
		"cols": width, "rows": height,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return 0, nil, fmt.Errorf("istek gövdesi JSON'a çevrilemedi: %w", err)
	}
	req, err := http.NewRequest("POST", serverURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return 0, nil, fmt.Errorf("HTTP isteği oluşturulamadı: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.SecretToken)
	resp, err := createApiClient().Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("sunucuya istek gönderilemedi: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, nil, fmt.Errorf("sunucudan beklenmeyen durum kodu: %s - %s", resp.Status, string(bodyBytes))
	}
	var sessionResponse struct {
		ID         int          `json:"id"`
		ValidUntil sql.NullTime `json:"valid_until"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sessionResponse); err != nil {
		return 0, nil, fmt.Errorf("sunucu cevabı okunamadı: %w", err)
	}
	if sessionResponse.ValidUntil.Valid {
		return sessionResponse.ID, &sessionResponse.ValidUntil.Time, nil
	}
	return sessionResponse.ID, nil, nil
}

func endSessionOnServer(sessionID int, status string, config *Config) {
	if sessionID == 0 {
		return
	}
	serverURL := fmt.Sprintf("%s:%s/api/agent/sessions/%d", config.ServerHost, config.ServerPort, sessionID)
	reqBody := map[string]string{"status": status}
	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("PATCH", serverURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Printf("HATA: Oturum bitiş isteği oluşturulamadı: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.SecretToken)
	resp, err := createApiClient().Do(req)
	if err != nil {
		log.Printf("HATA: Oturum bitiş isteği gönderilemedi: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("HATA: Oturum durumu güncellenemedi, sunucu yanıtı: %s", resp.Status)
	} else {
		log.Printf("✅ Oturum %d durumu sunucuda '%s' olarak güncellendi.", sessionID, status)
	}
}

type KeyPayload struct {
	RuleID       int    `json:"rule_id"`
	Username     string `json:"username"`
	SshPublicKey string `json:"ssh_public_key"`
}

func handleAddKey(w http.ResponseWriter, r *http.Request) {
	var payload KeyPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
		return
	}
	log.Printf("🔑 Anahtar ekleme komutu alındı: Kullanıcı: %s, Kural ID: %d", payload.Username, payload.RuleID)
	if err := addKeyToFile(payload.Username, payload.SshPublicKey, payload.RuleID); err != nil {
		log.Printf("HATA: Anahtar eklenemedi: %v", err)
		http.Error(w, fmt.Sprintf("Anahtar eklenemedi: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Anahtar kullanıcı '%s' için başarıyla eklendi.", payload.Username)
}

func handleRemoveKey(w http.ResponseWriter, r *http.Request) {
	var payload KeyPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
		return
	}
	log.Printf("🔥 Anahtar silme komutu alındı: Kullanıcı: %s", payload.Username)
	if err := removeKeyFromFile(payload.Username, payload.SshPublicKey); err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			log.Printf("UYARI: Silinecek anahtar bulunamadı: %v", err)
			http.Error(w, "Silinecek anahtar bulunamadı", http.StatusNotFound)
			return
		}
		log.Printf("HATA: Anahtar silinemedi: %v", err)
		http.Error(w, fmt.Sprintf("Anahtar silinemedi: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Anahtar kullanıcı '%s' için başarıyla silindi.", payload.Username)
}

func handleTerminateSession(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		SessionID int `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
		return
	}
	if payload.SessionID == 0 {
		http.Error(w, "Eksik session_id", http.StatusBadRequest)
		return
	}
	if err := terminateProcessBySessionID(payload.SessionID); err != nil {
		log.Printf("[ERROR] Oturum sonlandırılamadı: %v", err)
		http.Error(w, fmt.Sprintf("Oturum sonlandırılamadı: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Oturum %d başarıyla sonlandırıldı.", payload.SessionID)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

// authMiddleware, sunucu→ajan isteklerini doğrular. Öncelikli yöntem mTLS'dir:
// sunucu, TLS el sıkışmasında CA tarafından imzalı bir istemci sertifikası
// sunarsa (ajanın TLS katmanı zinciri zaten doğrulamıştır) istek kabul edilir.
// mTLS yoksa, eski sunucularla uyumluluk için paylaşımlı GUARDIAN_SECRET_TOKEN
// yedeğine düşülür.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1) mTLS: geçerli (CA imzalı) istemci sertifikası sunulmuşsa kabul et.
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			next.ServeHTTP(w, r)
			return
		}

		// 2) Yedek: paylaşımlı token. Token yapılandırılmamışsa yalnızca mTLS
		// kabul edildiği için istek reddedilir.
		expectedToken := os.Getenv("GUARDIAN_SECRET_TOKEN")
		if expectedToken == "" {
			http.Error(w, "Kimlik doğrulama başarısız: istemci sertifikası (mTLS) gerekli.", http.StatusUnauthorized)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization başlığı eksik", http.StatusUnauthorized)
			return
		}
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			http.Error(w, "Geçersiz Authorization başlığı formatı", http.StatusUnauthorized)
			return
		}
		if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expectedToken)) != 1 {
			http.Error(w, "Geçersiz token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// createApiClient, sunucuya yapılan çağrılar için HTTP istemcisi üretir. CA'yı
// doğrulamak için RootCAs, ajan→sunucu mTLS için de ajanın kendi sertifikasını
// (agent.crt/agent.key) istemci sertifikası olarak yükler. Sertifika
// yüklenemezse token yedeğiyle devam edilir.
func createApiClient() *http.Client {
	caCert, err := os.ReadFile(getEnv("TLS_CA_FILE", "../certs/ca.crt"))
	if err != nil {
		log.Fatalf("FATAL: CA sertifikası okunamadı: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{RootCAs: caCertPool}

	certFile := getEnv("AGENT_TLS_CERT_FILE", "../certs/agent.crt")
	keyFile := getEnv("AGENT_TLS_KEY_FILE", "../certs/agent.key")
	if cert, certErr := tls.LoadX509KeyPair(certFile, keyFile); certErr == nil {
		tlsConfig.Certificates = []tls.Certificate{cert}
	} else {
		log.Printf("UYARI: Ajan istemci sertifikası yüklenemedi (%s/%s): %v — sunucuya mTLS devre dışı, token yedeği kullanılacak.", certFile, keyFile, certErr)
	}

	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
}

type websocketWriter struct {
	conn      *websocket.Conn
	eventType string
}

func (w *websocketWriter) Write(p []byte) (n int, err error) {
	message := struct {
		Type string `json:"type"`
		Data []byte `json:"data"`
	}{
		Type: w.eventType,
		Data: p,
	}
	if err := w.conn.WriteJSON(message); err != nil {
		return 0, err
	}
	return len(p), nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func handleValidateUser(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
		return
	}

	log.Printf("🔎 Kullanıcı doğrulama isteği alındı: %s", payload.Username)

	_, err := getAuthorizedKeysPath(payload.Username)
	if err != nil {
		log.Printf("   -> KULLANICI BULUNAMADI: %s", payload.Username)
		http.Error(w, fmt.Sprintf("Kullanıcı '%s' bu sistemde bulunamadı.", payload.Username), http.StatusNotFound)
		return
	}

	log.Printf("   -> ✅ Kullanıcı doğrulandı: %s", payload.Username)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Kullanıcı '%s' geçerli.", payload.Username)
}

func sendHeartbeats(conn *websocket.Conn, sessionID int) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	heartbeatMsg := struct {
		Type      string `json:"type"`
		SessionID int    `json:"session_id"`
	}{
		Type:      "heartbeat",
		SessionID: sessionID,
	}

	for {
		select {
		case <-ticker.C:
			if err := conn.WriteJSON(heartbeatMsg); err != nil {
				log.Printf("[WARN] Heartbeat gönderilemedi, muhtemelen bağlantı kapandı: %v", err)
				return
			}
		}
	}
}
