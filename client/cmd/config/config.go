package config

import (
	"wh/cli/cmd/config/add"
	"wh/cli/cmd/config/rm"
	"wh/cli/cmd/config/use"
	"wh/cli/cmd/config/view"

	"github.com/spf13/cobra"
)

var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manages configuration values",
	Long: `Add a new endpoint as a configuration:

Add the configuration using the host name as config name.
	config add <URL> <APIKEY> 

Add the configuration using a specified config name
	config add <URL> <APIKEY> -name <CONFIG_NAME>.

Use a specific configuration:
	config use <CONFIG_NAME>`,
}

func init() {
	ConfigCmd.AddCommand(add.AddCmd)
	ConfigCmd.AddCommand(rm.RmCmd)
	ConfigCmd.AddCommand(use.UseCmd)
	ConfigCmd.AddCommand(view.ViewCmd)
}
