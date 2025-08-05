// guardian-cli/cmd/rules.go

package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"guardian.com/cli/client"
)

var getRulesCmd = &cobra.Command{
	Use:     "rules",
	Aliases: []string{"rule"},
	Short:   "Tüm erişim kurallarını listeler",
	Run: func(cmd *cobra.Command, args []string) {
		rules, err := apiClient.ListRules()
		if err != nil {
			log.Fatalf("Kurallar alınamadı: %v", err)
		}

		if len(rules) == 0 {
			fmt.Println("Kayıtlı kural bulunamadı.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tSERVER\tUSER\tKEY NAME\tVALID UNTIL")
		fmt.Fprintln(w, "--\t------\t------\t----\t--------\t-----------")
		for _, rule := range rules {
			validUntilFormatted := rule.ValidUntil.Format(time.RFC822)
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n", rule.ID, rule.Status, rule.ServerHostname, rule.Username, rule.KeyName, validUntilFormatted)
		}
		w.Flush()
	},
}

var createRuleCmd = &cobra.Command{
	Use:   "rule",
	Short: "Yeni bir erişim kuralı oluşturur (interaktif veya flag'lerle)",
	Long: `Yeni bir erişim kuralı oluşturur. 
Eğer --server-id, --user-id ve --key-id flag'leri verilmezse, interaktif seçim menüsü başlar.
Zamanlama için --duration VEYA --valid-from ile --valid-until flag'leri kullanılabilir.

Örnekler:
  guardian-cli create rule --server-id 1 --user-id 1 --key-id 1 --duration 2h
  guardian-cli create rule --server-id 1 --user-id 1 --key-id 1 --valid-from "2024-08-01 14:00" --valid-until "2024-08-01 16:00"
  guardian-cli create rule (interaktif modu başlatır)`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		duration, _ := cmd.Flags().GetString("set-duration")
		validFrom, _ := cmd.Flags().GetString("set-valid-from")
		validUntil, _ := cmd.Flags().GetString("set-valid-until")

		if duration != "1h" && validFrom != "" {
			return errors.New("--duration flag'i, --valid-from ve --valid-until flag'leri ile birlikte kullanılamaz")
		}
		if (validFrom != "" && validUntil == "") || (validFrom == "" && validUntil != "") {
			return errors.New("--valid-from ve --valid-until flag'leri birlikte kullanılmalıdır")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		serverID, _ := cmd.Flags().GetInt("server-id")
		userID, _ := cmd.Flags().GetInt("user-id")
		keyID, _ := cmd.Flags().GetInt("key-id")
		durationStr, _ := cmd.Flags().GetString("duration")
		validFromStr, _ := cmd.Flags().GetString("valid-from")
		validUntilStr, _ := cmd.Flags().GetString("valid-until")

		var err error

		if serverID == 0 || userID == 0 || keyID == 0 {
			fmt.Println("ℹ️ Gerekli ID'ler verilmedi, interaktif mod başlatılıyor...")
			if serverID == 0 {
				serverID, err = selectServer()
				if err != nil {
					log.Fatalf("Sunucu seçimi başarısız: %v", err)
				}
			}
			if userID == 0 {
				userID, err = selectUser()
				if err != nil {
					log.Fatalf("Kullanıcı seçimi başarısız: %v", err)
				}
			}
			if keyID == 0 {
				keyID, err = selectKey()
				if err != nil {
					log.Fatalf("Anahtar seçimi başarısız: %v", err)
				}
			}
		}

		var validFrom, validUntil time.Time
		timeFormat := "2006-01-02 15:04"

		if validFromStr != "" {
			validFrom, err = time.ParseInLocation(timeFormat, validFromStr, time.Local)
			if err != nil {
				log.Fatalf("Geçersiz başlangıç tarihi formatı: %v", err)
			}
			validUntil, err = time.ParseInLocation(timeFormat, validUntilStr, time.Local)
			if err != nil {
				log.Fatalf("Geçersiz bitiş tarihi formatı: %v", err)
			}
		} else {
			duration, err := time.ParseDuration(durationStr)
			if err != nil {
				log.Fatalf("Geçersiz süre formatı: %v", err)
			}
			validFrom = time.Now()
			validUntil = validFrom.Add(duration)
		}

		payload := client.CreateRulePayload{
			ServerID: serverID, PublicKeyID: keyID, SystemUserID: userID,
			ValidFrom: validFrom, ValidUntil: validUntil,
		}
		newRule, err := apiClient.CreateRule(payload)
		if err != nil {
			log.Fatalf("Hata: %v", err)
		}
		fmt.Printf("✅ Kural (ID: %d) başarıyla oluşturuldu.\n", newRule.ID)
		fmt.Printf("   Bu kural %s tarihine kadar geçerlidir.\n", newRule.ValidUntil.Format(time.RFC1123))
	},
}

var deleteRuleCmd = &cobra.Command{
	Use: "rule [ID]", Short: "Belirtilen ID'ye sahip bir erişim kuralını siler", Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ruleID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}
		err = apiClient.DeleteRule(ruleID)
		if err != nil {
			log.Fatalf("Hata: %v", err)
		}
		fmt.Printf("✅ Kural ID %d başarıyla silindi.\n", ruleID)
	},
}

var updateRuleCmd = &cobra.Command{
	Use: "rule [ID]", Short: "Belirtilen ID'ye sahip bir kuralın zamanını günceller",
	Long: `Bir erişim kuralının geçerlilik süresini günceller.
   Zamanlama için --duration VEYA --valid-from ile --valid-until flag'leri kullanılabilir.
   Eğer --duration kullanılırsa, kuralın başlangıç zamanı ŞİMDİ olarak ayarlanır.
   
   Örnekler:
     guardian-cli update rule 5 --duration 30m (Kuralı şimdiden 30dk sonrası için yeniden ayarlar)
     guardian-cli update rule 5 --valid-from "2024-08-01 18:00" --valid-until "2024-08-01 19:00"`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		durationFlag := cmd.Flags().Changed("duration")
		validFrom, _ := cmd.Flags().GetString("valid-from")
		validUntil, _ := cmd.Flags().GetString("valid-until")

		if durationFlag && validFrom != "" {
			return errors.New("--duration flag'i, --valid-from ve --valid-until flag'leri ile birlikte kullanılamaz")
		}
		if (validFrom != "" && validUntil == "") || (validFrom == "" && validUntil != "") {
			return errors.New("--valid-from ve --valid-until flag'leri birlikte kullanılmalıdır")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		ruleID, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatalf("Hata: ID sayısal bir değer olmalıdır: %v", err)
		}

		durationStr, _ := cmd.Flags().GetString("set-duration")
		validFromStr, _ := cmd.Flags().GetString("set-valid-from")
		validUntilStr, _ := cmd.Flags().GetString("set-valid-until")

		var validFrom, validUntil time.Time
		timeFormat := "2006-01-02 15:04"

		if validFromStr != "" {
			validFrom, err = time.ParseInLocation(timeFormat, validFromStr, time.Local)
			if err != nil {
				log.Fatalf("Geçersiz başlangıç tarihi formatı: %v", err)
			}
			validUntil, err = time.ParseInLocation(timeFormat, validUntilStr, time.Local)
			if err != nil {
				log.Fatalf("Geçersiz bitiş tarihi formatı: %v", err)
			}
		} else {
			duration, err := time.ParseDuration(durationStr)
			if err != nil {
				log.Fatalf("Geçersiz süre formatı: %v", err)
			}
			validFrom = time.Now()
			validUntil = validFrom.Add(duration)
		}

		payload := client.UpdateRulePayload{
			ValidFrom: validFrom, ValidUntil: validUntil,
		}
		updatedRule, err := apiClient.UpdateRule(ruleID, payload)
		if err != nil {
			log.Fatalf("Kural güncellenemedi: %v", err)
		}
		fmt.Printf("✅ Kural (ID: %d) başarıyla güncellendi.\n", updatedRule.ID)
		fmt.Printf("   Yeni durum: '%s', Yeni geçerlilik sonu: %s\n", updatedRule.Status, updatedRule.ValidUntil.Format(time.RFC1123))
	},
}

func selectServer() (int, error) {
	servers, err := apiClient.ListServers()
	if err != nil {
		return 0, err
	}
	if len(servers) == 0 {
		return 0, errors.New("kayıtlı sunucu bulunamadı. Lütfen önce 'guardian-cli create server' ile bir sunucu ekleyin")
	}

	options := make([]string, len(servers))
	for i, s := range servers {
		options[i] = fmt.Sprintf("%s (%s)", s.Hostname, s.IPAddress)
	}

	var selectedIndex int
	prompt := &survey.Select{
		Message: "Bir sunucu seçin:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selectedIndex); err != nil {
		return 0, err
	}
	return servers[selectedIndex].ID, nil
}

func selectUser() (int, error) {
	users, err := apiClient.ListUsers()
	if err != nil {
		return 0, err
	}
	if len(users) == 0 {
		return 0, errors.New("kayıtlı kullanıcı bulunamadı. Lütfen önce 'guardian-cli create user' ile bir kullanıcı ekleyin")
	}

	options := make([]string, len(users))
	for i, u := range users {

		if u.Description.Valid && u.Description.String != "" {
			options[i] = fmt.Sprintf("%s (%s)", u.Username, u.Description.String)
		} else {
			options[i] = u.Username
		}

	}

	var selectedIndex int
	prompt := &survey.Select{
		Message: "Bir sistem kullanıcısı seçin:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selectedIndex); err != nil {
		return 0, err
	}
	return users[selectedIndex].ID, nil
}

func selectKey() (int, error) {
	keys, err := apiClient.ListKeys()
	if err != nil {
		return 0, err
	}
	if len(keys) == 0 {
		return 0, errors.New("kayıtlı SSH anahtarı bulunamadı. Lütfen önce 'guardian-cli create key' ile bir anahtar ekleyin")
	}

	options := make([]string, len(keys))
	for i, k := range keys {
		options[i] = fmt.Sprintf("%s - %s", k.KeyName, k.FingerprintSHA256)
	}

	var selectedIndex int
	prompt := &survey.Select{
		Message: "Kullanılacak bir SSH anahtarı seçin:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selectedIndex); err != nil {
		return 0, err
	}
	return keys[selectedIndex].ID, nil
}
func init() {
	getCmd.AddCommand(getRulesCmd)
	createCmd.AddCommand(createRuleCmd)
	deleteCmd.AddCommand(deleteRuleCmd)
	updateCmd.AddCommand(updateRuleCmd)

	createRuleCmd.Flags().Int("server-id", 0, "Hedef sunucunun ID'si")
	createRuleCmd.Flags().Int("user-id", 0, "Bağlanılacak sistem kullanıcısının ID'si")
	createRuleCmd.Flags().Int("key-id", 0, "Kullanılacak public anahtarın ID'si")
	createRuleCmd.Flags().String("duration", "1h", "Kuralın geçerlilik süresi (örn: 30m, 2h)")
	createRuleCmd.Flags().String("valid-from", "", "Kuralın başlangıç zamanı ('YYYY-AA-GG SS:DD' formatında)")
	createRuleCmd.Flags().String("valid-until", "", "Kuralın bitiş zamanı ('YYYY-AA-GG SS:DD' formatında)")

	updateRuleCmd.Flags().String("set-duration", "", "Kuralın şimdiden itibaren yeni geçerlilik süresi")
	updateRuleCmd.Flags().String("set-valid-from", "", "Kuralın yeni başlangıç zamanı")
	updateRuleCmd.Flags().String("set-valid-until", "", "Kuralın yeni bitiş zamanı")
}
