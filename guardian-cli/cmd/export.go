package cmd

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var exportSessionCmd = &cobra.Command{
	Use:   "session [ID]",
	Short: "Bir oturumu asciinema (.cast) formatında dosyaya aktarır",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sessionID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}
		output, _ := cmd.Flags().GetString("output")
		if output == "" {
			output = fmt.Sprintf("session-%d.cast", sessionID)
		}

		data, err := apiClient.ExportSessionAsciicast(sessionID)
		if err != nil {
			log.Fatalf("Oturum dışa aktarılamadı: %v", err)
		}
		if err := os.WriteFile(output, data, 0644); err != nil {
			log.Fatalf("Dosya yazılamadı '%s': %v", output, err)
		}
		fmt.Printf("✅ Oturum %d, '%s' dosyasına aktarıldı (%d bayt).\n", sessionID, output, len(data))
	},
}

func init() {
	var exportCmd = &cobra.Command{Use: "export", Short: "Bir kaynağı dışa aktarır (session)"}
	rootCmd.AddCommand(exportCmd)
	exportCmd.AddCommand(exportSessionCmd)

	exportSessionCmd.Flags().String("output", "", "Çıktı dosyası yolu (varsayılan: session-<ID>.cast)")
}
