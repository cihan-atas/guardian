package cmd

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var banKeyCmd = &cobra.Command{
	Use:   "key [ID]",
	Short: "Bir public anahtarı geçici olarak yasaklar",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		keyID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}
		durationMinutes, _ := cmd.Flags().GetInt("minutes")
		reason, _ := cmd.Flags().GetString("reason")

		ban, err := apiClient.BanKey(keyID, durationMinutes, reason)
		if err != nil {
			log.Fatalf("Anahtar yasaklanamadı: %v", err)
		}
		fmt.Printf("✅ Anahtar ID %d yasaklandı. Bitiş: %s\n", keyID, ban.BannedUntil.Format(time.RFC1123))
	},
}

var unbanKeyCmd = &cobra.Command{
	Use:   "key [ID]",
	Short: "Bir public anahtarın yasağını kaldırır",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		keyID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}
		if err := apiClient.UnbanKey(keyID); err != nil {
			log.Fatalf("Yasak kaldırılamadı: %v", err)
		}
		fmt.Printf("✅ Anahtar ID %d yasağı kaldırıldı.\n", keyID)
	},
}

func init() {
	var banCmd = &cobra.Command{Use: "ban", Short: "Bir kaynağı yasaklar (key)"}
	var unbanCmd = &cobra.Command{Use: "unban", Short: "Bir kaynağın yasağını kaldırır (key)"}
	rootCmd.AddCommand(banCmd, unbanCmd)
	banCmd.AddCommand(banKeyCmd)
	unbanCmd.AddCommand(unbanKeyCmd)

	banKeyCmd.Flags().Int("minutes", 60, "Yasak süresi (dakika)")
	banKeyCmd.Flags().String("reason", "", "Yasaklama gerekçesi")
}
