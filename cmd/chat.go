package cmd

import (
	"github.com/kingjethro999/goo/core"
	"github.com/kingjethro999/goo/memory"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive AI chat session",
	Long:  `Opens a persistent multi-turn chat session with context memory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := memory.NewStore()
		if err != nil {
			return err
		}
		session, err := store.NewSession("chat")
		if err != nil {
			return err
		}
		return core.RunChatSession(session, store)
	},
}
