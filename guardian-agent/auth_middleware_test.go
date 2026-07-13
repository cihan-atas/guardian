package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CA üretilemedi: %v", err)
	}
	cert, _ := x509.ParseCertificate(der)
	return cert, key
}

func testLeaf(t *testing.T, cn string, ca *x509.Certificate, caKey *ecdsa.PrivateKey) tls.Certificate {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("yaprak üretilemedi: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// startAgentAuthServer, authMiddleware'i sunucu→ajan mTLS'i taklit eden bir
// TLS test sunucusu olarak başlatır.
func startAgentAuthServer(t *testing.T, ca *x509.Certificate, serverCert tls.Certificate) *httptest.Server {
	t.Helper()
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	h := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv := httptest.NewUnstartedServer(h)
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.VerifyClientCertIfGiven,
		ClientCAs:    pool,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func agentTestClient(ca *x509.Certificate, clientCert *tls.Certificate) *http.Client {
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	cfg := &tls.Config{RootCAs: pool}
	if clientCert != nil {
		cfg.Certificates = []tls.Certificate{*clientCert}
	}
	return &http.Client{Transport: &http.Transport{TLSClientConfig: cfg}}
}

func TestAgentAuthMiddleware_MTLSAccepted(t *testing.T) {
	t.Setenv("GUARDIAN_SECRET_TOKEN", "")
	ca, caKey := testCA(t)
	agentSrvCert := testLeaf(t, "agent", ca, caKey)
	serverClientCert := testLeaf(t, "guardian-server", ca, caKey)

	srv := startAgentAuthServer(t, ca, agentSrvCert)
	client := agentTestClient(ca, &serverClientCert)

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("istek başarısız: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mTLS ile 200 bekleniyordu, alınan: %d", resp.StatusCode)
	}
}

func TestAgentAuthMiddleware_NoCertNoTokenRejected(t *testing.T) {
	t.Setenv("GUARDIAN_SECRET_TOKEN", "")
	ca, caKey := testCA(t)
	agentSrvCert := testLeaf(t, "agent", ca, caKey)

	srv := startAgentAuthServer(t, ca, agentSrvCert)
	client := agentTestClient(ca, nil)

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("istek başarısız: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("sertifikasız+token'sız 401 bekleniyordu, alınan: %d", resp.StatusCode)
	}
}

func TestAgentAuthMiddleware_TokenFallback(t *testing.T) {
	t.Setenv("GUARDIAN_SECRET_TOKEN", "yedek-token")
	ca, caKey := testCA(t)
	agentSrvCert := testLeaf(t, "agent", ca, caKey)

	srv := startAgentAuthServer(t, ca, agentSrvCert)
	client := agentTestClient(ca, nil)

	// Doğru token → 200.
	req, _ := http.NewRequest("GET", srv.URL, nil)
	req.Header.Set("Authorization", "Bearer yedek-token")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("istek başarısız: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("doğru token'da 200 bekleniyordu, alınan: %d", resp.StatusCode)
	}

	// Yanlış token → 403.
	req2, _ := http.NewRequest("GET", srv.URL, nil)
	req2.Header.Set("Authorization", "Bearer yanlis")
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("istek başarısız: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("yanlış token'da 403 bekleniyordu, alınan: %d", resp2.StatusCode)
	}
}
