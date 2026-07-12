package services

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)

// CertInfo, bir sertifikanın özet bilgisi (UI süre-sonu göstergesi için).
type CertInfo struct {
	Subject   string    `json:"subject"`
	Issuer    string    `json:"issuer"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
	DaysLeft  int       `json:"days_left"`
}

// CertInfoFromX509, bir sertifikadan CertInfo üretir (kalan gün dahil).
func CertInfoFromX509(c *x509.Certificate) *CertInfo {
	daysLeft := int(time.Until(c.NotAfter).Hours() / 24)
	return &CertInfo{
		Subject:   c.Subject.CommonName,
		Issuer:    c.Issuer.CommonName,
		NotBefore: c.NotBefore,
		NotAfter:  c.NotAfter,
		DaysLeft:  daysLeft,
	}
}

// ReadCertInfo, PEM sertifika dosyasını okuyup CertInfo döndürür.
func ReadCertInfo(path string) (*CertInfo, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("sertifika okunamadı (%s): %w", path, err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("sertifika PEM çözülemedi (%s)", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("sertifika parse edilemedi (%s): %w", path, err)
	}
	return CertInfoFromX509(cert), nil
}

// CA, agent sertifikalarını imzalamak için CA cert+anahtarını tutar.
// ca.key sunucu host'unda (yalnızca root okuyabilecek şekilde) bulunmalıdır;
// yoksa enrollment devre dışı kalır.
type CA struct {
	cert    *x509.Certificate
	key     crypto.Signer
	certPEM []byte
}

// LoadCA, verilen yollardan CA sertifikası ve özel anahtarını yükler.
// Anahtar PKCS8, PKCS1 (RSA) veya EC formatında olabilir.
func LoadCA(certPath, keyPath string) (*CA, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("CA sertifikası okunamadı (%s): %w", certPath, err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("CA anahtarı okunamadı (%s): %w", keyPath, err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("CA sertifikası PEM çözülemedi")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("CA sertifikası parse edilemedi: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("CA anahtarı PEM çözülemedi")
	}
	signer, err := parsePrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("CA anahtarı parse edilemedi: %w", err)
	}

	return &CA{cert: cert, key: signer, certPEM: certPEM}, nil
}

func parsePrivateKey(der []byte) (crypto.Signer, error) {
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if s, ok := k.(crypto.Signer); ok {
			return s, nil
		}
		return nil, fmt.Errorf("PKCS8 anahtarı imzalayıcı değil")
	}
	if k, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return k, nil
	}
	if k, err := x509.ParseECPrivateKey(der); err == nil {
		return k, nil
	}
	return nil, fmt.Errorf("desteklenmeyen özel anahtar formatı")
}

// CACertPEM, CA sertifikasını PEM olarak döndürür (agent'a dağıtılır).
func (c *CA) CACertPEM() []byte {
	return c.certPEM
}

// SignAgentCSR, agent'ın ürettiği CSR'ı CA ile imzalar ve verilen IP/DNS
// SAN'larıyla bir agent sertifikası (PEM) döndürür.
func (c *CA) SignAgentCSR(csrPEM []byte, ips []net.IP, dnsNames []string) ([]byte, error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil {
		return nil, fmt.Errorf("CSR PEM çözülemedi")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("CSR parse edilemedi: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("CSR imzası geçersiz: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("seri numarası üretilemedi: %w", err)
	}

	// 127.0.0.1 her zaman eklenir (agent yerel sağlık/kendine bağlanma için).
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

	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               csr.Subject,
		NotBefore:             time.Now().Add(-5 * time.Minute),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		IPAddresses:           ips,
		DNSNames:              dnsNames,
	}
	if len(tmpl.Subject.CommonName) == 0 && len(dnsNames) > 0 {
		tmpl.Subject = pkix.Name{CommonName: dnsNames[0]}
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.cert, csr.PublicKey, c.key)
	if err != nil {
		return nil, fmt.Errorf("sertifika imzalanamadı: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}
