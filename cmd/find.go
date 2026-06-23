package cmd

import (
	"strings"

	"github.com/kingjethro999/goo/core"
	"github.com/spf13/cobra"
)

var findCmd = &cobra.Command{
	Use:   "find [query]",
	Short: "Deep search your system for misplaced files",
	Long:  `Goo Find an extensive search across your system to find anything misplaced, deep or taking your time.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		return core.RunFind(query)
	},
}

func init() {
	rootCmd.AddCommand(findCmd)
}
