//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Bu testler yalnızca Windows'ta derlenip çalışır (build tag: windows).
// Gerçek Windows runtime doğrulaması sırasında (`go test ./...` bir Windows
// host'ta koşturulduğunda) Windows'a özgü yol mantığını uçtan uca doğrular.

// TestGetAuthorizedKeysPath_WindowsAdmin, GUARDIAN_WINDOWS_ADMIN_USERS'ta
// listelenen bir kullanıcı için paylaşımlı administrators_authorized_keys
// yolunun (ProgramData tabanlı, ters bölü ayraçlı) döndüğünü doğrular.
// user.Lookup gerektirmemesi için hedef, admin dalının pür yol üretimidir.
func TestGetAuthorizedKeysPath_WindowsAdmin(t *testing.T) {
	t.Setenv("ProgramData", `C:\ProgramData`)
	got := windowsAdminAuthorizedKeysPath(`C:\ProgramData`)
	want := `C:\ProgramData\ssh\administrators_authorized_keys`
	if got != want {
		t.Fatalf("Windows admin yolu beklenenle eşleşmiyor:\n got=%q\nwant=%q", got, want)
	}
	// Windows'ta ayraç ters bölü olmalı.
	if strings.Contains(got, "/") {
		t.Errorf("Windows yolunda ileri bölü olmamalı: %q", got)
	}
}

// TestDefaultAgentConfigPath_Windows, ProgramData altında guardian\agent.conf
// beklendiğini doğrular.
func TestDefaultAgentConfigPath_Windows(t *testing.T) {
	t.Setenv("ProgramData", `C:\ProgramData`)
	got := defaultAgentConfigPath()
	want := filepath.Join(`C:\ProgramData`, "guardian", "agent.conf")
	if got != want {
		t.Errorf("Windows config yolu beklenenle eşleşmiyor: %q != %q", got, want)
	}
}
