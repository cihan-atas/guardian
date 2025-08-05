package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"guardian.com/cli/client"
)

var getKeysCmd = &cobra.Command{
	Use:     "keys",
	Aliases: []string{"key"},
	Short:   "Tüm kayıtlı public anahtarları listeler",
	Run: func(cmd *cobra.Command, args []string) {
		keys, err := apiClient.ListKeys()
		if err != nil {
			log.Fatalf("Anahtarlar alınamadı: %v", err)
		}

		if len(keys) == 0 {
			fmt.Println("Kayıtlı public anahtar bulunamadı.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tKEY NAME\tFINGERPRINT (SHA256)")
		fmt.Fprintln(w, "--\t--------\t----------------------")
		for _, key := range keys {
			fmt.Fprintf(w, "%d\t%s\t%s\n", key.ID, key.KeyName, key.FingerprintSHA256)
		}
		w.Flush()
	},
}

var deleteKeyCmd = &cobra.Command{
	Use:   "key [ID]",
	Short: "Belirtilen ID'ye sahip bir public anahtarı siler",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("silinecek anahtarın ID'si gereklidir")
		}
		if _, err := strconv.Atoi(args[0]); err != nil {
			return errors.New("ID sayısal bir değer olmalıdır")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		keyID, _ := strconv.Atoi(args[0])
		err := apiClient.DeleteKey(keyID)
		if err != nil {
			log.Fatalf("Hata: %v", err)
		}
		fmt.Printf("✅ Anahtar ID %d başarıyla silindi.\n", keyID)
	},
}
var createKeyCmd = &cobra.Command{
	Use:   "key",
	Short: "Yeni bir public anahtar kaydeder",
	Run: func(cmd *cobra.Command, args []string) {
		keyName, _ := cmd.Flags().GetString("name")
		filePath, _ := cmd.Flags().GetString("file")

		keyData, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Fatalf("Anahtar dosyası okunamadı '%s': %v", filePath, err)
		}

		payload := client.CreateKeyPayload{
			KeyName:      keyName,
			SshPublicKey: string(keyData),
		}

		newKey, err := apiClient.CreateKey(payload)
		if err != nil {
			log.Fatalf("Hata: %v", err)
		}

		fmt.Printf("✅ Anahtar başarıyla oluşturuldu!\n")
		fmt.Printf("  ID: %d\n  Name: %s\n  Fingerprint: %s\n", newKey.ID, newKey.KeyName, newKey.FingerprintSHA256)
	},
}

var updateKeyCmd = &cobra.Command{
	Use:   "key [ID]",
	Short: "Belirtilen ID'ye sahip bir anahtarın adını günceller",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		keyID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}

		name, _ := cmd.Flags().GetString("name")

		payload := client.UpdateKeyPayload{
			KeyName: name,
		}

		updatedKey, err := apiClient.UpdateKey(keyID, payload)
		if err != nil {
			log.Fatalf("Anahtar güncellenemedi: %v", err)
		}

		fmt.Printf("✅ Anahtar adı başarıyla '%s' olarak güncellendi (ID: %d).\n", updatedKey.KeyName, updatedKey.ID)
	},
}

func init() {
	getCmd.AddCommand(getKeysCmd)
	createCmd.AddCommand(createKeyCmd)
	deleteCmd.AddCommand(deleteKeyCmd)
	updateCmd.AddCommand(updateKeyCmd)

	updateKeyCmd.Flags().String("name", "", "Anahtar için yeni isim (zorunlu)")
	updateKeyCmd.MarkFlagRequired("name")

	createKeyCmd.Flags().String("name", "", "Anahtar için bir isim (zorunlu)")
	createKeyCmd.Flags().String("file", "", "Public anahtar dosyasının yolu (zorunlu)")
	createKeyCmd.MarkFlagRequired("name")
	createKeyCmd.MarkFlagRequired("file")
}
