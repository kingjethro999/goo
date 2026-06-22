package cmd

import (
    "github.com/spf13/cobra"
    "github.com/yourusername/goo/core"
    "github.com/yourusername/goo/memory"
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
