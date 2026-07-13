package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFileIntoEnv(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "agent.conf")
	content := `# yorum satırı, atlanmalı

GUARDIAN_SERVER_HOST=https://ornek.local
GUARDIAN_SERVER_PORT="5555"
  GUARDIAN_AGENT_PORT = 6666
BOZUK_SATIR_ESITSIZ
`
	if err := os.WriteFile(confPath, []byte(content), 0600); err != nil {
		t.Fatalf("conf yazılamadı: %v", err)
	}

	t.Setenv("GUARDIAN_AGENT_CONFIG", confPath)
	// Önceden set edilmiş bir değer korunmalı (dosya üzerine yazmamalı).
	t.Setenv("GUARDIAN_SERVER_HOST", "https://onceden-ayarli.local")
	// Bunlar dosyadan gelecek (henüz set değil).
	os.Unsetenv("GUARDIAN_SERVER_PORT")
	os.Unsetenv("GUARDIAN_AGENT_PORT")

	loadConfigFileIntoEnv()

	if got := os.Getenv("GUARDIAN_SERVER_HOST"); got != "https://onceden-ayarli.local" {
		t.Errorf("mevcut env değeri korunmalıydı, alınan: %q", got)
	}
	if got := os.Getenv("GUARDIAN_SERVER_PORT"); got != "5555" {
		t.Errorf("tırnaklar temizlenip 5555 gelmeliydi, alınan: %q", got)
	}
	if got := os.Getenv("GUARDIAN_AGENT_PORT"); got != "6666" {
		t.Errorf("boşluklar kırpılıp 6666 gelmeliydi, alınan: %q", got)
	}
	if _, ok := os.LookupEnv("BOZUK_SATIR_ESITSIZ"); ok {
		t.Error("eşittir içermeyen satır ayrıştırılmamalıydı")
	}
}

func TestLoadConfigFileIntoEnv_MissingFileIsNoOp(t *testing.T) {
	t.Setenv("GUARDIAN_AGENT_CONFIG", filepath.Join(t.TempDir(), "yok.conf"))
	// Panik/side-effect olmadan sessizce dönmeli.
	loadConfigFileIntoEnv()
}

func TestGetEnv_Fallback(t *testing.T) {
	os.Unsetenv("GUARDIAN_TEST_YOK")
	if got := getEnv("GUARDIAN_TEST_YOK", "varsayilan"); got != "varsayilan" {
		t.Errorf("yoksa fallback dönmeli, alınan: %q", got)
	}
	t.Setenv("GUARDIAN_TEST_VAR", "deger")
	if got := getEnv("GUARDIAN_TEST_VAR", "varsayilan"); got != "deger" {
		t.Errorf("varsa gerçek değer dönmeli, alınan: %q", got)
	}
}
