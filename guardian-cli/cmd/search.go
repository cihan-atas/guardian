package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [sorgu]",
	Short: "Tüm oturumlarda çalıştırılmış komutları arar",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.Join(args, " ")
		limit, _ := cmd.Flags().GetInt("limit")

		matches, err := apiClient.SearchCommands(query, limit)
		if err != nil {
			log.Fatalf("Komut araması başarısız: %v", err)
		}
		if len(matches) == 0 {
			fmt.Println("Eşleşen komut bulunamadı.")
			return
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "SESSION\tUSER\tSERVER\tTIME\tCOMMAND")
		fmt.Fprintln(w, "-------\t----\t------\t----\t-------")
		for _, m := range matches {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
				m.SessionID, m.Username, m.ServerHostname, m.StartTime.Format(time.RFC822), m.Command)
		}
		w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().Int("limit", 100, "Maksimum eşleşme sayısı")
}
