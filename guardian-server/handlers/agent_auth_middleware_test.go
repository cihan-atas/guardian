package handlers

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

// makeCA, test için bir kök CA sertifikası + anahtarı üretir.
func makeCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("CA anahtarı üretilemedi: %v", err)
	}
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
		t.Fatalf("CA sertifikası üretilemedi: %v", err)
	}
	cert, _ := x509.ParseCertificate(der)
	return cert, key
}

// makeLeaf, verilen CA tarafından imzalanmış bir yaprak (istemci/sunucu)
// sertifikası üretir.
func makeLeaf(t *testing.T, cn string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("yaprak anahtarı üretilemedi: %v", err)
	}
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
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("yaprak sertifikası üretilemedi: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: nil}
}

// newAgentAuthServer, AgentAuth ile sarılmış bir handler'ı, ajan→sunucu
// mTLS'i taklit eden bir TLS test sunucusu olarak başlatır.
func newAgentAuthServer(t *testing.T, caCert *x509.Certificate, serverCert tls.Certificate) *httptest.Server {
	t.Helper()
	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	h := AgentAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func clientWith(caCert *x509.Certificate, clientCert *tls.Certificate) *http.Client {
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	cfg := &tls.Config{RootCAs: pool}
	if clientCert != nil {
		cfg.Certificates = []tls.Certificate{*clientCert}
	}
	return &http.Client{Transport: &http.Transport{TLSClientConfig: cfg}}
}

func TestAgentAuth_MTLSAccepted(t *testing.T) {
	t.Setenv("GUARDIAN_SECRET_TOKEN", "") // token yedeği kapalı: yalnızca mTLS
	caCert, caKey := makeCA(t)
	serverCert := makeLeaf(t, "server", caCert, caKey)
	agentCert := makeLeaf(t, "agent-1", caCert, caKey)

	srv := newAgentAuthServer(t, caCert, serverCert)
	client := clientWith(caCert, &agentCert)

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("istek başarısız: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mTLS istemci sertifikasıyla 200 bekleniyordu, alınan: %d", resp.StatusCode)
	}
}

func TestAgentAuth_NoCertNoTokenRejected(t *testing.T) {
	t.Setenv("GUARDIAN_SECRET_TOKEN", "") // token yedeği kapalı
	caCert, caKey := makeCA(t)
	serverCert := makeLeaf(t, "server", caCert, caKey)

	srv := newAgentAuthServer(t, caCert, serverCert)
	client := clientWith(caCert, nil) // istemci sertifikası yok

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("istek başarısız: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("sertifikasız+token'sız istekte 401 bekleniyordu, alınan: %d", resp.StatusCode)
	}
}

func TestAgentAuth_TokenFallback(t *testing.T) {
	t.Setenv("GUARDIAN_SECRET_TOKEN", "gizli-yedek-token")
	caCert, caKey := makeCA(t)
	serverCert := makeLeaf(t, "server", caCert, caKey)

	srv := newAgentAuthServer(t, caCert, serverCert)
	client := clientWith(caCert, nil) // sertifika yok → token yedeğine düşer

	req, _ := http.NewRequest("GET", srv.URL, nil)
	req.Header.Set("Authorization", "Bearer gizli-yedek-token")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("istek başarısız: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("geçerli token yedeğiyle 200 bekleniyordu, alınan: %d", resp.StatusCode)
	}

	// Yanlış token reddedilmeli.
	req2, _ := http.NewRequest("GET", srv.URL, nil)
	req2.Header.Set("Authorization", "Bearer yanlis-token")
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("istek başarısız: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("yanlış token'da 403 bekleniyordu, alınan: %d", resp2.StatusCode)
	}
}
