package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"guardian.com/cli/client"
)

var getServersCmd = &cobra.Command{
	Use:     "servers",
	Aliases: []string{"server"},
	Short:   "Tüm kayıtlı sunucuları listeler",
	Long:    `Guardian sistemine kayıtlı olan tüm sunucuların ID, Hostname, IP Adresi ve Açıklama bilgilerini listeler.`,
	Run: func(cmd *cobra.Command, args []string) {
		servers, err := apiClient.ListServers()
		if err != nil {
			log.Fatalf("Sunucular alınamadı: %v", err)
		}

		if len(servers) == 0 {
			fmt.Println("Kayıtlı sunucu bulunamadı.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tHOSTNAME\tIP ADDRESS\tDESCRIPTION")
		fmt.Fprintln(w, "--\t--------\t----------\t-----------")
		for _, server := range servers {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", server.ID, server.Hostname, server.IPAddress, server.Description)
		}
		w.Flush()
	},
}

var createServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Yeni bir sunucu kaydeder",
	Run: func(cmd *cobra.Command, args []string) {
		hostname, _ := cmd.Flags().GetString("hostname")
		ip, _ := cmd.Flags().GetString("ip")
		desc, _ := cmd.Flags().GetString("desc")

		payload := client.CreateServerPayload{
			Hostname:    hostname,
			IPAddress:   ip,
			Description: desc,
		}

		newServer, err := apiClient.CreateServer(payload)
		if err != nil {
			log.Fatalf("Hata: %v", err)
		}

		fmt.Printf("✅ Sunucu '%s' (ID: %d) başarıyla oluşturuldu.\n", newServer.Hostname, newServer.ID)
	},
}

var deleteServerCmd = &cobra.Command{
	Use:   "server [ID]",
	Short: "Belirtilen ID'ye sahip bir sunucuyu siler",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		serverID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}

		err = apiClient.DeleteServer(serverID)
		if err != nil {
			log.Fatalf("Hata: %v", err)
		}

		fmt.Printf("✅ Sunucu ID %d başarıyla silindi.\n", serverID)
	},
}

var updateServerCmd = &cobra.Command{
	Use:   "server [ID]",
	Short: "Belirtilen ID'ye sahip bir sunucuyu günceller",
	Args:  cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !cmd.Flags().Changed("hostname") && !cmd.Flags().Changed("ip") && !cmd.Flags().Changed("desc") {
			return errors.New("güncelleme için en az bir flag belirtilmelidir: --hostname, --ip, --desc")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		serverID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}

		hostname, _ := cmd.Flags().GetString("hostname")
		ip, _ := cmd.Flags().GetString("ip")
		desc, _ := cmd.Flags().GetString("desc")

		payload := client.UpdateServerPayload{
			Hostname:    hostname,
			IPAddress:   ip,
			Description: desc,
		}

		updatedServer, err := apiClient.UpdateServer(serverID, payload)
		if err != nil {
			log.Fatalf("Sunucu güncellenemedi: %v", err)
		}

		fmt.Printf("✅ Sunucu '%s' (ID: %d) başarıyla güncellendi.\n", updatedServer.Hostname, updatedServer.ID)
	},
}

func init() {
	getCmd.AddCommand(getServersCmd)
	createCmd.AddCommand(createServerCmd)
	deleteCmd.AddCommand(deleteServerCmd)
	updateCmd.AddCommand(updateServerCmd)

	createServerCmd.Flags().String("hostname", "", "Sunucunun hostname'i (zorunlu)")
	createServerCmd.Flags().String("ip", "", "Sunucunun IP adresi (zorunlu)")
	createServerCmd.Flags().String("desc", "", "Sunucu için kısa açıklama")
	createServerCmd.MarkFlagRequired("hostname")
	createServerCmd.MarkFlagRequired("ip")

	updateServerCmd.Flags().String("hostname", "", "Sunucu için yeni hostname")
	updateServerCmd.Flags().String("ip", "", "Sunucu için yeni IP adresi")
	updateServerCmd.Flags().String("desc", "", "Sunucu için yeni açıklama")
}
