package cmd

import (
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create [kaynak]",
	Short: "Yeni bir kaynak oluşturur (key, rule)",
	Long: `'create' komutu, Guardian sistemine yeni bir kaynak ekler.
    	Örnek:
    	  guardian-cli create key --name "my-new-key" --file "/path/to/id_rsa.pub"
    	  guardian-cli create rule --server-id 1 --key-id 1 --user-id 1 --duration 1h`,
}

func init() {
	rootCmd.AddCommand(createCmd)
}
