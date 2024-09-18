package add

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"wh/cli/config"

	"github.com/spf13/cobra"
)

var AddCmd = &cobra.Command{
	Use:   "add <ENDPOINT> <API_KEY>",
	Short: "Add a new endpoint to the configuration.",
	Args:  cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		endpoint, err := url.Parse(args[0])
		if err != nil {
			fmt.Printf("Error: Endpoint is not a valid URL.\n")
			os.Exit(1)
			return
		}

		name := strings.ToLower(endpoint.Host)

		nameFlag := cmd.Flag("name")
		if nameFlag != nil {
			value := nameFlag.Value.String()
			if value != "" {
				name = value
			}
		}

		cfg, err := config.GetConfiguration()
		if err != nil {
			fmt.Printf("Error: Failed to retrieve configuration. %v\n", err)
			os.Exit(1)
			return
		}

		for _, server := range cfg.Servers {
			if server.Name == name {
				fmt.Printf("Error: A configuration with this name already exist.\n")
				os.Exit(1)
				return
			}
		}

		cfg.Server = name
		cfg.Servers = append(cfg.Servers, config.Server{
			Name:     name,
			Endpoint: endpoint.String(),
			ApiKey:   args[1],
		})

		err = config.StoreConfiguration(cfg)
		if err != nil {
			fmt.Printf("Error: Failed to store configuration. %v\n", err)
			os.Exit(1)
			return
		}

		fmt.Printf("Added configuration using the name '%s'\n", name)
	},
}

func init() {
	AddCmd.Flags().StringP("name", "n", "", "Adds an optional name for the configuration")
}
