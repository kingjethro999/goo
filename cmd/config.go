package cmd

import (
	"fmt"

	"github.com/kingjethro999/goo/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var setKeyCmd = &cobra.Command{
	Use:   "set-key [provider]",
	Short: "Set an API key for a provider",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := args[0]
		fmt.Printf("Enter API key for %s: ", provider)
		var key string
		fmt.Scanln(&key)
		if key == "" {
			return fmt.Errorf("key cannot be empty")
		}
		if err := config.SetAPIKey(provider, key); err != nil {
			return err
		}
		fmt.Printf("Successfully saved key for %s\n", provider)
		return nil
	},
}

func init() {
	configCmd.AddCommand(setKeyCmd)
}
