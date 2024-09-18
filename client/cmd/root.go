package cmd

import (
	"os"

	"wh/cli/cmd/config"
	"wh/cli/cmd/tunnel"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cli",
	Short: "Command line interface to work with the wh API",
	Long: `This CLI is responsible to establish the tunnel.

Add the configuration using the URL as config name:
	config add <URL> <APIKEY>

Create a tunnel from an endpoint to a local server:
	tunnel <endpoint> <local_server>.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(config.ConfigCmd)
	rootCmd.AddCommand(tunnel.TunnelCmd)
}
