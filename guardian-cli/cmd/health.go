package cmd

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var getServerHealthCmd = &cobra.Command{
	Use:     "server-health",
	Aliases: []string{"health"},
	Short:   "Sunucu agent'larının sağlık/çevrimiçi durumunu gösterir",
	Run: func(cmd *cobra.Command, args []string) {
		health, err := apiClient.GetServersHealth()
		if err != nil {
			log.Fatalf("Sağlık durumu alınamadı: %v", err)
		}
		if len(health) == 0 {
			fmt.Println("Kayıtlı sunucu bulunamadı.")
			return
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "SERVER ID\tHOSTNAME\tIP ADDRESS\tSTATUS\tLATENCY (ms)")
		fmt.Fprintln(w, "---------\t--------\t----------\t------\t-----------")
		for _, h := range health {
			status := "çevrimdışı"
			if h.Online {
				status = "çevrimiçi"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%.1f\n", h.ServerID, h.Hostname, h.IPAddress, status, h.LatencyMS)
		}
		w.Flush()
	},
}

func init() {
	getCmd.AddCommand(getServerHealthCmd)
}
