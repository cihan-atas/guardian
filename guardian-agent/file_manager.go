// guardian/guardian-agent/file_manager.go

package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var ErrKeyNotFound = errors.New("silinecek anahtar dosyada bulunamadı")

// getAuthorizedKeysPath, kullanıcı adına göre authorized_keys dosyasının yolunu bulur.
// Bu fonksiyonun kendisi test edilemez (sistemdeki gerçek kullanıcılara bağlı olduğu için),
// bu yüzden onu çağıran fonksiyonları, dosya yolunu parametre alacak şekilde ayırdık.
func getAuthorizedKeysPath(username string) (string, error) {
	usr, err := user.Lookup(username)
	if err != nil {
		return "", fmt.Errorf("kullanıcı '%s' bulunamadı: %w", username, err)
	}
	// Windows'ta yönetici hesapların anahtarları OpenSSH tarafından yalnızca
	// paylaşımlı administrators_authorized_keys dosyasından okunur. Hangi
	// kullanıcıların admin sayılacağı GUARDIAN_WINDOWS_ADMIN_USERS (virgüllü)
	// ile belirtilir. Diğer tüm durumlar per-user dosyayı kullanır.
	if runtime.GOOS == "windows" && isWindowsAdminUser(username) {
		pd := os.Getenv("ProgramData")
		if pd == "" {
			pd = `C:\ProgramData`
		}
		return filepath.Join(pd, "ssh", "administrators_authorized_keys"), nil
	}
	return filepath.Join(usr.HomeDir, ".ssh", "authorized_keys"), nil
}

// isWindowsAdminUser, kullanıcının GUARDIAN_WINDOWS_ADMIN_USERS listesinde olup
// olmadığını (büyük/küçük harf duyarsız) döndürür.
func isWindowsAdminUser(username string) bool {
	for _, u := range strings.Split(os.Getenv("GUARDIAN_WINDOWS_ADMIN_USERS"), ",") {
		if strings.EqualFold(strings.TrimSpace(u), username) {
			return true
		}
	}
	return false
}

// addKeyToSpecificFile, belirtilen bir dosyaya anahtar ekleyen, test edilebilir iç fonksiyondur.
func addKeyToSpecificFile(path string, publicKey string, ruleID int) error {
	sshDir := filepath.Dir(path)
	if _, err := os.Stat(sshDir); os.IsNotExist(err) {
		if err := os.MkdirAll(sshDir, 0700); err != nil {
			return fmt.Errorf(".ssh dizini oluşturulamadı: %w", err)
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("authorized_keys dosyası açılamadı: %w", err)
	}
	defer f.Close()

	trimmedPublicKey := strings.TrimSpace(publicKey)

	serverHost := os.Getenv("GUARDIAN_SERVER_HOST")
	serverPort := os.Getenv("GUARDIAN_SERVER_PORT")
	serverID := os.Getenv("GUARDIAN_AGENT_SERVER_ID")
	secretToken := os.Getenv("GUARDIAN_SECRET_TOKEN")
	agentSshKeyPath := os.Getenv("GUARDIAN_AGENT_SSH_KEY_PATH")

	caCertFile := os.Getenv("TLS_CA_FILE")
	agentCertFile := os.Getenv("AGENT_TLS_CERT_FILE")
	agentKeyFile := os.Getenv("AGENT_TLS_KEY_FILE")

	envVars := fmt.Sprintf(`environment="GUARDIAN_SERVER_HOST=%s",environment="GUARDIAN_SERVER_PORT=%s",environment="GUARDIAN_AGENT_SERVER_ID=%s",environment="GUARDIAN_SECRET_TOKEN=%s",environment="GUARDIAN_AGENT_SSH_KEY_PATH=%s",environment="TLS_CA_FILE=%s",environment="AGENT_TLS_CERT_FILE=%s",environment="AGENT_TLS_KEY_FILE=%s"`,
		serverHost, serverPort, serverID, secretToken, agentSshKeyPath, caCertFile, agentCertFile, agentKeyFile)

	agentPath := "/usr/local/bin/guardian-agent"
	command := fmt.Sprintf(`command="%s proxy --rule-id=%d"`, agentPath, ruleID)

	// "restrict" port/X11/agent-forwarding ve ~/.ssh/rc dahil her şeyi kapatır,
	// ama bu arada PTY tahsisini de (no-pty) kapatır. PTY olmadan istemci
	// yerel terminalini raw mode'a hiç almaz; tüm girdi satır satır (kernel'in
	// canonical/cooked mode tamponuyla) gönderilir, bu da ok tuşu/tab/Ctrl+C
	// gibi her şeyi bozar. OpenSSH seçenekleri soldan sağa işlendiği ve sonraki
	// seçenek önceki kısıtlamayı geçersiz kıldığı için "restrict"ten SONRA
	// "pty" ekleyerek sadece PTY'yi geri açıyoruz, diğer kısıtlamalar duruyor.
	lineToAdd := fmt.Sprintf("%s,%s,restrict,pty %s\n", envVars, command, trimmedPublicKey)

	if _, err := f.WriteString(lineToAdd); err != nil {
		return fmt.Errorf("anahtar dosyaya yazılamadı: %w", err)
	}
	return nil
}

// addKeyToFile, dışarıdan çağrılan ana fonksiyondur. Dosya yolunu bulur ve iç helper'ı çağırır.
func addKeyToFile(username, publicKey string, ruleID int) error {
	path, err := getAuthorizedKeysPath(username)
	if err != nil {
		return err
	}

	if err := addKeyToSpecificFile(path, publicKey, ruleID); err != nil {
		return err // Hata detayını olduğu gibi yukarı aktar.
	}

	log.Printf("✅ Anahtar, proxy komutuyla ve TÜM ortam değişkenleriyle birlikte kullanıcı '%s' için eklendi.", username)
	return nil
}

// removeKeyFromSpecificFile, belirtilen bir dosyadan anahtarı silen, test edilebilir iç fonksiyondur.
func removeKeyFromSpecificFile(path string, publicKey string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Dosya hiç yoksa, anahtar da yoktur.
		return ErrKeyNotFound
	}

	input, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("authorized_keys dosyası okunamadı: %w", err)
	}

	lines := strings.Split(string(input), "\n")
	var outputLines []string
	keyToRemove := strings.TrimSpace(publicKey)

	var keyWasRemoved = false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue // Boş satırları atla
		}

		// Sadece bizim eklediğimiz Guardian satırlarını (environment ile başlayan)
		// ve silmek istediğimiz anahtarı içerenleri silmeliyiz.
		// Kullanıcının normal, komutsuz anahtarlarını silmemeliyiz.
		if strings.Contains(trimmedLine, keyToRemove) && strings.HasPrefix(trimmedLine, "environment=") {
			keyWasRemoved = true
		} else {
			outputLines = append(outputLines, line)
		}
	}

	if !keyWasRemoved {
		return ErrKeyNotFound
	}

	output := strings.Join(outputLines, "\n")
	// Dosyada hala içerik varsa ve sonu newline ile bitmiyorsa, ekle.
	if len(output) > 0 && !strings.HasSuffix(output, "\n") {
		output += "\n"
	}

	if err := ioutil.WriteFile(path, []byte(output), 0600); err != nil {
		return fmt.Errorf("authorized_keys dosyası güncellenemedi: %w", err)
	}

	return nil
}

// removeKeyFromFile, dışarıdan çağrılan ana fonksiyondur. Dosya yolunu bulur ve iç helper'ı çağırır.
func removeKeyFromFile(username, publicKey string) error {
	path, err := getAuthorizedKeysPath(username)
	if err != nil {
		return err
	}
	return removeKeyFromSpecificFile(path, publicKey)
}

// PID yönetimi fonksiyonları (değişiklik yok)
const pidDir = "/opt/guardian/pids"

func createPidFile(sessionID int) error {
	if err := os.MkdirAll(pidDir, 0700); err != nil {
		return fmt.Errorf("PID dizini oluşturulamadı: %w", err)
	}

	pid := os.Getpid()
	pidFile := filepath.Join(pidDir, fmt.Sprintf("session_%d.pid", sessionID))

	err := ioutil.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
	if err != nil {
		return fmt.Errorf("PID dosyası yazılamadı: %w", err)
	}

	log.Printf("✅ PID dosyası oluşturuldu: %s (PID: %d)", pidFile, pid)
	return nil
}

func removePidFile(sessionID int) {
	pidFile := filepath.Join(pidDir, fmt.Sprintf("session_%d.pid", sessionID))
	err := os.Remove(pidFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[WARN] PID dosyası silinemedi: %v", err)
		}
	} else {
		log.Printf("✅ PID dosyası silindi: %s", pidFile)
	}
}

func terminateProcessBySessionID(sessionID int) error {
	pidFile := filepath.Join(pidDir, fmt.Sprintf("session_%d.pid", sessionID))

	pidBytes, err := ioutil.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("sonlandırılacak oturum (PID dosyası) bulunamadı: session_id %d", sessionID)
		}
		return fmt.Errorf("PID dosyası okunamadı: %w", err)
	}

	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		return fmt.Errorf("PID dosyasındaki içerik geçersiz: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		// Bu hata genellikle oluşmaz, asıl hata Kill() aşamasında gelir.
		// Yine de eski PID dosyasını temizleyelim.
		log.Printf("[WARN] Proses bulunamadı (PID: %d), eski PID dosyası temizleniyor.", pid)
		removePidFile(sessionID)
		return fmt.Errorf("proses bulunamadı (PID: %d): %w", pid, err)
	}

	log.Printf("❗️ Oturum nazikçe sonlandırılıyor (SIGTERM)... PID: %d, Session ID: %d", pid, sessionID)
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Proses zaten ölmüşse "no such process" hatası alınır. Bu normal bir durumdur.
		log.Printf("[WARN] SIGTERM gönderilemedi (muhtemelen zaten kapalı): %v", err)
	} else {
		// guardian-agent'ın proxy sürecine nazikçe kapanması (SSH oturumunu
		// düzgün kapatıp uzaktaki shell'in terminal modlarını sıfırlamasına
		// fırsat vermesi) için kısa bir süre tanıyoruz. Yanıt vermezse
		// SIGKILL ile zorlarız; sonsuza kadar takılı kalmasın diye.
		exited := false
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			if probeErr := process.Signal(syscall.Signal(0)); probeErr != nil {
				exited = true
				break
			}
		}
		if !exited {
			log.Printf("[WARN] Proses SIGTERM'e 2 saniye içinde yanıt vermedi, SIGKILL ile zorlanıyor: PID %d", pid)
			if err := process.Kill(); err != nil {
				log.Printf("[WARN] SIGKILL de başarısız (muhtemelen zaten kapalı): %v", err)
			}
		} else {
			log.Printf("✅ Oturum nazikçe sonlandırıldı: PID %d", pid)
		}
	}

	// Her durumda (başarılı veya başarısız sonlandırma) PID dosyasını temizle.
	removePidFile(sessionID)
	return nil
}
