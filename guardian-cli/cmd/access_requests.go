package cmd

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var getAccessRequestsCmd = &cobra.Command{
	Use:     "access-requests",
	Aliases: []string{"access-request", "requests"},
	Short:   "Erişim taleplerini listeler (onay akışı)",
	Run: func(cmd *cobra.Command, args []string) {
		status, _ := cmd.Flags().GetString("status")
		reqs, err := apiClient.ListAccessRequests(status)
		if err != nil {
			log.Fatalf("Erişim talepleri alınamadı: %v", err)
		}
		if len(reqs) == 0 {
			fmt.Println("Erişim talebi bulunamadı.")
			return
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tSERVER\tUSER\tKEY NAME\tREQUESTED BY\tVALID UNTIL\tREASON")
		fmt.Fprintln(w, "--\t------\t------\t----\t--------\t------------\t-----------\t------")
		for _, r := range reqs {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				r.ID, r.Status, r.ServerHostname, r.Username, r.KeyName, r.RequestedBy,
				r.ValidUntil.Format(time.RFC822), r.RequestReason)
		}
		w.Flush()
	},
}

var approveRequestCmd = &cobra.Command{
	Use:   "request [ID]",
	Short: "Bir erişim talebini onaylar",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}
		if err := apiClient.ApproveAccessRequest(id); err != nil {
			log.Fatalf("Talep onaylanamadı: %v", err)
		}
		fmt.Printf("✅ Erişim talebi ID %d onaylandı.\n", id)
	},
}

var rejectRequestCmd = &cobra.Command{
	Use:   "request [ID]",
	Short: "Bir erişim talebini reddeder",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}
		reason, _ := cmd.Flags().GetString("reason")
		if err := apiClient.RejectAccessRequest(id, reason); err != nil {
			log.Fatalf("Talep reddedilemedi: %v", err)
		}
		fmt.Printf("✅ Erişim talebi ID %d reddedildi.\n", id)
	},
}

func init() {
	getCmd.AddCommand(getAccessRequestsCmd)
	getAccessRequestsCmd.Flags().String("status", "", "Duruma göre filtrele (örn: pending, approved, rejected)")

	var approveCmd = &cobra.Command{Use: "approve", Short: "Bir kaynağı onaylar (request)"}
	var rejectCmd = &cobra.Command{Use: "reject", Short: "Bir kaynağı reddeder (request)"}
	rootCmd.AddCommand(approveCmd, rejectCmd)
	approveCmd.AddCommand(approveRequestCmd)
	rejectCmd.AddCommand(rejectRequestCmd)

	rejectRequestCmd.Flags().String("reason", "", "Reddetme gerekçesi")
}
