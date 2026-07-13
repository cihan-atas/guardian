package cmd

import (
	"fmt"
	"log"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Giriş yapılan yönetici hesabının kimliğini gösterir",
	Run: func(cmd *cobra.Command, args []string) {
		me, err := apiClient.GetMe()
		if err != nil {
			log.Fatalf("Kimlik bilgisi alınamadı: %v", err)
		}
		fmt.Printf("Kullanıcı: %s\n", me.Username)
		fmt.Printf("Görünen ad: %s\n", me.DisplayName)
		fmt.Printf("Rol: %s\n", me.Role)
		fmt.Printf("2FA etkin: %t\n", me.TotpEnabled)
	},
}

var changePasswordCmd = &cobra.Command{
	Use:   "change-password",
	Short: "Giriş yapılan yönetici hesabının parolasını değiştirir",
	Run: func(cmd *cobra.Command, args []string) {
		current, _ := cmd.Flags().GetString("current")
		newPass, _ := cmd.Flags().GetString("new")

		// Flag verilmediyse güvenli şekilde gizli giriş iste.
		if current == "" {
			if err := survey.AskOne(&survey.Password{Message: "Mevcut parola:"}, &current); err != nil {
				log.Fatalf("Giriş iptal edildi: %v", err)
			}
		}
		if newPass == "" {
			if err := survey.AskOne(&survey.Password{Message: "Yeni parola:"}, &newPass); err != nil {
				log.Fatalf("Giriş iptal edildi: %v", err)
			}
			var confirm string
			if err := survey.AskOne(&survey.Password{Message: "Yeni parola (tekrar):"}, &confirm); err != nil {
				log.Fatalf("Giriş iptal edildi: %v", err)
			}
			if confirm != newPass {
				log.Fatal("Hata: Girilen yeni parolalar eşleşmiyor.")
			}
		}

		if err := apiClient.ChangePassword(current, newPass); err != nil {
			log.Fatalf("Parola değiştirilemedi: %v", err)
		}
		fmt.Println("✅ Parola başarıyla değiştirildi.")
	},
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
	rootCmd.AddCommand(changePasswordCmd)

	changePasswordCmd.Flags().String("current", "", "Mevcut parola (verilmezse sorulur)")
	changePasswordCmd.Flags().String("new", "", "Yeni parola (verilmezse sorulur)")
}
