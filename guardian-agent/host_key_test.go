package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

// writeTestHostKey, geçici bir dizine authorized_keys formatında bir ed25519
// public key yazar ve hem dosya yolunu hem de beklenen anahtarı döndürür.
func writeTestHostKey(t *testing.T) (path string, want ssh.PublicKey) {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519 anahtar üretilemedi: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("ssh public key oluşturulamadı: %v", err)
	}
	path = filepath.Join(t.TempDir(), "host_key.pub")
	if err := os.WriteFile(path, ssh.MarshalAuthorizedKey(sshPub), 0644); err != nil {
		t.Fatalf("host key dosyası yazılamadı: %v", err)
	}
	return path, sshPub
}

func TestGetHostKey_Valid(t *testing.T) {
	path, want := writeTestHostKey(t)

	got, err := getHostKey(path)
	if err != nil {
		t.Fatalf("geçerli host key için hata beklenmiyordu: %v", err)
	}
	// Anahtarların wire-format karşılaştırması (aynı public key mi?)
	if string(got.Marshal()) != string(want.Marshal()) {
		t.Error("ayrıştırılan host key, yazılan anahtarla eşleşmiyor")
	}
	if got.Type() != want.Type() {
		t.Errorf("anahtar tipi eşleşmiyor: %q != %q", got.Type(), want.Type())
	}
}

func TestGetHostKey_MissingFile(t *testing.T) {
	_, err := getHostKey(filepath.Join(t.TempDir(), "yok.pub"))
	if err == nil {
		t.Fatal("var olmayan dosya için hata bekleniyordu")
	}
}

func TestGetHostKey_InvalidContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bozuk.pub")
	if err := os.WriteFile(path, []byte("bu bir ssh anahtarı değil\n"), 0644); err != nil {
		t.Fatalf("bozuk dosya yazılamadı: %v", err)
	}
	if _, err := getHostKey(path); err == nil {
		t.Fatal("geçersiz anahtar içeriği için hata bekleniyordu")
	}
}
