package cmd

import (
	"strings"

	"github.com/kingjethro999/goo/core"
	"github.com/kingjethro999/goo/memory"
	"github.com/spf13/cobra"
)

var askCmd = &cobra.Command{
	Use:   "ask [question]",
	Short: "Ask a question",
	Long:  `Send a single question to the AI and get a response without starting a persistent chat session.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := memory.NewStore()
		if err != nil {
			return err
		}
		question := strings.Join(args, " ")
		return core.RunAskOnce(question, store)
	},
}
