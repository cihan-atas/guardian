// guardian/guardian-server/services/risky_command_service.go
//
// Canlı oturumlarda çalıştırılan komutları riskli kalıplara karşı denetler.
// Tespit tamamen sunucu tarafında yapılır (UI'daki görsel vurgudan bağımsız);
// eşleşme bir "alert" doğurur (bkz. alert_service.go).

package services

import "regexp"

// RiskSeverity, bir riskli komut eşleşmesinin ciddiyet düzeyi.
type RiskSeverity string

const (
	SeverityHigh     RiskSeverity = "high"
	SeverityCritical RiskSeverity = "critical"
)

// RiskyRule, tek bir riskli komut kalıbı.
type RiskyRule struct {
	Name     string
	Pattern  *regexp.Regexp
	Severity RiskSeverity
}

// RiskyMatch, bir komutun eşleştiği kuralı taşır.
type RiskyMatch struct {
	Command  string
	RuleName string
	Severity RiskSeverity
}

// riskyRules, denetlenen kalıp listesi. "critical" olanlar (yıkıcı/geri
// dönüşü zor işlemler) opsiyonel otomatik aksiyonu tetikleyebilir; "high"
// olanlar yalnızca uyarı üretir.
var riskyRules = []RiskyRule{
	{"Özyinelemeli kök silme", regexp.MustCompile(`\brm\s+(-[a-zA-Z]*\s+)*-?[a-zA-Z]*[rf][a-zA-Z]*\s+(/|/\s|\*|~|\$HOME|/etc|/var|/usr)`), SeverityCritical},
	{"Disk üzerine dd", regexp.MustCompile(`\bdd\b[^\n]*\bof=/dev/`), SeverityCritical},
	{"Dosya sistemi biçimlendirme", regexp.MustCompile(`\bmkfs(\.\w+)?\b`), SeverityCritical},
	{"Fork bomb", regexp.MustCompile(`:\s*\(\s*\)\s*\{.*\|.*&\s*\}`), SeverityCritical},
	{"Aygıta doğrudan yazma", regexp.MustCompile(`>\s*/dev/(sd[a-z]|nvme\d|vd[a-z])`), SeverityCritical},
	{"Sistem kapatma/yeniden başlatma", regexp.MustCompile(`\b(shutdown|reboot|poweroff|halt|init\s+0)\b`), SeverityCritical},

	{"İndir ve çalıştır (curl|sh)", regexp.MustCompile(`\b(curl|wget)\b[^\n|]*\|\s*(sudo\s+)?(ba)?sh\b`), SeverityHigh},
	{"Yetki yükseltme (sudo/su)", regexp.MustCompile(`\b(sudo|su)\b`), SeverityHigh},
	{"Geniş izin (chmod 777)", regexp.MustCompile(`\bchmod\s+(-R\s+)?0?777\b`), SeverityHigh},
	{"Sahiplik değişimi (chown)", regexp.MustCompile(`\bchown\s+(-R\s+)`), SeverityHigh},
	{"Güvenlik duvarını temizleme", regexp.MustCompile(`\biptables\s+-F\b`), SeverityHigh},
	{"Geçmişi temizleme", regexp.MustCompile(`\bhistory\s+-c\b`), SeverityHigh},
	{"Parola değişimi", regexp.MustCompile(`\bpasswd\b`), SeverityHigh},
	{"Kullanıcı ekle/sil", regexp.MustCompile(`\b(useradd|userdel|adduser|deluser)\b`), SeverityHigh},
	{"Hassas dosyaya erişim", regexp.MustCompile(`/etc/(passwd|shadow|sudoers)`), SeverityHigh},
	{"Servis durdurma/devre dışı", regexp.MustCompile(`\bsystemctl\s+(stop|disable|mask)\b`), SeverityHigh},
	{"Ters kabuk (nc/ncat)", regexp.MustCompile(`\bn(et)?cat\b|\bnc\s+-[a-z]*e\b`), SeverityHigh},
}

// DetectRisky, komutu kurallara karşı denetler ve en yüksek ciddiyetli
// eşleşmeyi döner. Eşleşme yoksa nil döner.
func DetectRisky(command string) *RiskyMatch {
	var best *RiskyMatch
	for i := range riskyRules {
		rule := &riskyRules[i]
		if rule.Pattern.MatchString(command) {
			if best == nil || (best.Severity == SeverityHigh && rule.Severity == SeverityCritical) {
				best = &RiskyMatch{Command: command, RuleName: rule.Name, Severity: rule.Severity}
				if rule.Severity == SeverityCritical {
					break
				}
			}
		}
	}
	return best
}

// CleanCommand, ham girdi tamponundan (keystroke akışı) temizlenmiş komut
// metni üretir. cleanString ile aynı escape/kontrol temizliğini uygular;
// canlı tespitte handler bu fonksiyonu kullanır.
func CleanCommand(raw string) string {
	return cleanString(raw)
}
