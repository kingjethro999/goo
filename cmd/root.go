package cmd

import (
    "github.com/spf13/cobra"
    "github.com/yourusername/goo/config"
)

var (
    version string
    cfgFile string
    debug   bool
    quiet   bool
    raw     bool
)

var rootCmd = &cobra.Command{
    Use:   "goo",
    Short: "Goo — AI CLI assistant for your terminal",
    Long: `Goo is a context-aware AI CLI assistant.
It combines AI chat, task management, GitHub tooling, and web search
in a single terminal application with persistent memory.`,
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        return config.Load(cfgFile)
    },
}

func Execute() error {
    return rootCmd.Execute()
}

func SetVersion(v, c, d string) {
    version = v
    // attach to version command
}

func init() {
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/goo/config.toml)")
    rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug output")
    rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-essential output")
    rootCmd.PersistentFlags().BoolVar(&raw, "raw", false, "raw output, no colours")

    rootCmd.AddCommand(chatCmd)
    rootCmd.AddCommand(askCmd)
    rootCmd.AddCommand(searchCmd)
    rootCmd.AddCommand(taskCmd)
    rootCmd.AddCommand(ghCmd)
    rootCmd.AddCommand(configCmd)
    rootCmd.AddCommand(historyCmd)
    rootCmd.AddCommand(versionCmd)
}
