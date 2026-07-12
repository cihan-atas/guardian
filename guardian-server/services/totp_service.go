// guardian/guardian-server/services/totp_service.go
//
// RFC 6238 TOTP (zaman tabanlı tek kullanımlık parola) uygulaması — yalnızca
// standart kütüphane. Yönetici hesapları için opsiyonel iki adımlı doğrulama
// (2FA) burada üretilir/doğrulanır. Google Authenticator, Authy vb. uyumlu
// (SHA1, 6 hane, 30 sn adım).

package services

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	totpDigits = 6
	totpPeriod = 30 * time.Second
	// totpSkew, saat kayması için kabul edilen komşu pencere sayısı (±1 => ~90 sn).
	totpSkew = 1
)

// GenerateTOTPSecret, base32 kodlanmış (padding'siz) rastgele bir gizli anahtar
// üretir (20 bayt = 160 bit, RFC 4226 önerisi).
func GenerateTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// totpAt, belirli bir zaman için TOTP kodunu (sıfır dolgulu 6 hane) üretir.
func totpAt(secret string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).
		DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", fmt.Errorf("geçersiz TOTP anahtarı: %w", err)
	}
	counter := uint64(t.Unix()) / uint64(totpPeriod.Seconds())

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	// RFC 4226 dinamik kırpma.
	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset])&0x7f)<<24 |
		(uint32(sum[offset+1])&0xff)<<16 |
		(uint32(sum[offset+2])&0xff)<<8 |
		(uint32(sum[offset+3]) & 0xff)
	code = code % 1_000_000
	return fmt.Sprintf("%06d", code), nil
}

// ValidateTOTP, verilen kodu geçerli pencerede (±totpSkew) doğrular. Sabit
// zamanlı karşılaştırma kullanır.
func ValidateTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false
	}
	now := time.Now()
	for i := -totpSkew; i <= totpSkew; i++ {
		want, err := totpAt(secret, now.Add(time.Duration(i)*totpPeriod))
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// TOTPProvisioningURI, authenticator uygulamalarının (QR ile) tanıdığı
// otpauth:// URI'sini üretir.
func TOTPProvisioningURI(secret, account, issuer string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprintf("%d", totpDigits))
	q.Set("period", fmt.Sprintf("%d", int(totpPeriod.Seconds())))
	return fmt.Sprintf("otpauth://totp/%s?%s", label, q.Encode())
}
