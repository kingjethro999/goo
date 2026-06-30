package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kingjethro999/goo/config"
	"github.com/kingjethro999/goo/tools/ai"
	"github.com/kingjethro999/goo/tools/github"
	"github.com/kingjethro999/goo/tools/search"
	"github.com/kingjethro999/goo/tools/tasks"
)

var AllTools = []ai.Tool{
	ai.SearchWebTool,
	{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "run_command",
			Description: "Execute a shell command on the user's machine. Use for installs, builds, file ops, git commands, etc. Always confirm dangerous ops. Returns stdout+stderr.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"command": {"type":"string","description":"The full shell command to run"},
					"cwd":    {"type":"string","description":"Working directory (absolute path). Defaults to home dir."}
				},
				"required": ["command"]
			}`),
		},
	},
	{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "read_file",
			Description: "Read the contents of a file on the user's machine.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type":"string","description":"Absolute or relative path to the file"}
				},
				"required": ["path"]
			}`),
		},
	},
	{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "write_file",
			Description: "Write or overwrite content to a file. Use for code edits, config updates, etc.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path":    {"type":"string","description":"Absolute or relative path to the file"},
					"content": {"type":"string","description":"Full file content to write"}
				},
				"required": ["path","content"]
			}`),
		},
	},
	{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "find_files",
			Description: "Search for files by name pattern or content in a directory. Use glob patterns.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"dir":     {"type":"string","description":"Directory to search (absolute path)"},
					"pattern": {"type":"string","description":"Glob pattern or filename substring, e.g. '*.go' or 'resume'"},
					"content": {"type":"string","description":"Optional: search for this text inside files"}
				},
				"required": ["dir","pattern"]
			}`),
		},
	},
	{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "list_tasks",
			Description: "List the user's current tasks. Use when asked about tasks, todos, reminders.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
	},
	{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "add_task",
			Description: "Add a new task for the user.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"title":    {"type":"string","description":"Task title"},
					"priority": {"type":"string","enum":["low","medium","high","urgent"]},
					"due_date": {"type":"string","description":"ISO 8601 date, optional"}
				},
				"required": ["title"]
			}`),
		},
	},
	{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "complete_task",
			Description: "Mark a task as done by ID.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"task_id": {"type":"integer"}
				},
				"required": ["task_id"]
			}`),
		},
	},
	{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "get_github_prs",
			Description: "Get open pull requests assigned to or created by the user.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
	},
	{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "get_github_stats",
			Description: "Get the user's GitHub contribution stats (repos, commits, PRs, followers).",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"username": {"type":"string","description":"GitHub username. Leave empty to use configured user."}
				}
			}`),
		},
	},
	{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "get_github_repos",
			Description: "List repositories for a GitHub user.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"username": {"type":"string","description":"GitHub username. Leave empty to use configured user."},
					"limit":    {"type":"integer","description":"Max repos to return (default 10)"}
				}
			}`),
		},
	},
}

type ToolDeps struct {
	Tavily *search.Client
	Tasks  *tasks.Manager
	GitHub *github.Client
}

func parseDueDate(d string) *time.Time {
	if d == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, d)
	if err != nil {
		t, _ = time.Parse("2006-01-02", d)
	}
	return &t
}

func formatTasksForAI(t []tasks.Task) string {
	if len(t) == 0 {
		return "No tasks found."
	}
	b, _ := json.MarshalIndent(t, "", "  ")
	return string(b)
}

func formatPRsForAI(prs []github.PullRequest) string {
	if len(prs) == 0 {
		return "No open PRs."
	}
	b, _ := json.MarshalIndent(prs, "", "  ")
	return string(b)
}

func formatStatsForAI(stats *github.ContributionStats) string {
	if stats == nil {
		return "No stats available."
	}
	b, _ := json.MarshalIndent(stats, "", "  ")
	return string(b)
}

// ExecuteToolCall dispatches a tool call and returns its result.
func ExecuteToolCall(call *ai.ToolCall, deps ToolDeps) (string, error) {
	switch call.Name {

	case "search_web":
		var args struct {
			Query string `json:"query"`
		}
		json.Unmarshal(call.Arguments, &args)
		resp, err := deps.Tavily.Search(args.Query)
		if err != nil {
			return "", fmt.Errorf("search request failed: %w", err)
		}
		if resp == nil {
			return "No results.", nil
		}
		b, _ := json.MarshalIndent(resp.Results, "", "  ")
		return string(b), nil

	case "run_command":
		var args struct {
			Command string `json:"command"`
			Cwd     string `json:"cwd"`
		}
		json.Unmarshal(call.Arguments, &args)
		if args.Cwd == "" {
			home, _ := os.UserHomeDir()
			args.Cwd = home
		}
		cmd := exec.Command("bash", "-c", args.Command)
		cmd.Dir = args.Cwd
		out, err := cmd.CombinedOutput()
		result := strings.TrimSpace(string(out))
		if result == "" {
			result = "(no output)"
		}
		if err != nil {
			return fmt.Sprintf("exit error: %v\n%s", err, result), nil
		}
		if len(result) > 4000 {
			result = result[:4000] + "\n… (truncated)"
		}
		return result, nil

	case "read_file":
		var args struct {
			Path string `json:"path"`
		}
		json.Unmarshal(call.Arguments, &args)
		data, err := os.ReadFile(args.Path)
		if err != nil {
			return "", fmt.Errorf("read_file: %w", err)
		}
		content := string(data)
		if len(content) > 8000 {
			content = content[:8000] + "\n… (truncated, file too large)"
		}
		return content, nil

	case "write_file":
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		json.Unmarshal(call.Arguments, &args)
		if err := os.MkdirAll(filepath.Dir(args.Path), 0755); err != nil {
			return "", fmt.Errorf("write_file mkdir: %w", err)
		}
		if err := os.WriteFile(args.Path, []byte(args.Content), 0644); err != nil {
			return "", fmt.Errorf("write_file: %w", err)
		}
		return fmt.Sprintf("Written %d bytes to %s", len(args.Content), args.Path), nil

	case "find_files":
		var args struct {
			Dir     string `json:"dir"`
			Pattern string `json:"pattern"`
			Content string `json:"content"`
		}
		json.Unmarshal(call.Arguments, &args)
		if args.Dir == "" {
			home, _ := os.UserHomeDir()
			args.Dir = home
		}
		// Use find or glob
		var results []string
		err := filepath.Walk(args.Dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			matched, _ := filepath.Match(args.Pattern, info.Name())
			if !matched && !strings.Contains(strings.ToLower(info.Name()), strings.ToLower(args.Pattern)) {
				return nil
			}
			if args.Content != "" {
				data, readErr := os.ReadFile(path)
				if readErr != nil || !strings.Contains(string(data), args.Content) {
					return nil
				}
			}
			results = append(results, path)
			if len(results) >= 50 {
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("find_files: %w", err)
		}
		if len(results) == 0 {
			return "No files found.", nil
		}
		return strings.Join(results, "\n"), nil

	case "list_tasks":
		tasksList, err := deps.Tasks.List(tasks.TaskFilters{Status: "todo"})
		if err != nil {
			return "", err
		}
		return formatTasksForAI(tasksList), nil

	case "add_task":
		var args struct {
			Title    string `json:"title"`
			Priority string `json:"priority"`
			DueDate  string `json:"due_date"`
		}
		json.Unmarshal(call.Arguments, &args)
		task, err := deps.Tasks.Add(args.Title, "", args.Priority, "", nil, parseDueDate(args.DueDate))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Task created: #%d %q", task.ID, task.Title), nil

	case "complete_task":
		var args struct {
			TaskID int `json:"task_id"`
		}
		json.Unmarshal(call.Arguments, &args)
		if err := deps.Tasks.Complete(args.TaskID); err != nil {
			return "", err
		}
		return "Task marked complete", nil

	case "get_github_prs":
		prs, err := deps.GitHub.GetMyPRs("")
		if err != nil {
			return "", err
		}
		return formatPRsForAI(prs), nil

	case "get_github_stats":
		var args struct {
			Username string `json:"username"`
		}
		json.Unmarshal(call.Arguments, &args)
		if args.Username == "" {
			args.Username = config.Get("github.username")
		}
		stats, err := deps.GitHub.GetContributionStats(args.Username)
		if err != nil {
			return "", err
		}
		return formatStatsForAI(stats), nil

	case "get_github_repos":
		var args struct {
			Username string `json:"username"`
			Limit    int    `json:"limit"`
		}
		json.Unmarshal(call.Arguments, &args)
		if args.Username == "" {
			args.Username = config.Get("github.username")
		}
		if args.Limit == 0 {
			args.Limit = 10
		}
		repos, err := deps.GitHub.GetRepos(args.Username, args.Limit)
		if err != nil {
			return "", err
		}
		if len(repos) == 0 {
			return "No public repositories found.", nil
		}
		b, _ := json.MarshalIndent(repos, "", "  ")
		return string(b), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", call.Name)
	}
}
