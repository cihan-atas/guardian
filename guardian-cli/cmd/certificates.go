package cmd

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var getCertificatesCmd = &cobra.Command{
	Use:     "certificates",
	Aliases: []string{"certs", "certificate"},
	Short:   "CA, sunucu ve agent sertifikalarının süre-sonu özetini gösterir",
	Run: func(cmd *cobra.Command, args []string) {
		certs, err := apiClient.GetCertificates()
		if err != nil {
			log.Fatalf("Sertifikalar alınamadı: %v", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "SCOPE\tSUBJECT\tNOT AFTER\tDAYS LEFT")
		fmt.Fprintln(w, "-----\t-------\t---------\t---------")
		if certs.CA != nil {
			fmt.Fprintf(w, "CA\t%s\t%s\t%d\n", certs.CA.Subject, certs.CA.NotAfter, certs.CA.DaysLeft)
		} else if certs.CAError != "" {
			fmt.Fprintf(w, "CA\t(hata: %s)\t-\t-\n", certs.CAError)
		}
		if certs.Server != nil {
			fmt.Fprintf(w, "SERVER\t%s\t%s\t%d\n", certs.Server.Subject, certs.Server.NotAfter, certs.Server.DaysLeft)
		} else if certs.ServerError != "" {
			fmt.Fprintf(w, "SERVER\t(hata: %s)\t-\t-\n", certs.ServerError)
		}
		for _, a := range certs.Agents {
			if a.Cert != nil {
				fmt.Fprintf(w, "AGENT %s\t%s\t%s\t%d\n", a.Hostname, a.Cert.Subject, a.Cert.NotAfter, a.Cert.DaysLeft)
			} else {
				online := "çevrimdışı"
				if a.Online {
					online = "sertifika okunamadı"
				}
				fmt.Fprintf(w, "AGENT %s\t(%s)\t-\t-\n", a.Hostname, online)
			}
		}
		w.Flush()
	},
}

var renewServerCertCmd = &cobra.Command{
	Use:   "server-cert",
	Short: "Guardian sunucu sertifikasını seçilen süreyle yeniden imzalar",
	Run: func(cmd *cobra.Command, args []string) {
		days, _ := cmd.Flags().GetInt("days")
		cert, restartRequired, err := apiClient.RenewServerCert(days)
		if err != nil {
			log.Fatalf("Sunucu sertifikası yenilenemedi: %v", err)
		}
		fmt.Printf("✅ Sunucu sertifikası yenilendi. Yeni bitiş: %s (%d gün)\n", cert.NotAfter, cert.DaysLeft)
		if restartRequired {
			fmt.Println("⚠️  Değişikliğin etkin olması için sunucunun yeniden başlatılması gerekir.")
		}
	},
}

func init() {
	getCmd.AddCommand(getCertificatesCmd)

	var renewCmd = &cobra.Command{Use: "renew", Short: "Bir sertifikayı yeniden imzalar (server-cert)"}
	rootCmd.AddCommand(renewCmd)
	renewCmd.AddCommand(renewServerCertCmd)

	renewServerCertCmd.Flags().Int("days", 365, "Sertifika geçerlilik süresi (gün)")
}
