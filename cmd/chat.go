package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/kingjethro999/goo/core"
	"github.com/kingjethro999/goo/memory"
	"github.com/spf13/cobra"
)

var (
	chatDir   string
	chatUndo  bool
	chatImage string
)

var chatCmd = &cobra.Command{
	Use:   "chat [instruction]",
	Short: "Start an AI agent coding session",
	Long:  `Opens an agent coding session scoped to a directory, capable of writing files and running shell commands.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if chatUndo {
			return core.UndoLastSession()
		}

		targetDir := chatDir
		if targetDir == "" {
			var err error
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		store, err := memory.NewStore()
		if err != nil {
			return err
		}
		
		session, err := store.NewSession("agent")
		if err != nil {
			return err
		}

		initialInstruction := strings.Join(args, " ")
		return core.RunAgentSession(session, store, initialInstruction, targetDir, chatImage)
	},
}

func init() {
	chatCmd.Flags().StringVarP(&chatDir, "dir", "d", "", "Scope session to this directory")
	chatCmd.Flags().BoolVar(&chatUndo, "undo", false, "Undo the last agent action session")
	chatCmd.Flags().StringVarP(&chatImage, "image", "i", "", "Attach image for visual debugging (local path or URL)")
}
