package cmd

import (
	"fmt"
	"strings"

	"github.com/kingjethro999/goo/tools/search"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search the web using Tavily",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		client := search.NewClient()
		resp, err := client.Search(query)
		if err != nil {
			return err
		}
		if resp.Answer != "" {
			fmt.Printf("Summary: %s\n\n", resp.Answer)
		}
		for i, r := range resp.Results {
			fmt.Printf("%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Content)
		}
		return nil
	},
}
