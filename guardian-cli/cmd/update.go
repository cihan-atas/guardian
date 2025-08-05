package cmd

import (
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:     "update [kaynak] [kaynak-id]",
	Aliases: []string{"edit", "patch"},
	Short:   "Var olan bir kaynağı günceller (user, server, key, rule)",
	Long: `update komutu, Guardian sistemindeki bir kaynağı ID'sini belirterek günceller.
    	Örnek:
    	  guardian-cli update user 2 --desc "Yeni açıklama"`,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
