package cmd

import (
	"fmt"
	"strings"

	"github.com/kingjethro999/goo/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var setKeyCmd = &cobra.Command{
	Use:   "set-key [provider]",
	Short: "Set an API key for a provider (groq, openai, claude, deepseek, github, tavily, …)",
	Long: `Store an API key securely (encrypted, machine-local).

Supported providers:
  groq       — default AI (llama-3.3-70b, mixtral, gemma)
  openai     — GPT-4o, GPT-4o-mini
  claude     — Claude 3.5 Sonnet (Anthropic)
  deepseek   — DeepSeek Chat
  github     — GitHub Personal Access Token (PAT)
  tavily     — Tavily web search

Examples:
  goo config set-key groq
  goo config set-key openai
  goo config set-key github
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := strings.ToLower(args[0])
		fmt.Printf("Enter API key for %s: ", provider)
		var key string
		fmt.Scanln(&key)
		if key == "" {
			return fmt.Errorf("key cannot be empty")
		}
		if err := config.SetAPIKey(provider, key); err != nil {
			return err
		}
		fmt.Printf("✓ Saved key for %s\n", provider)
		return nil
	},
}

var listKeysCmd = &cobra.Command{
	Use:   "list-keys",
	Short: "List all stored API key slots",
	RunE: func(cmd *cobra.Command, args []string) error {
		slots := config.ListSlots()
		if len(slots) == 0 {
			fmt.Println("No API keys stored. Use: goo config set-key <provider>")
			return nil
		}
		fmt.Println("Stored API keys:")
		for _, s := range slots {
			fmt.Printf("  • %s\n", s)
		}
		return nil
	},
}

var setProviderCmd = &cobra.Command{
	Use:   "set-provider [provider]",
	Short: "Switch AI provider (groq, openai, claude, deepseek)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := strings.ToLower(args[0])
		valid := map[string]bool{"groq": true, "openai": true, "claude": true, "deepseek": true}
		if !valid[provider] {
			return fmt.Errorf("unknown provider: %s (valid: groq, openai, claude, deepseek)", provider)
		}
		if err := config.Set("general.default_provider", provider); err != nil {
			return err
		}
		fmt.Printf("✓ Switched AI provider to: %s\n", provider)
		return nil
	},
}

func init() {
	configCmd.AddCommand(setKeyCmd)
	configCmd.AddCommand(listKeysCmd)
	configCmd.AddCommand(setProviderCmd)
}
