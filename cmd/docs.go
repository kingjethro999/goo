package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var docsCmd = &cobra.Command{
	Use:    "docs",
	Short:  "Generate documentation",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "./man"
		os.MkdirAll(dir, 0755)
		header := &doc.GenManHeader{
			Title:   "GOO",
			Section: "1",
			Source:  "Goo AI CLI",
		}
		return doc.GenManTree(rootCmd, header, dir)
	},
}

func init() {
	rootCmd.AddCommand(docsCmd)
}
