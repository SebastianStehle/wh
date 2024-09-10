package view

import (
	"fmt"
	"os"

	"wh/cli/config"

	"github.com/alexeyco/simpletable"
	"github.com/spf13/cobra"
)

var ViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Lists the configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.GetConfiguration()
		if err != nil {
			fmt.Printf("Error: Failed to retrieve configuration. %v\n", err)
			os.Exit(1)
			return
		}

		table := simpletable.New()

		table.Header = &simpletable.Header{
			Cells: []*simpletable.Cell{
				{Align: simpletable.AlignLeft, Text: "Name"},
				{Align: simpletable.AlignLeft, Text: "Endpoint"},
			},
		}

		for _, server := range cfg.Servers {
			name := server.Name

			if name == cfg.Server {
				name = fmt.Sprintf("> %s", name)
			}

			r := []*simpletable.Cell{
				{Align: simpletable.AlignLeft, Text: name},
				{Align: simpletable.AlignLeft, Text: server.Endpoint},
			}

			table.Body.Cells = append(table.Body.Cells, r)
		}

		table.Footer = &simpletable.Footer{
			Cells: []*simpletable.Cell{
				{Align: simpletable.AlignRight, Span: 2, Text: fmt.Sprintf("Used Configuration: %s", cfg.Server)},
			},
		}

		table.SetStyle(simpletable.StyleCompactLite)
		fmt.Println(table.String())
	},
}
