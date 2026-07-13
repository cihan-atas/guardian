package cmd

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"guardian.com/cli/client"
)

var getAdminsCmd = &cobra.Command{
	Use:     "admins",
	Aliases: []string{"admin", "admin-users"},
	Short:   "Tüm yönetici hesaplarını (RBAC) listeler",
	Run: func(cmd *cobra.Command, args []string) {
		admins, err := apiClient.ListAdminUsers()
		if err != nil {
			log.Fatalf("Yönetici hesapları alınamadı: %v", err)
		}
		if len(admins) == 0 {
			fmt.Println("Kayıtlı yönetici hesabı bulunamadı.")
			return
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tUSERNAME\tROLE\tDISPLAY NAME\tDISABLED\tLAST LOGIN")
		fmt.Fprintln(w, "--\t--------\t----\t------------\t--------\t----------")
		for _, a := range admins {
			lastLogin := "-"
			if a.LastLogin != nil {
				lastLogin = a.LastLogin.Format(time.RFC822)
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%t\t%s\n", a.ID, a.Username, a.Role, a.DisplayName, a.Disabled, lastLogin)
		}
		w.Flush()
	},
}

var createAdminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Yeni bir yönetici hesabı (RBAC) oluşturur",
	Run: func(cmd *cobra.Command, args []string) {
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		role, _ := cmd.Flags().GetString("role")
		displayName, _ := cmd.Flags().GetString("display-name")

		payload := client.CreateAdminPayload{
			Username:    username,
			Password:    password,
			Role:        role,
			DisplayName: displayName,
		}
		newAdmin, err := apiClient.CreateAdminUser(payload)
		if err != nil {
			log.Fatalf("Hata: %v", err)
		}
		fmt.Printf("✅ Yönetici '%s' (ID: %d, rol: %s) başarıyla oluşturuldu.\n", newAdmin.Username, newAdmin.ID, newAdmin.Role)
	},
}

var updateAdminCmd = &cobra.Command{
	Use:   "admin [ID]",
	Short: "Bir yönetici hesabını günceller (rol, ad, parola, devre dışı)",
	Args:  cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !cmd.Flags().Changed("role") && !cmd.Flags().Changed("display-name") &&
			!cmd.Flags().Changed("password") && !cmd.Flags().Changed("disabled") {
			return fmt.Errorf("güncelleme için en az bir flag belirtilmelidir: --role, --display-name, --password, --disabled")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		adminID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}

		var payload client.UpdateAdminPayload
		if cmd.Flags().Changed("role") {
			payload.Role, _ = cmd.Flags().GetString("role")
		}
		if cmd.Flags().Changed("display-name") {
			payload.DisplayName, _ = cmd.Flags().GetString("display-name")
		}
		if cmd.Flags().Changed("password") {
			payload.Password, _ = cmd.Flags().GetString("password")
		}
		if cmd.Flags().Changed("disabled") {
			disabled, _ := cmd.Flags().GetBool("disabled")
			payload.Disabled = &disabled
		}

		if err := apiClient.UpdateAdminUser(adminID, payload); err != nil {
			log.Fatalf("Yönetici güncellenemedi: %v", err)
		}
		fmt.Printf("✅ Yönetici hesabı (ID: %d) başarıyla güncellendi.\n", adminID)
	},
}

var deleteAdminCmd = &cobra.Command{
	Use:   "admin [ID]",
	Short: "Bir yönetici hesabını siler",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		adminID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}
		if err := apiClient.DeleteAdminUser(adminID); err != nil {
			log.Fatalf("Hata: %v", err)
		}
		fmt.Printf("✅ Yönetici hesabı ID %d başarıyla silindi.\n", adminID)
	},
}

func init() {
	getCmd.AddCommand(getAdminsCmd)
	createCmd.AddCommand(createAdminCmd)
	updateCmd.AddCommand(updateAdminCmd)
	deleteCmd.AddCommand(deleteAdminCmd)

	createAdminCmd.Flags().String("username", "", "Yönetici kullanıcı adı (zorunlu)")
	createAdminCmd.Flags().String("password", "", "Yönetici parolası (zorunlu)")
	createAdminCmd.Flags().String("role", "viewer", "Rol: viewer | operator | admin")
	createAdminCmd.Flags().String("display-name", "", "Görünen ad")
	createAdminCmd.MarkFlagRequired("username")
	createAdminCmd.MarkFlagRequired("password")

	updateAdminCmd.Flags().String("role", "", "Yeni rol: viewer | operator | admin")
	updateAdminCmd.Flags().String("display-name", "", "Yeni görünen ad")
	updateAdminCmd.Flags().String("password", "", "Yeni parola")
	updateAdminCmd.Flags().Bool("disabled", false, "Hesabı devre dışı bırak (true) / etkinleştir (false)")
}
