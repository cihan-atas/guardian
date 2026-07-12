package main

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// loadConfigFileIntoEnv, agent.conf tarzı KEY=VALUE dosyasını okuyup env'de
// HENÜZ AYARLANMAMIŞ anahtarları doldurur. Linux'ta systemd EnvironmentFile
// bunları zaten yüklediği için no-op olur; Windows'ta (systemd yok) agent'ın
// yapılandırma kaynağı budur. Yol: GUARDIAN_AGENT_CONFIG ya da OS varsayılanı.
func loadConfigFileIntoEnv() {
	path := os.Getenv("GUARDIAN_AGENT_CONFIG")
	if path == "" {
		path = defaultAgentConfigPath()
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

// defaultAgentConfigPath, OS'a göre varsayılan agent.conf yolunu döndürür.
func defaultAgentConfigPath() string {
	if runtime.GOOS == "windows" {
		pd := os.Getenv("ProgramData")
		if pd == "" {
			pd = `C:\ProgramData`
		}
		return filepath.Join(pd, "guardian", "agent.conf")
	}
	return "/etc/guardian/agent.conf"
}
