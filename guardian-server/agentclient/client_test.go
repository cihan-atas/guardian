package agentclient

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"guardian.com/server/models"
)

// newTestClient, verilen httptest sunucusuna bağlanan bir Client kurar.
// New()'in CA dosyası gereksinimini atlamak için alanları doğrudan doldururuz
// (test aynı pakette olduğundan mümkün).
func newTestClient(t *testing.T, srv *httptest.Server) (*Client, string) {
	t.Helper()
	// srv.URL: https://127.0.0.1:PORT → host ve port'u ayır.
	trimmed := strings.TrimPrefix(srv.URL, "https://")
	host, port, ok := strings.Cut(trimmed, ":")
	if !ok {
		t.Fatalf("sunucu URL ayrıştırılamadı: %s", srv.URL)
	}
	return &Client{
		httpClient: srv.Client(), // sunucunun sertifikasına güvenen istemci
		agentPort:  port,
	}, host
}

// TestSendCommand_SuccessNoRetry: 200 dönen ajan ilk denemede başarılı olmalı.
func TestSendCommand_SuccessNoRetry(t *testing.T) {
	var hits int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, host := newTestClient(t, srv)
	if err := c.SendKeyCommand(host, "add", models.KeyPayload{Username: "u", RuleID: 1}); err != nil {
		t.Fatalf("başarılı komut beklenmiyordu hata: %v", err)
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("1 istek bekleniyordu, alınan: %d", n)
	}
}

// TestSendCommand_HTTPErrorNoRetry: 200 dışı yanıt (ajana ulaştı) tekrar
// denenmemeli — belirsiz durumu çift uygulama riski yaratmamak için.
func TestSendCommand_HTTPErrorNoRetry(t *testing.T) {
	var hits int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, host := newTestClient(t, srv)
	err := c.SendKeyCommand(host, "add", models.KeyPayload{Username: "u", RuleID: 1})
	if err == nil {
		t.Fatal("500 yanıtında hata bekleniyordu")
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("HTTP yanıtı alındığında tekrar denenmemeli; 1 istek bekleniyordu, alınan: %d", n)
	}
}

// TestSendCommand_TransportRetry: ajan ulaşılamazsa (taşıma hatası) komut
// maxCommandAttempts kez denenmeli ve sonunda hata dönmeli.
func TestSendCommand_TransportRetry(t *testing.T) {
	// Kapalı bir porta bağlan: her denemede taşıma hatası oluşur.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	c, host := newTestClient(t, srv)
	srv.Close() // sunucuyu kapat → bağlantı reddedilir

	start := time.Now()
	err := c.SendKeyCommand(host, "add", models.KeyPayload{Username: "u", RuleID: 1})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("ulaşılamayan ajan için hata bekleniyordu")
	}
	if !strings.Contains(err.Error(), "denemede") {
		t.Fatalf("hata mesajı yeniden deneme sayısını içermeli, alınan: %v", err)
	}
	// En az iki backoff beklemesi olmalı: 300ms + 600ms = 900ms.
	if elapsed < 900*time.Millisecond {
		t.Fatalf("üstel backoff ile en az ~900ms bekleniyordu, geçen: %s", elapsed)
	}
}
