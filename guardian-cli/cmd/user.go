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

var getUsersCmd = &cobra.Command{
	Use:     "users",
	Aliases: []string{"user"},
	Short:   "Tüm kayıtlı sistem kullanıcılarını listeler",
	Long:    `Guardian sistemine kayıtlı olan tüm sistem kullanıcılarını (örn: root, ec2-user) listeler.`,
	Run: func(cmd *cobra.Command, args []string) {
		users, err := apiClient.ListUsers()
		if err != nil {
			log.Fatalf("Kullanıcılar alınamadı: %v", err)
		}
		if len(users) == 0 {
			fmt.Println("Kayıtlı kullanıcı bulunamadı.")
			return
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tUSERNAME\tDESCRIPTION")
		fmt.Fprintln(w, "--\t--------\t-----------")
		for _, user := range users {
			// DEĞİŞİKLİK: Description'ın geçerli olup olmadığını kontrol et.
			// Eğer geçerliyse (Valid=true) içindeki string'i yaz, değilse "-" yaz.
			description := "-"
			if user.Description.Valid {
				description = user.Description.String
			}
			fmt.Fprintf(w, "%d\t%s\t%s\n", user.ID, user.Username, description)
		}
		w.Flush()
	},
}

var createUserCmd = &cobra.Command{
	Use:   "user",
	Short: "Yeni bir sistem kullanıcısı tanımlar",
	Run: func(cmd *cobra.Command, args []string) {
		username, _ := cmd.Flags().GetString("username")
		desc, _ := cmd.Flags().GetString("desc")

		// DEĞİŞİKLİK: Payload'ı backend'in beklediği NullString yapısına uygun hale getir.
		payload := client.CreateUserPayload{
			Username: username,
			Description: client.NullString{
				String: desc,
				Valid:  desc != "", // Eğer kullanıcı --desc flag'i ile bir değer girdiyse Valid=true olur.
			},
		}

		newUser, err := apiClient.CreateUser(payload)
		if err != nil {
			log.Fatalf("Hata: %v", err)
		}
		fmt.Printf("✅ Kullanıcı '%s' (ID: %d) başarıyla oluşturuldu.\n", newUser.Username, newUser.ID)
	},
}

var deleteUserCmd = &cobra.Command{
	Use:   "user [ID]",
	Short: "Belirtilen ID'ye sahip bir kullanıcıyı siler",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("silinecek kullanıcının ID'si gereklidir")
		}
		if _, err := strconv.Atoi(args[0]); err != nil {
			return errors.New("ID sayısal bir değer olmalıdır")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		userID, _ := strconv.Atoi(args[0])
		err := apiClient.DeleteUser(userID)
		if err != nil {
			log.Fatalf("Hata: %v", err)
		}
		fmt.Printf("✅ Kullanıcı ID %d başarıyla silindi.\n", userID)
	},
}

var updateUserCmd = &cobra.Command{
	Use:   "user [ID]",
	Short: "Belirtilen ID'ye sahip bir kullanıcının açıklamasını günceller",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		userID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}

		desc, _ := cmd.Flags().GetString("desc")

		payload := client.UpdateUserPayload{
			Description: client.NullString{
				String: desc,
				Valid:  true, // PATCH işleminde boş string göndermek de bir güncellemedir.
			},
		}

		updatedUser, err := apiClient.UpdateUser(userID, payload)
		if err != nil {
			log.Fatalf("Kullanıcı güncellenemedi: %v", err)
		}

		fmt.Printf("✅ Kullanıcı '%s' (ID: %d) başarıyla güncellendi.\n", updatedUser.Username, updatedUser.ID)
	},
}

func init() {
	getCmd.AddCommand(getUsersCmd)
	createCmd.AddCommand(createUserCmd)
	deleteCmd.AddCommand(deleteUserCmd)
	updateCmd.AddCommand(updateUserCmd)

	updateUserCmd.Flags().String("desc", "", "Kullanıcı için yeni açıklama (zorunlu)")
	updateUserCmd.MarkFlagRequired("desc")
	createUserCmd.Flags().String("username", "", "Sistem kullanıcı adı (zorunlu)")
	createUserCmd.Flags().String("desc", "", "Kullanıcı için kısa açıklama")
	createUserCmd.MarkFlagRequired("username")
}
