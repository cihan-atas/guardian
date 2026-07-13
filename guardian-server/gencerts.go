package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// genCertsValidityDays, üretilen CA ve sunucu sertifikalarının geçerlilik süresi
// (generate-certs.sh ile aynı: 10 yıl).
const genCertsValidityDays = 3650

// runGenCerts, openssl'e gerek kalmadan crypto/x509 ile bir CA (kendinden
// imzalı) + sunucu sertifikası (SAN'lı) + anahtarlarını üretir. generate-certs.sh
// betiğinin ürettiği yapıyı (ca.crt/ca.key + server.crt/server.key) taklit eder.
//
// Yollar env'den okunur (TLS_CA_FILE, TLS_CA_KEY_FILE, TLS_CERT_FILE,
// TLS_KEY_FILE); yoksa ../certs/* varsayılanları kullanılır. SAN için
// GUARDIAN_CERT_HOSTS (virgüllü) desteklenir; yoksa localhost + 127.0.0.1.
// --force verilmedikçe mevcut dosyaların üzerine yazılmaz.
func runGenCerts(args []string) {
	force := false
	for _, a := range args {
		if a == "--force" || a == "-force" {
			force = true
		}
	}

	caCertFile := getEnv("TLS_CA_FILE", "../certs/ca.crt")
	caKeyFile := getEnv("TLS_CA_KEY_FILE", "../certs/ca.key")
	certFile := getEnv("TLS_CERT_FILE", "../certs/server.crt")
	keyFile := getEnv("TLS_KEY_FILE", "../certs/server.key")

	// --force yoksa mevcut dosyalar korunur.
	if !force {
		if existing := existingFiles(caCertFile, caKeyFile, certFile, keyFile); len(existing) > 0 {
			log.Printf("UYARI: Aşağıdaki dosyalar zaten mevcut; üzerine yazılmadı. Yeniden üretmek için --force kullanın:")
			for _, p := range existing {
				log.Printf("  - %s", p)
			}
			os.Exit(1)
		}
	}

	// Hedef dizinleri oluştur.
	for _, p := range []string{caCertFile, caKeyFile, certFile, keyFile} {
		if dir := filepath.Dir(p); dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				log.Fatalf("dizin oluşturulamadı (%s): %v", dir, err)
			}
		}
	}

	// SAN'ları çözümle (IP ve DNS ayrı listelere).
	ips, dnsNames := parseCertHosts(os.Getenv("GUARDIAN_CERT_HOSTS"))

	// --- Adım 1: CA (kendinden imzalı) ---
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("CA anahtarı üretilemedi: %v", err)
	}
	caSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		log.Fatalf("CA seri numarası üretilemedi: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber: caSerial,
		Subject: pkix.Name{
			Country:      []string{"TR"},
			Organization: []string{"Guardian Self-Signed"},
			CommonName:   "Guardian Root CA",
		},
		NotBefore:             time.Now().Add(-5 * time.Minute),
		NotAfter:              time.Now().AddDate(0, 0, genCertsValidityDays),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		log.Fatalf("CA sertifikası imzalanamadı: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		log.Fatalf("CA sertifikası parse edilemedi: %v", err)
	}

	// --- Adım 2: Sunucu sertifikası (CA ile imzalı, SAN'lı) ---
	srvKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("sunucu anahtarı üretilemedi: %v", err)
	}
	srvSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		log.Fatalf("sunucu seri numarası üretilemedi: %v", err)
	}
	srvCN := "localhost"
	if len(dnsNames) > 0 {
		srvCN = dnsNames[0]
	}
	srvTmpl := &x509.Certificate{
		SerialNumber: srvSerial,
		Subject: pkix.Name{
			Country:      []string{"TR"},
			Organization: []string{"Guardian Self-Signed"},
			CommonName:   srvCN,
		},
		NotBefore:             time.Now().Add(-5 * time.Minute),
		NotAfter:              time.Now().AddDate(0, 0, genCertsValidityDays),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		IPAddresses:           ips,
		DNSNames:              dnsNames,
	}
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caCert, &srvKey.PublicKey, caKey)
	if err != nil {
		log.Fatalf("sunucu sertifikası imzalanamadı: %v", err)
	}

	// --- Dosyaları yaz (anahtarlar 0600, sertifikalar 0644) ---
	writePEM(caKeyFile, "PRIVATE KEY", mustMarshalKey(caKey), 0o600)
	writePEM(caCertFile, "CERTIFICATE", caDER, 0o644)
	writePEM(keyFile, "PRIVATE KEY", mustMarshalKey(srvKey), 0o600)
	writePEM(certFile, "CERTIFICATE", srvDER, 0o644)

	fmt.Printf("✅ Sertifikalar üretildi:\n  CA cert : %s\n  CA key  : %s\n  Cert    : %s\n  Key     : %s\n",
		caCertFile, caKeyFile, certFile, keyFile)
	fmt.Printf("SAN → DNS: %v, IP: %v\n", dnsNames, ipsToStrings(ips))
}

// existingFiles, verilen yollardan mevcut olanları döndürür.
func existingFiles(paths ...string) []string {
	var out []string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// parseCertHosts, virgülle ayrılmış host listesini IP ve DNS SAN'larına ayırır.
// Boş verilirse localhost + 127.0.0.1 kullanılır. 127.0.0.1 her zaman eklenir.
func parseCertHosts(raw string) ([]net.IP, []string) {
	var ips []net.IP
	var dnsNames []string

	parts := strings.Split(raw, ",")
	for _, part := range parts {
		h := strings.TrimSpace(part)
		if h == "" {
			continue
		}
		if ip := net.ParseIP(h); ip != nil {
			ips = append(ips, ip)
		} else {
			dnsNames = append(dnsNames, h)
		}
	}

	if len(ips) == 0 && len(dnsNames) == 0 {
		dnsNames = append(dnsNames, "localhost")
		ips = append(ips, net.IPv4(127, 0, 0, 1))
		return ips, dnsNames
	}

	// 127.0.0.1 her zaman SAN'da bulunsun.
	hasLoopback := false
	for _, ip := range ips {
		if ip.Equal(net.IPv4(127, 0, 0, 1)) {
			hasLoopback = true
			break
		}
	}
	if !hasLoopback {
		ips = append(ips, net.IPv4(127, 0, 0, 1))
	}
	return ips, dnsNames
}

// mustMarshalKey, RSA anahtarını PKCS8 DER'e kodlar; hata olursa süreci sonlandırır.
func mustMarshalKey(key *rsa.PrivateKey) []byte {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		log.Fatalf("anahtar kodlanamadı: %v", err)
	}
	return der
}

// writePEM, verilen DER bloğunu PEM olarak dosyaya yazar.
func writePEM(path, blockType string, der []byte, perm os.FileMode) {
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, pemBytes, perm); err != nil {
		log.Fatalf("dosya yazılamadı (%s): %v", path, err)
	}
}

// ipsToStrings, IP listesini yazdırılabilir string'lere çevirir.
func ipsToStrings(ips []net.IP) []string {
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ip.String())
	}
	return out
}
