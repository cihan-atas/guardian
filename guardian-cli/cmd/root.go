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
		username := os.Getenv("GUARDIAN_ADMIN_USERNAME")
		password := os.Getenv("GUARDIAN_ADMIN_PASSWORD")
		caCertFile := os.Getenv("TLS_CA_FILE")

		if username == "" || password == "" || apiURL == "" || caCertFile == "" {
			log.Fatal("HATA: GUARDIAN_SERVER_HOST, GUARDIAN_SERVER_PORT, GUARDIAN_ADMIN_USERNAME, GUARDIAN_ADMIN_PASSWORD ve TLS_CA_FILE ortam değişkenleri ayarlanmalıdır.")
		}

		var err error
		apiClient, err = client.New(apiURL, caCertFile)
		if err != nil {
			log.Fatalf("API istemcisi oluşturulamadı: %v", err)
		}
		if err := apiClient.Login(username, password); err != nil {
			log.Fatalf("Giriş yapılamadı: %v", err)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Hata: '%s'\n", err)
		os.Exit(1)
	}
}
