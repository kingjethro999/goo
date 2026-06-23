package core

import (
	"encoding/json"
	"fmt"
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
			Name:        "list_tasks",
			Description: "List the user's current tasks. Use when asked about tasks, todos, reminders, or what's pending.",
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
			Description: "Get the user's GitHub contribution stats including commit streak.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
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

func formatSearchForAI(res *search.SearchResponse) string {
	if res == nil {
		return "No results."
	}
	b, _ := json.MarshalIndent(res.Results, "", "  ")
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

func ExecuteToolCall(call *ai.ToolCall, deps ToolDeps) (string, error) {
	switch call.Name {
	case "search_web":
		var args struct {
			Query string `json:"query"`
		}
		json.Unmarshal(call.Arguments, &args)
		resp, err := deps.Tavily.Search(args.Query)
		if err != nil {
			return "", err
		}
		return formatSearchForAI(resp), nil

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
		err := deps.Tasks.Complete(args.TaskID)
		if err != nil {
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
		stats, err := deps.GitHub.GetContributionStats(config.Get("github.username"))
		if err != nil {
			return "", err
		}
		return formatStatsForAI(stats), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", call.Name)
	}
}
