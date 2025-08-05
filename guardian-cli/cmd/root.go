package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"guardian.com/cli/client"
)

var apiClient *client.Client

var rootCmd = &cobra.Command{
	Use:   "guardian-cli",
	Short: "Guardian, geçici ve denetlenebilir SSH erişimi için bir yönetim aracıdır.",
	Long: `Guardian CLI, Guardian sunucusuyla etkileşim kurarak sunucuları, kullanıcıları, 
    	  anahtarları ve erişim kurallarını yönetmenizi sağlayan bir komut satırı aracıdır.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if cmd.Name() == "version" || cmd.Name() == "help" {
			return
		}

		apiURL := os.Getenv("GUARDIAN_SERVER_HOST") + ":" + os.Getenv("GUARDIAN_SERVER_PORT") + "/api"
		adminToken := os.Getenv("GUARDIAN_ADMIN_TOKEN")
		caCertFile := os.Getenv("TLS_CA_FILE")

		if adminToken == "" || apiURL == "" || caCertFile == "" {
			log.Fatal("HATA: GUARDIAN_SERVER_HOST, GUARDIAN_SERVER_PORT, GUARDIAN_ADMIN_TOKEN ve TLS_CA_FILE ortam değişkenleri ayarlanmalıdır.")
		}

		var err error
		apiClient, err = client.New(apiURL, adminToken, caCertFile)
		if err != nil {
			log.Fatalf("API istemcisi oluşturulamadı: %v", err)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Hata: '%s'\n", err)
		os.Exit(1)
	}
}
