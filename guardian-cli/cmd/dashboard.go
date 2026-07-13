package cmd

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var getStatsCmd = &cobra.Command{
	Use:     "stats",
	Aliases: []string{"dashboard"},
	Short:   "Gösterge paneli özet istatistiklerini gösterir",
	Run: func(cmd *cobra.Command, args []string) {
		stats, err := apiClient.GetDashboardStats()
		if err != nil {
			log.Fatalf("İstatistikler alınamadı: %v", err)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintf(w, "Aktif oturumlar:\t%d\n", stats.ActiveSessions)
		fmt.Fprintf(w, "Bugünkü oturumlar:\t%d\n", stats.TodaySessions)
		fmt.Fprintf(w, "Başarısız oturumlar:\t%d\n", stats.FailedSessions)
		fmt.Fprintf(w, "Bekleyen kurallar:\t%d\n", stats.PendingRules)
		fmt.Fprintf(w, "Süresi dolmuş kurallar:\t%d\n", stats.ExpiredRules)
		fmt.Fprintf(w, "Toplam sunucular:\t%d\n", stats.TotalServers)
		fmt.Fprintf(w, "Toplam kullanıcılar:\t%d\n", stats.TotalUsers)
		fmt.Fprintf(w, "Toplam anahtarlar:\t%d\n", stats.TotalKeys)
		fmt.Fprintf(w, "Yasaklı anahtarlar:\t%d\n", stats.BannedKeys)
		w.Flush()
	},
}

var getAlertsCmd = &cobra.Command{
	Use:     "alerts",
	Aliases: []string{"alert"},
	Short:   "Son güvenlik uyarılarını (riskli komut tespiti) listeler",
	Run: func(cmd *cobra.Command, args []string) {
		limit, _ := cmd.Flags().GetInt("limit")
		alerts, err := apiClient.GetAlerts(limit)
		if err != nil {
			log.Fatalf("Uyarılar alınamadı: %v", err)
		}
		if len(alerts) == 0 {
			fmt.Println("Uyarı bulunamadı.")
			return
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tSEVERITY\tRULE\tUSER\tSERVER\tACTION\tTIME\tCOMMAND")
		fmt.Fprintln(w, "--\t--------\t----\t----\t------\t------\t----\t-------")
		for _, a := range alerts {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				a.ID, a.Severity, a.RuleName, a.Username, a.ServerHostname, a.ActionTaken,
				a.CreatedAt.Format(time.RFC822), a.Command)
		}
		w.Flush()
	},
}

func init() {
	getCmd.AddCommand(getStatsCmd)
	getCmd.AddCommand(getAlertsCmd)
	getAlertsCmd.Flags().Int("limit", 20, "Gösterilecek uyarı sayısı")
}
