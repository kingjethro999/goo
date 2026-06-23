package cmd

import (
	"fmt"
	"github.com/kingjethro999/goo/core"
	"github.com/kingjethro999/goo/memory"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View chat history",
}

var historyShowCmd = &cobra.Command{
	Use:   "show",
	Short: "List all past sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, _ := memory.NewStore()
		sessions, err := store.ListSessions(50)
		if err != nil {
			return err
		}
		for _, s := range sessions {
			fmt.Printf("%s - %s - %s\n", s.ID[:8], s.StartedAt.Format("Jan 02 15:04"), s.Title)
		}
		return nil
	},
}

var historyResumeCmd = &cobra.Command{
	Use:   "resume [session-id]",
	Short: "Resume a past session with full context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, _ := memory.NewStore()
		session, err := store.GetSession(args[0])
		if err != nil {
			return fmt.Errorf("session not found: %s", args[0])
		}
		return core.RunChatSession(session, store)
	},
}

func init() {
	historyCmd.AddCommand(historyShowCmd)
	historyCmd.AddCommand(historyResumeCmd)
}
