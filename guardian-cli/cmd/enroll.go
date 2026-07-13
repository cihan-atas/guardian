package cmd

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var enrollCmd = &cobra.Command{
	Use:   "enroll [SERVER-ID]",
	Short: "Bir sunucu için agent kayıt token'ı ve kurulum komutu üretir",
	Long: `Belirtilen sunucu için tek kullanımlık bir kayıt (enroll) token'ı üretir ve
hedef makinede çalıştırılacak kurulum komutunu ekrana yazar.

Örnekler:
  guardian-cli enroll 1
  guardian-cli enroll 1 --os windows --days 730`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		serverID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: SERVER-ID sayısal bir değer olmalıdır: %v", err)
		}
		osName, _ := cmd.Flags().GetString("os")
		days, _ := cmd.Flags().GetInt("days")

		resp, err := apiClient.GenerateEnrollToken(serverID, days, osName)
		if err != nil {
			log.Fatalf("Kayıt token'ı üretilemedi: %v", err)
		}

		fmt.Printf("Sunucu: %s (%s)\n", resp.ServerHostname, resp.ServerIP)
		fmt.Printf("Token son geçerlilik: %s\n", resp.ExpiresAt.Format(time.RFC1123))
		if !resp.BinaryAvailable {
			fmt.Printf("⚠️  %s için agent ikili dosyası sunucuda mevcut değil; kurulum başarısız olabilir.\n", resp.OS)
		}
		fmt.Println("\nHedef makinede çalıştırın:")
		fmt.Println(resp.InstallCommand)
	},
}

func init() {
	rootCmd.AddCommand(enrollCmd)
	enrollCmd.Flags().String("os", "linux", "Hedef işletim sistemi: linux | windows")
	enrollCmd.Flags().Int("days", 0, "Agent sertifikası geçerlilik süresi (gün); 0 = sunucu varsayılanı")
}
