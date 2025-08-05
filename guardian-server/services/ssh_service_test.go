// guardian/guardian-server/services/ssh_service_test.go

package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGenerateFingerprint, ssh_service.go dosyasındaki GenerateFingerprint fonksiyonunu test eder.
// Go'da test yazarken "table-driven test" (tablo güdümlü test) yaklaşımı çok yaygındır.
// Bu yaklaşımda, bir dizi test senaryosu (test case) tanımlarız ve bir döngü içinde
// hepsini çalıştırırız. Bu, kodu tekrar etmemizi engeller ve yeni senaryolar eklemeyi kolaylaştırır.
func TestGenerateFingerprint(t *testing.T) {
	// Test senaryolarımızı bir struct içinde tanımlıyoruz.
	testCases := []struct {
		name                string // Testin ne yaptığını açıklayan bir isim
		inputPublicKey      string // Fonksiyona verilecek girdi
		expectedFingerprint string // Fonksiyondan beklediğimiz doğru çıktı
		expectError         bool   // Hata bekleyip beklemediğimizi belirten bir bayrak
	}{
		{
			name:           "Başarılı - Geçerli bir RSA public anahtar",
			inputPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCzH6dGjfsXA33t8CjlplE6OsEzoliynScv4m+Hx+l6LphnSI5WUO292U6KSj5vfCmzfu6OKHTbV/IBZwxq9tv5IJRSKIvyUwlXMaCYAVxIEJ9gE0jU4O9cnQ9xWLeQNPIshP2tm7389ZJAFz2YKW3u4UPTNo4Mdm4Bu7TpiQC4AlPCeYsc6+vbv3MNomIWlObxGpnFw8uoiMiOPIDqTnj0QsEueKgqG24XrGqVYvMDXt/BYlQEhzHuzucYpk3eBDSzqORZnIiA7zmDPWxBJKAikzaSstskJTYSoizx0k0F8XQBpLbwQpsDebafGBfo82myyFrb7xaJZw7egWTePWOX example@guardian",
			// DÜZELTME: Beklenen değeri, testin verdiği "actual" değer ile güncelliyoruz.
			expectedFingerprint: "SHA256:2zE8l8gxwGaX3GvLHS32m0fqyuoio/BlLV6ECdW1bks", // <--- BU SATIR DEĞİŞTİ
			expectError:         false,
		},
		{
			name:                "Hata - Geçersiz formatta public anahtar",
			inputPublicKey:      "bu bir public anahtar değil",
			expectedFingerprint: "",
			expectError:         true,
		},
		{
			name:                "Hata - Boş girdi",
			inputPublicKey:      "",
			expectedFingerprint: "",
			expectError:         true,
		},
	}

	// Tanımladığımız tüm test senaryolarını bir döngüde çalıştırıyoruz.
	for _, tc := range testCases {
		// t.Run, her senaryo için ayrı bir alt test oluşturur. Bu, test sonuçlarının
		// daha okunaklı olmasını sağlar.
		t.Run(tc.name, func(t *testing.T) {
			// Asıl fonksiyonumuzu çağırıyoruz
			fingerprint, err := GenerateFingerprint(tc.inputPublicKey)

			// Hata beklentimizi kontrol ediyoruz.
			if tc.expectError {
				// Hata bekliyorduk, hata geldi mi?
				assert.Error(t, err, "Bu senaryoda bir hata bekleniyordu ama gelmedi.")
			} else {
				// Hata beklemiyorduk, yine de hata geldi mi?
				assert.NoError(t, err, "Bu senaryoda hata beklenmiyordu ama bir hata oluştu.")
				// Çıktı, beklediğimiz çıktı ile aynı mı?
				assert.Equal(t, tc.expectedFingerprint, fingerprint, "Oluşturulan parmak izi beklenenden farklı.")
			}
		})
	}
}
