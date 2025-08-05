package cmd

import (
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get [kaynak]",
	Short: "Bir veya daha fazla kaynağı listeler (servers, users, keys, rules)",
	Long: `'get' komutu, Guardian sistemindeki çeşitli kaynakları listelemek için kullanılır.
    	Örnek:
    	  guardian-cli get servers
    	  guardian-cli get users`,
}

func init() {
	rootCmd.AddCommand(getCmd)
}
