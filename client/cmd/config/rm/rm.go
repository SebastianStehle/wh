package rm

import (
	"fmt"
	"os"
	"slices"

	"wh/cli/config"

	"github.com/spf13/cobra"
)

var RmCmd = &cobra.Command{
	Use:   "rm <CONFIG_NAME>",
	Short: "Removes an item from the configuration.",
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		cfg, err := config.GetConfiguration()
		if err != nil {
			fmt.Printf("Error: Failed to retrieve configuration. %v\n", err)
			os.Exit(1)
			return
		}

		hasConfig := false
		for i, server := range cfg.Servers {
			if server.Name == name {
				cfg.Servers = slices.Delete(cfg.Servers, i, i+1)
				hasConfig = true
				break
			}
		}

		if !hasConfig {
			fmt.Printf("Error: Config with this name '%s' does not exist.\n", name)
			os.Exit(1)
			return
		}

		err = config.StoreConfiguration(cfg)
		if err != nil {
			fmt.Printf("Error: Failed to store configuration. %v\n", err)
			os.Exit(1)
			return
		}

		fmt.Printf("Configuration %s removed.\n", name)
	},
}

func init() {}
