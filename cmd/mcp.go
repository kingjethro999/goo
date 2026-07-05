package cmd

import (
	"context"
	"fmt"

	"github.com/kingjethro999/goo/config"
	"github.com/kingjethro999/goo/internal/mcp"
	"github.com/spf13/cobra"
)

func toMCPServers(confs []config.MCPServerConf) []mcp.ServerConfig {
	var out []mcp.ServerConfig
	for _, c := range confs {
		kind := mcp.TransportStdio
		if c.Transport == "http" || c.URL != "" {
			kind = mcp.TransportHTTP
		}
		out = append(out, mcp.ServerConfig{
			Name:         c.Name,
			Transport:    kind,
			Command:      c.Command,
			Args:         c.Args,
			Env:          c.Env,
			URL:          c.URL,
			Headers:      c.Headers,
			Enabled:      c.Enabled,
			SafetyPolicy: c.SafetyPolicy,
		})
	}
	return out
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage Model Context Protocol (MCP) servers",
	Long:  `List, inspect, and manage MCP servers configured in Goo.`,
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured MCP servers and available tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		confs := config.GetMCPServers()
		if len(confs) == 0 {
			fmt.Println("No MCP servers configured.")
			return nil
		}
		servers := toMCPServers(confs)

		mgr := mcp.NewManager(nil)
		_ = mgr.LoadFromConfig(ctx, servers)
		defer mgr.CloseAll()

		fmt.Printf("Configured MCP Servers (%d):\n\n", len(servers))
		for _, sc := range servers {
			status := "disconnected/disabled"
			toolCount := 0
			if client, ok := mgr.Client(sc.Name); ok && client.IsConnected() {
				status = "connected"
				toolCount = len(client.Tools())
			}
			fmt.Printf("• %s [%s] (%s) - %d tools\n", sc.Name, sc.Transport, status, toolCount)
			if client, ok := mgr.Client(sc.Name); ok && client.IsConnected() {
				for _, t := range client.Tools() {
					fmt.Printf("    - %s: %s\n", t.Name, t.Description)
				}
			}
		}
		return nil
	},
}

var mcpStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of MCP servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		confs := config.GetMCPServers()
		if len(confs) == 0 {
			fmt.Println("No MCP servers configured.")
			return nil
		}
		servers := toMCPServers(confs)
		mgr := mcp.NewManager(nil)
		_ = mgr.LoadFromConfig(ctx, servers)
		defer mgr.CloseAll()

		fmt.Println("MCP Server Status:")
		for _, sc := range servers {
			client, ok := mgr.Client(sc.Name)
			state := "offline"
			if ok && client.IsConnected() {
				state = "online"
			}
			fmt.Printf("  %-15s : %s\n", sc.Name, state)
		}
		return nil
	},
}

func init() {
	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpStatusCmd)
	rootCmd.AddCommand(mcpCmd)
}
