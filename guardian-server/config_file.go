package main

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// loadConfigFileIntoEnv, server.conf tarzı KEY=VALUE dosyasını okuyup env'de
// HENÜZ AYARLANMAMIŞ anahtarları doldurur. Linux'ta systemd EnvironmentFile
// bunları zaten yüklediği için no-op olur; systemd olmayan ortamlarda
// (ör. Windows) sunucunun yapılandırma kaynağı budur. Yol:
// GUARDIAN_SERVER_CONFIG ya da OS varsayılanı.
func loadConfigFileIntoEnv() {
	path := os.Getenv("GUARDIAN_SERVER_CONFIG")
	if path == "" {
		path = defaultServerConfigPath()
	}
	f, err := os.Open(path)
	if err != nil {
		return // dosya yoksa sessiz geç (env'den okumaya devam edilir)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"`)
		if key != "" && os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

// defaultServerConfigPath, OS'a göre varsayılan server.conf yolunu döndürür.
func defaultServerConfigPath() string {
	if runtime.GOOS == "windows" {
		pd := os.Getenv("ProgramData")
		if pd == "" {
			pd = `C:\ProgramData`
		}
		return filepath.Join(pd, "guardian", "server.conf")
	}
	return "/etc/guardian/server.conf"
}
