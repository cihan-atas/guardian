package cmd

import (
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:     "delete [kaynak] [kaynak-id]",
	Aliases: []string{"rm"},
	Short:   "Bir veya daha fazla kaynağı siler (user, key, rule)",
	Long: `'delete' komutu, Guardian sistemindeki bir kaynağı ID'sini belirterek siler.
    	Örnek:
    	  guardian-cli delete user 2
    	  guardian-cli rm key 3`,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
