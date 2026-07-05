package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kingjethro999/goo/config"
	"github.com/kingjethro999/goo/internal/agent"
	"github.com/kingjethro999/goo/internal/mcp"
	"github.com/kingjethro999/goo/internal/orchestrator"
	"github.com/kingjethro999/goo/internal/tools"
	"github.com/spf13/cobra"
)

var (
	tasksFileFlag string
	stopAllFlag   bool
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage and execute agent orchestration",
	Long:  `Run parallel sub-agents, inspect their status, and control execution lifecycles.`,
}

var agentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active and historical sub-agent status",
	RunE: func(cmd *cobra.Command, args []string) error {
		list := orchestrator.GlobalStatus.ListStatus()
		if len(list) == 0 {
			fmt.Println("No active or historical sub-agents found.")
			return nil
		}
		fmt.Printf("%-10s %-10s %-25s %-20s %s\n", "TASK ID", "STATUS", "SCOPE", "SESSION", "ERROR")
		fmt.Println(strings.Repeat("-", 80))
		for _, ts := range list {
			fmt.Printf("%-10s %-10s %-25s %-20s %s\n", ts.TaskID, ts.Status, ts.Scope, ts.SessionID, ts.Error)
		}
		return nil
	},
}

var agentStopCmd = &cobra.Command{
	Use:   "stop [task_id]",
	Short: "Stop a running sub-agent or all sub-agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		if stopAllFlag {
			if err := orchestrator.GlobalStatus.StopAll(); err != nil {
				return err
			}
			fmt.Println("All running sub-agents stopped.")
			return nil
		}
		if len(args) == 0 {
			return fmt.Errorf("must specify task_id or --all flag")
		}
		taskID := args[0]
		if err := orchestrator.GlobalStatus.StopTask(taskID); err != nil {
			return err
		}
		fmt.Printf("Sub-agent %s stopped.\n", taskID)
		return nil
	},
}

type mcpToolProvider struct {
	mgr    *mcp.Manager
	policy agent.SafetyPolicy
}

func (m *mcpToolProvider) ToolsForScope(scope []string) []tools.Tool {
	var names []string
	for name := range m.mgr.Clients() {
		names = append(names, name)
	}
	return mcp.BuildToolList(m.mgr, names, m.policy)
}

var agentRunCmd = &cobra.Command{
	Use:   "run [goal]",
	Short: "Run agent orchestration on a goal or task list",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		goal := strings.Join(args, " ")
		if goal == "" && tasksFileFlag == "" {
			return fmt.Errorf("must provide a goal or specify --tasks file")
		}

		ctx := context.Background()
		confs := config.GetMCPServers()
		servers := toMCPServers(confs)
		mgr := mcp.NewManager(nil)
		_ = mgr.LoadFromConfig(ctx, servers)
		defer mgr.CloseAll()

		policyMode := config.Get("agent.safety_policy")
		allowlist := config.GetAgentAllowlist()
		policy := agent.NewStandardPolicy(policyMode, allowlist)

		var names []string
		for name := range mgr.Clients() {
			names = append(names, name)
		}
		leadTools := mcp.BuildToolList(mgr, names, policy)

		lead := agent.NewCore(agent.CoreConfig{
			Tools:  leadTools,
			Policy: policy,
		})

		var tasks []orchestrator.Task
		var err error

		if tasksFileFlag != "" {
			tasks, err = orchestrator.LoadTasksFile(tasksFileFlag)
			if err != nil {
				return fmt.Errorf("failed to load tasks file: %w", err)
			}
		} else {
			var files []string
			_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && !strings.HasPrefix(path, ".") {
					files = append(files, path)
				}
				return nil
			})
			fmt.Println("Decomposing goal into parallel tasks...")
			tasks, err = orchestrator.Decompose(ctx, goal, files)
			if err != nil {
				return fmt.Errorf("task decomposition failed: %w", err)
			}
		}

		fmt.Printf("Starting parallel execution of %d tasks...\n", len(tasks))
		maxParallel := orchestrator.GetMaxParallelAgents()
		tp := &mcpToolProvider{mgr: mgr, policy: policy}
		orch := orchestrator.New(lead, maxParallel, policy, tp)

		runCtx, _ := orchestrator.GlobalStatus.StartRun(ctx, "", tasks)
		results := orch.RunParallel(runCtx, tasks)

		report := orchestrator.Merge(results)
		fmt.Println("\n" + report.Summary)
		return nil
	},
}

func init() {
	agentStopCmd.Flags().BoolVar(&stopAllFlag, "all", false, "Stop all running sub-agents")
	agentRunCmd.Flags().StringVar(&tasksFileFlag, "tasks", "", "Path to tasks.yaml or JSON task list")

	agentCmd.AddCommand(agentStatusCmd)
	agentCmd.AddCommand(agentStopCmd)
	agentCmd.AddCommand(agentRunCmd)
	rootCmd.AddCommand(agentCmd)
}
