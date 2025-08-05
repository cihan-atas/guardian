// guardian-cli/cmd/sessions.go (GÜNCELLENMİŞ HALİ)

package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var getSessionsCmd = &cobra.Command{
	Use:     "sessions",
	Aliases: []string{"session"},
	Short:   "Tüm oturumları listeler",
	Run: func(cmd *cobra.Command, args []string) {
		sessions, err := apiClient.ListSessions()
		if err != nil {
			log.Fatalf("Oturumlar alınamadı: %v", err)
		}

		if len(sessions) == 0 {
			fmt.Println("Kayıtlı oturum bulunamadı.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tUSER\tSERVER ID\tSTART TIME\tEND TIME")
		fmt.Fprintln(w, "--\t------\t----\t---------\t----------\t--------")
		for _, s := range sessions {
			endTime := "-"
			if s.EndTime != nil {
				endTime = s.EndTime.Format(time.RFC822)
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s\t%s\n", s.ID, s.Status, s.Username, s.ServerID, s.StartTime.Format(time.RFC822), endTime)
		}
		w.Flush()
	},
}

var getSessionCommandsCmd = &cobra.Command{
	Use:   "session-commands [ID]",
	Short: "Bir oturumun komut geçmişini görüntüler",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sessionID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}

		details, err := apiClient.GetSessionDetails(sessionID)
		if err != nil {
			log.Fatalf("Oturum detayları alınamadı: %v", err)
		}

		fmt.Printf("--- Oturum Bilgileri (ID: %d) ---\n", details.SessionInfo.ID)
		fmt.Printf("Kullanıcı: %s\n", details.SessionInfo.Username)
		fmt.Printf("Sunucu: %s (%s)\n", details.SessionInfo.ServerHostname, details.SessionInfo.ServerIP)
		fmt.Printf("Durum: %s\n\n", details.SessionInfo.Status)

		if len(details.Commands) == 0 {
			fmt.Println("Bu oturumda kayıtlı komut bulunamadı.")
			return
		}

		fmt.Println("--- Komut Geçmişi ---")
		for _, c := range details.Commands {
			fmt.Printf("\n# [%s] $ %s\n", c.Timestamp.Format(time.RFC1123), c.Command)
			fmt.Println(strings.TrimSpace(c.Output))
		}
	},
}

var terminateSessionCmd = &cobra.Command{
	Use:   "session [ID]",
	Short: "Aktif bir oturumu zorla sonlandırır",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sessionID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}

		err = apiClient.TerminateSession(sessionID)
		if err != nil {
			log.Fatalf("Oturum sonlandırılamadı: %v", err)
		}
		fmt.Printf("✅ Oturum %d için sonlandırma komutu başarıyla gönderildi.\n", sessionID)
	},
}

var watchSessionCmd = &cobra.Command{
	Use:   "session [ID]",
	Short: "Aktif bir oturumu tarayıcıda canlı izler",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sessionID := args[0]

		// DEĞİŞİKLİK: API URL'si yerine UI URL'sini kullan
		uiHost := os.Getenv("GUARDIAN_UI_HOST")
		if uiHost == "" {
			log.Fatal("HATA: GUARDIAN_UI_HOST ortam değişkeni ayarlanmamış. (Örnek: export GUARDIAN_UI_HOST=\"http://localhost\")")
		}

		// DEĞİŞİKLİK: URL'yi Angular'ın route yapısına uygun şekilde oluştur (/live/:id)
		url := fmt.Sprintf("%s/live/%s", uiHost, sessionID)
		openBrowser(url)
	},
}

var replaySessionCmd = &cobra.Command{
	Use:   "session [ID]",
	Short: "Tamamlanmış bir oturumu tarayıcıda tekrar oynatır",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sessionID := args[0]

		// DEĞİŞİKLİK: API URL'si yerine UI URL'sini kullan
		uiHost := os.Getenv("GUARDIAN_UI_HOST")
		if uiHost == "" {
			log.Fatal("HATA: GUARDIAN_UI_HOST ortam değişkeni ayarlanmamış. (Örnek: export GUARDIAN_UI_HOST=\"http://localhost\")")
		}

		// DEĞİŞİKLİK: URL'yi Angular'ın route yapısına uygun şekilde oluştur (/replay/:id)
		url := fmt.Sprintf("%s/replay/%s", uiHost, sessionID)
		openBrowser(url)
	},
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("desteklenmeyen platform")
	}
	if err != nil {
		log.Printf("Tarayıcı açılamadı: %v\nLütfen şu adresi manuel olarak ziyaret edin: %s", err, url)
	} else {
		fmt.Printf("Tarayıcıda şu adres açılıyor: %s\n", url)
	}
}

func init() {
	getCmd.AddCommand(getSessionsCmd)
	getCmd.AddCommand(getSessionCommandsCmd)

	var terminateCmd = &cobra.Command{Use: "terminate", Short: "Aktif bir kaynağı sonlandırır (session)", Aliases: []string{"kill"}}
	var watchCmd = &cobra.Command{Use: "watch", Short: "Bir kaynağı canlı izler (session)"}
	var replayCmd = &cobra.Command{Use: "replay", Short: "Bir kaynağın tekrarını oynatır (session)"}

	rootCmd.AddCommand(terminateCmd, watchCmd, replayCmd)

	terminateCmd.AddCommand(terminateSessionCmd)
	watchCmd.AddCommand(watchSessionCmd)
	replayCmd.AddCommand(replaySessionCmd)
}
