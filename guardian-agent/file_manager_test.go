// guardian/guardian-agent/file_manager_test.go

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddAndRemoveKey(t *testing.T) {
	// 1. Her test için izole, geçici bir dizin oluştur.
	// Test bittiğinde bu dizin otomatik olarak silinecektir.
	tempDir := t.TempDir()
	authKeysFile := filepath.Join(tempDir, "authorized_keys")

	// 2. Testlerde kullanılacak sabit anahtarlar tanımla.
	key1 := "ssh-rsa AAAA... key-one@example.com"
	key2 := "ssh-ed25519 BBBB... key-two@example.com"
	key3_commented := "# ssh-rsa CCCC... commented-key@example.com"

	// --- TEST SENARYOSU 1: Anahtar Ekleme ---
	t.Run("should add a key with command to an empty file", func(t *testing.T) {
		// Ortam değişkenlerini test için ayarla
		t.Setenv("GUARDIAN_AGENT_SERVER_ID", "99")

		// Fonksiyonu çağır
		err := addKeyToSpecificFile(authKeysFile, key1, 101)
		assert.NoError(t, err)

		// Dosyanın içeriğini kontrol et
		content, err := os.ReadFile(authKeysFile)
		assert.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, `command="/usr/local/bin/guardian-agent proxy --rule-id=101"`)
		assert.Contains(t, contentStr, `environment="GUARDIAN_AGENT_SERVER_ID=99"`)
		assert.Contains(t, contentStr, key1)
		assert.True(t, strings.HasSuffix(contentStr, "\n"), "File should end with a newline")
	})

	// --- TEST SENARYOSU 2: İkinci Anahtar Ekleme ---
	t.Run("should append a second key to the file", func(t *testing.T) {
		err := addKeyToSpecificFile(authKeysFile, key2, 102)
		assert.NoError(t, err)

		content, _ := os.ReadFile(authKeysFile)
		contentStr := string(content)

		// Her iki anahtarın da dosyada olduğundan emin ol
		assert.Contains(t, contentStr, key1)
		assert.Contains(t, contentStr, key2)
		assert.Contains(t, contentStr, "--rule-id=101")
		assert.Contains(t, contentStr, "--rule-id=102")

		// Dosyada en az iki satır olmalı
		lines := strings.Split(strings.TrimSpace(contentStr), "\n")
		assert.Len(t, lines, 2)
	})

	// --- TEST SENARYOSU 3: Anahtar Silme ---
	t.Run("should remove a specific key but keep others", func(t *testing.T) {
		err := removeKeyFromSpecificFile(authKeysFile, key1)
		assert.NoError(t, err)

		content, _ := os.ReadFile(authKeysFile)
		contentStr := string(content)

		// Silinen anahtarın artık dosyada OLMADIĞINDAN emin ol
		assert.NotContains(t, contentStr, key1)
		assert.NotContains(t, contentStr, "--rule-id=101")

		// Diğer anahtarın hala dosyada OLDUĞUNDAN emin ol
		assert.Contains(t, contentStr, key2)
		assert.Contains(t, contentStr, "--rule-id=102")
	})

	// --- TEST SENARYOSU 4: Var Olmayan Anahtarı Silme ---
	t.Run("should return ErrKeyNotFound when trying to remove a non-existent key", func(t *testing.T) {
		// key1'i zaten sildik, tekrar silmeye çalışalım
		err := removeKeyFromSpecificFile(authKeysFile, key1)
		assert.Error(t, err)
		assert.Equal(t, ErrKeyNotFound, err)
	})

	// --- TEST SENARYOSU 5: Yorum Satırlarını Korumalı ---
	t.Run("should not remove commented out keys", func(t *testing.T) {
		// Dosyanın sonuna yorumlu bir anahtar ekleyelim
		f, _ := os.OpenFile(authKeysFile, os.O_APPEND|os.O_WRONLY, 0600)
		f.WriteString(key3_commented + "\n")
		f.Close()

		// key2'yi silelim
		err := removeKeyFromSpecificFile(authKeysFile, key2)
		assert.NoError(t, err)

		content, _ := os.ReadFile(authKeysFile)
		contentStr := string(content)

		// key2'nin silindiğinden emin ol
		assert.NotContains(t, contentStr, key2)

		// Yorumlu anahtarın hala orada olduğundan emin ol
		assert.Contains(t, contentStr, key3_commented)
	})
}
