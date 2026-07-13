package cmd

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var getAuditLogsCmd = &cobra.Command{
	Use:     "audit-logs",
	Aliases: []string{"audit", "audit-log"},
	Short:   "Denetim kayıtlarını listeler (filtre + sayfalama)",
	Run: func(cmd *cobra.Command, args []string) {
		page, _ := cmd.Flags().GetInt("page")
		limit, _ := cmd.Flags().GetInt("limit")
		search, _ := cmd.Flags().GetString("search")
		action, _ := cmd.Flags().GetString("action")
		status, _ := cmd.Flags().GetString("status")

		logs, err := apiClient.ListAuditLogs(page, limit, search, action, status)
		if err != nil {
			log.Fatalf("Denetim kayıtları alınamadı: %v", err)
		}
		if len(logs) == 0 {
			fmt.Println("Denetim kaydı bulunamadı.")
			return
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tADMIN\tACTION\tTARGET\tSTATUS\tTIME\tERROR")
		fmt.Fprintln(w, "--\t-----\t------\t------\t------\t----\t-----")
		for _, l := range logs {
			target := l.TargetType
			if l.TargetID != nil {
				target = fmt.Sprintf("%s#%d", l.TargetType, *l.TargetID)
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
				l.ID, l.AdminRef, l.Action, target, l.Status, l.CreatedAt.Format(time.RFC822), l.ErrorMessage)
		}
		w.Flush()
	},
}

func init() {
	getCmd.AddCommand(getAuditLogsCmd)
	getAuditLogsCmd.Flags().Int("page", 1, "Sayfa numarası")
	getAuditLogsCmd.Flags().Int("limit", 20, "Sayfa başına kayıt")
	getAuditLogsCmd.Flags().String("search", "", "Serbest metin araması")
	getAuditLogsCmd.Flags().String("action", "", "Eyleme göre filtre (örn: CREATE_RULE)")
	getAuditLogsCmd.Flags().String("status", "", "Duruma göre filtre (örn: SUCCESS, FAILURE)")
}
