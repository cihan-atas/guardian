package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAddKeyToSpecificFile_LineFormat, üretilen authorized_keys satırının
// güvenlik açısından kritik biçim kurallarını doğrular:
//   - "restrict" kısıtlamasından SONRA "pty" gelmeli (PTY geri açılıyor),
//   - forced-command ilgili rule-id ile yazılmalı,
//   - GUARDIAN_SECRET_TOKEN kesinlikle dosyaya YAZILMAMALI.
func TestAddKeyToSpecificFile_LineFormat(t *testing.T) {
	authFile := filepath.Join(t.TempDir(), "authorized_keys")
	pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1 kullanici@ornek"

	// Sızmaması gereken gizli token ayarlı olsa bile dosyaya girmemeli.
	t.Setenv("GUARDIAN_SECRET_TOKEN", "COK-GIZLI-TOKEN")
	t.Setenv("GUARDIAN_AGENT_SERVER_ID", "7")

	if err := addKeyToSpecificFile(authFile, pubKey, 42); err != nil {
		t.Fatalf("anahtar eklenemedi: %v", err)
	}

	content, err := os.ReadFile(authFile)
	if err != nil {
		t.Fatalf("dosya okunamadı: %v", err)
	}
	line := string(content)

	if strings.Contains(line, "COK-GIZLI-TOKEN") {
		t.Error("GUARDIAN_SECRET_TOKEN authorized_keys'e yazılmamalıydı")
	}
	if !strings.Contains(line, "restrict,pty") {
		t.Error("satır 'restrict,pty' (kısıtlamadan sonra pty) içermeliydi")
	}
	if !strings.Contains(line, "--rule-id=42") {
		t.Error("forced-command rule-id=42 içermeliydi")
	}
	// pty, mutlaka restrict'ten sonra gelmeli (soldan sağa işlenir).
	if strings.Index(line, "restrict") > strings.Index(line, ",pty") {
		t.Error("'pty', 'restrict'ten sonra gelmeliydi")
	}
	// Public key satırın sonunda yer almalı.
	if !strings.Contains(line, pubKey) {
		t.Error("public key satırda bulunmalıydı")
	}
}
