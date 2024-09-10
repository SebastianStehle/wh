package use

import (
	"fmt"
	"os"

	"wh/cli/config"

	"github.com/spf13/cobra"
)

var UseCmd = &cobra.Command{
	Use:   "use <CONFIG_NAME>",
	Short: "Uses an item from the configuration.",
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
		for _, server := range cfg.Servers {
			if server.Name == name {
				hasConfig = true
				break
			}
		}

		if hasConfig {
			fmt.Printf("Error: Config with this name does not exist.\n")
			os.Exit(1)
			return
		}

		cfg.Server = name

		err = config.StoreConfiguration(cfg)
		if err != nil {
			fmt.Printf("Error: Failed to store configuration. %v\n", err)
			os.Exit(1)
			return
		}

		fmt.Printf("Configuration %s set active.\n", name)
	},
}

func init() {}
