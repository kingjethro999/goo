import os

files = {
    "cmd/gh.go": """package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var ghCmd = &cobra.Command{
	Use:   "gh",
	Short: "GitHub integration",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("GitHub integration (coming soon)")
	},
}
""",
    "cmd/config.go": """package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Config manager (coming soon)")
	},
}
""",
    "cmd/history.go": """package cmd

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
			fmt.Printf("%s - %s - %s\\n", s.ID[:8], s.StartedAt.Format("Jan 02 15:04"), s.Title)
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
"""
}

for path, content in files.items():
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        f.write(content)
    print(f"Wrote {path}")
