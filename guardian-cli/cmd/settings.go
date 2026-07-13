package cmd

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var getSettingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Bildirim/alarm ve saklama ayarlarını gösterir",
	Run: func(cmd *cobra.Command, args []string) {
		s, err := apiClient.GetSettings()
		if err != nil {
			log.Fatalf("Ayarlar alınamadı: %v", err)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintf(w, "Webhook URL:\t%s\n", s.WebhookURL)
		fmt.Fprintf(w, "SMTP host:\t%s\n", s.SMTPHost)
		fmt.Fprintf(w, "SMTP port:\t%s\n", s.SMTPPort)
		fmt.Fprintf(w, "SMTP user:\t%s\n", s.SMTPUser)
		fmt.Fprintf(w, "SMTP parola tanımlı:\t%t\n", s.SMTPPassSet)
		fmt.Fprintf(w, "SMTP from:\t%s\n", s.SMTPFrom)
		fmt.Fprintf(w, "Alarm e-posta alıcısı:\t%s\n", s.AlertEmailTo)
		fmt.Fprintf(w, "Riskli komut otomatik eylem:\t%s\n", s.RiskyAutoaction)
		fmt.Fprintf(w, "Saklama süresi (gün):\t%d\n", s.RetentionDays)
		if s.RetentionLastRun != "" {
			fmt.Fprintf(w, "Son temizlik:\t%s (silinen: %d)\n", s.RetentionLastRun, s.RetentionLastDeleted)
		}
		w.Flush()
	},
}

var updateSettingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Bildirim/alarm ve saklama ayarlarını günceller",
	Long: `Ayarları günceller. Yalnızca verilen flag'ler değiştirilir; kalan alanlar
mevcut değerleriyle korunur (arka planda GET + PUT yapılır).`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		changed := false
		for _, name := range settingsFlagNames {
			if cmd.Flags().Changed(name) {
				changed = true
			}
		}
		if !changed {
			return fmt.Errorf("güncelleme için en az bir flag belirtilmelidir: --%s", settingsFlagNames[0])
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// PUT tam nesneyi beklediğinden mevcut ayarları çekip üstüne yazıyoruz.
		s, err := apiClient.GetSettings()
		if err != nil {
			log.Fatalf("Mevcut ayarlar alınamadı: %v", err)
		}
		// Salt-okunur alanlar geri gönderilmemeli.
		s.RetentionLastRun = ""
		s.RetentionLastDeleted = 0
		s.SMTPPassSet = false

		if cmd.Flags().Changed("webhook-url") {
			s.WebhookURL, _ = cmd.Flags().GetString("webhook-url")
		}
		if cmd.Flags().Changed("smtp-host") {
			s.SMTPHost, _ = cmd.Flags().GetString("smtp-host")
		}
		if cmd.Flags().Changed("smtp-port") {
			s.SMTPPort, _ = cmd.Flags().GetString("smtp-port")
		}
		if cmd.Flags().Changed("smtp-user") {
			s.SMTPUser, _ = cmd.Flags().GetString("smtp-user")
		}
		if cmd.Flags().Changed("smtp-pass") {
			s.SMTPPass, _ = cmd.Flags().GetString("smtp-pass")
		}
		if cmd.Flags().Changed("smtp-from") {
			s.SMTPFrom, _ = cmd.Flags().GetString("smtp-from")
		}
		if cmd.Flags().Changed("alert-email-to") {
			s.AlertEmailTo, _ = cmd.Flags().GetString("alert-email-to")
		}
		if cmd.Flags().Changed("risky-autoaction") {
			s.RiskyAutoaction, _ = cmd.Flags().GetString("risky-autoaction")
		}
		if cmd.Flags().Changed("retention-days") {
			s.RetentionDays, _ = cmd.Flags().GetInt("retention-days")
		}

		if err := apiClient.UpdateSettings(*s); err != nil {
			log.Fatalf("Ayarlar güncellenemedi: %v", err)
		}
		fmt.Println("✅ Ayarlar başarıyla güncellendi.")
	},
}

var testNotificationCmd = &cobra.Command{
	Use:   "test-notification",
	Short: "Yapılandırılmış bildirim kanallarına (webhook/e-posta) test bildirimi gönderir",
	Run: func(cmd *cobra.Command, args []string) {
		if err := apiClient.TestNotification(); err != nil {
			log.Fatalf("Test bildirimi gönderilemedi: %v", err)
		}
		fmt.Println("✅ Test bildirimi gönderildi.")
	},
}

// settingsFlagNames, update settings için tanımlı tüm flag adlarını tutar
// (PreRunE'de "en az bir flag" kontrolü için).
var settingsFlagNames = []string{
	"webhook-url", "smtp-host", "smtp-port", "smtp-user", "smtp-pass",
	"smtp-from", "alert-email-to", "risky-autoaction", "retention-days",
}

func init() {
	getCmd.AddCommand(getSettingsCmd)
	updateCmd.AddCommand(updateSettingsCmd)
	rootCmd.AddCommand(testNotificationCmd)

	updateSettingsCmd.Flags().String("webhook-url", "", "Bildirim webhook URL'si")
	updateSettingsCmd.Flags().String("smtp-host", "", "SMTP sunucu adresi")
	updateSettingsCmd.Flags().String("smtp-port", "", "SMTP portu")
	updateSettingsCmd.Flags().String("smtp-user", "", "SMTP kullanıcı adı")
	updateSettingsCmd.Flags().String("smtp-pass", "", "SMTP parolası (yalnızca yeni değer girildiğinde güncellenir)")
	updateSettingsCmd.Flags().String("smtp-from", "", "Gönderen e-posta adresi")
	updateSettingsCmd.Flags().String("alert-email-to", "", "Alarm e-posta alıcısı")
	updateSettingsCmd.Flags().String("risky-autoaction", "", "Riskli komutta otomatik eylem: none | terminate | ban")
	updateSettingsCmd.Flags().Int("retention-days", 0, "Kayıt saklama süresi (gün); 0 = sınırsız")
}
