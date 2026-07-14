package main

import (
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsWindowsAdminUser(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		username string
		want     bool
	}{
		{"boş liste", "", "administrator", false},
		{"tam eşleşme", "administrator,root", "administrator", true},
		{"büyük/küçük harf duyarsız", "Administrator", "administrator", true},
		{"boşluklu giriş kırpılır", " admin , svc ", "admin", true},
		{"listede yok", "admin,svc", "guest", false},
		{"ikinci öğe eşleşir", "admin,svc", "svc", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GUARDIAN_WINDOWS_ADMIN_USERS", tt.envValue)
			if got := isWindowsAdminUser(tt.username); got != tt.want {
				t.Errorf("isWindowsAdminUser(%q) env=%q = %v; beklenen %v",
					tt.username, tt.envValue, got, tt.want)
			}
		})
	}
}

func TestDefaultAgentConfigPath(t *testing.T) {
	got := defaultAgentConfigPath()
	if runtime.GOOS == "windows" {
		// Windows'ta ProgramData altında guardian\agent.conf beklenir.
		if !strings.HasSuffix(filepath.ToSlash(got), "guardian/agent.conf") {
			t.Errorf("Windows yolu 'guardian/agent.conf' ile bitmeliydi, alınan: %q", got)
		}
	} else {
		if got != "/etc/guardian/agent.conf" {
			t.Errorf("Unix yolu /etc/guardian/agent.conf olmalıydı, alınan: %q", got)
		}
	}
}

// TestGetAuthorizedKeysPath, gerçek SSH/sunucu gerektirmez; yalnızca yerel
// kullanıcı veritabanı (user.Lookup) üzerinden yol üretimini doğrular.
func TestGetAuthorizedKeysPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bu test Unix ev dizini varsayımına dayanır")
	}
	cur, err := user.Current()
	if err != nil {
		t.Skipf("mevcut kullanıcı okunamadı: %v", err)
	}

	got, err := getAuthorizedKeysPath(cur.Username)
	if err != nil {
		t.Fatalf("mevcut kullanıcı için hata beklenmiyordu: %v", err)
	}
	want := filepath.Join(cur.HomeDir, ".ssh", "authorized_keys")
	if got != want {
		t.Errorf("yol beklenenle eşleşmiyor: %q != %q", got, want)
	}
}

func TestGetAuthorizedKeysPath_UnknownUser(t *testing.T) {
	// Neredeyse kesinlikle var olmayan bir kullanıcı adı.
	if _, err := getAuthorizedKeysPath("guardian_yok_kullanici_zzz"); err == nil {
		t.Fatal("var olmayan kullanıcı için hata bekleniyordu")
	}
}

// TestWindowsAdminAuthorizedKeysPath, Windows yönetici anahtar yolu üreten saf
// yardımcıyı her platformda doğrular (yol ayracı GOOS'a göre değiştiğinden
// karşılaştırma da filepath.Join ile yapılır).
func TestWindowsAdminAuthorizedKeysPath(t *testing.T) {
	// ProgramData verildiğinde onu kullanır.
	if got, want := windowsAdminAuthorizedKeysPath(`D:\PD`), filepath.Join(`D:\PD`, "ssh", "administrators_authorized_keys"); got != want {
		t.Errorf("verilen ProgramData kullanılmalıydı: %q != %q", got, want)
	}
	// Boş ProgramData → varsayılan C:\ProgramData.
	got := windowsAdminAuthorizedKeysPath("")
	want := filepath.Join(`C:\ProgramData`, "ssh", "administrators_authorized_keys")
	if got != want {
		t.Errorf("boş ProgramData'da varsayılan beklenirdi: %q != %q", got, want)
	}
	if !strings.HasSuffix(filepath.ToSlash(got), "ssh/administrators_authorized_keys") {
		t.Errorf("yol administrators_authorized_keys ile bitmeliydi: %q", got)
	}
}
