package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/kingjethro999/goo/config"
	"github.com/kingjethro999/goo/memory"
	"github.com/kingjethro999/goo/tools/ai"
	"github.com/kingjethro999/goo/tools/github"
	"github.com/kingjethro999/goo/tools/search"
	"github.com/kingjethro999/goo/tools/tasks"
)

var AllTools = []ai.Tool{
	ai.SearchWebTool,
	{
		Type: "function",
		Function: &ai.ToolFunction{
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
		Function: &ai.ToolFunction{
			Name:        "read_file",
			Description: "Read the contents of a file on the user's machine. Can optionally read specific line ranges for large files.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path":       {"type":"string","description":"Absolute or relative path to the file"},
					"start_line": {"type":"integer","description":"Optional 1-indexed starting line number to read"},
					"end_line":   {"type":"integer","description":"Optional 1-indexed ending line number to read (inclusive)"}
				},
				"required": ["path"]
			}`),
		},
	},
	{
		Type: "function",
		Function: &ai.ToolFunction{
			Name:        "write_file",
			Description: "Write or edit a file. For existing files, specify old_content to replace a specific part. Can optionally specify start_line and end_line to restrict replacement range. For new files, set create_if_missing to true.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path":              {"type":"string","description":"Absolute or relative path to the file"},
					"old_content":       {"type":"string","description":"Exact substring from the file to replace. Leave empty ONLY for new files."},
					"new_content":       {"type":"string","description":"The new content to write (or replacement content if old_content is provided)."},
					"create_if_missing": {"type":"boolean","description":"Must be true if creating a new file."},
					"start_line":        {"type":"integer","description":"Optional 1-indexed starting line number to restrict where old_content is searched and replaced"},
					"end_line":          {"type":"integer","description":"Optional 1-indexed ending line number to restrict search range"}
				},
				"required": ["path","new_content"]
			}`),
		},
	},
	{
		Type: "function",
		Function: &ai.ToolFunction{
			Name:        "delete_file",
			Description: "Delete a file from the user's machine.",
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
		Function: &ai.ToolFunction{
			Name:        "list_dir",
			Description: "List files and subdirectories inside a directory. Returns names, whether each item is a directory or file, and file sizes.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"dir": {"type":"string","description":"Directory to list (absolute or relative path). Defaults to current directory."}
				}
			}`),
		},
	},
	{
		Type: "function",
		Function: &ai.ToolFunction{
			Name:        "grep_search",
			Description: "Search for exact strings or regex patterns across files in a directory or within a specific file. Returns matching file names, line numbers, and matching line snippets.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query":       {"type":"string","description":"The search pattern or literal string to look for"},
					"path":        {"type":"string","description":"Directory or file path to search in (defaults to current directory)"},
					"is_regex":    {"type":"boolean","description":"Treat query as a regular expression pattern"},
					"ignore_case": {"type":"boolean","description":"Perform case-insensitive search"}
				},
				"required": ["query"]
			}`),
		},
	},
	{
		Type: "function",
		Function: &ai.ToolFunction{
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
		Function: &ai.ToolFunction{
			Name:        "list_tasks",
			Description: "List the user's current tasks. Use when asked about tasks, todos, reminders.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
	},
	{
		Type: "function",
		Function: &ai.ToolFunction{
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
		Function: &ai.ToolFunction{
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
		Function: &ai.ToolFunction{
			Name:        "get_github_prs",
			Description: "Get open pull requests assigned to or created by the user.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
	},
	{
		Type: "function",
		Function: &ai.ToolFunction{
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
		Function: &ai.ToolFunction{
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

		var outBuf bytes.Buffer
		ptyFile, err := pty.Start(cmd)
		if err == nil {
			go func() {
				_, _ = io.Copy(&outBuf, ptyFile)
			}()
			_ = cmd.Wait()
			_ = ptyFile.Close()
		} else {
			out, execErr := cmd.CombinedOutput()
			outBuf.Write(out)
			err = execErr
		}

		result := strings.TrimSpace(outBuf.String())
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
			Path      string `json:"path"`
			StartLine int    `json:"start_line"`
			EndLine   int    `json:"end_line"`
		}
		json.Unmarshal(call.Arguments, &args)
		absPath, err := filepath.Abs(args.Path)
		if err != nil {
			absPath = args.Path
		}
		data, err := os.ReadFile(absPath)
		if err != nil {
			return "", fmt.Errorf("read_file: %w", err)
		}

		// Automated image vision inspection for workspace images
		ext := strings.ToLower(filepath.Ext(absPath))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" || ext == ".gif" {
			fmt.Printf("👁  Analysed workspace image: %s\n", absPath)
			imgExt := strings.TrimPrefix(ext, ".")
			if imgExt == "jpg" {
				imgExt = "jpeg"
			}
			b64 := base64.StdEncoding.EncodeToString(data)
			imageURL := fmt.Sprintf("data:image/%s;base64,%s", imgExt, b64)

			groqClient, err := ai.NewGroqClient()
			if err == nil {
				visionMsg := memory.Message{
					Role:     "user",
					Content:  "Describe this workspace image concisely for an AI coding agent (UI elements, layout, colors, text, diagrams, buttons).",
					ImageURL: imageURL,
				}
				opts := ai.StreamOptions{
					Model:           "meta-llama/llama-4-scout-17b-16e-instruct",
					ReasoningEffort: "low",
				}
				var visBuf strings.Builder
				_, err = groqClient.StreamChatWithToolsEx(context.Background(), []memory.Message{visionMsg}, &visBuf, nil, opts)
				if err == nil && visBuf.Len() > 0 {
					return fmt.Sprintf("[Visual Analysis of image %s]: %s", absPath, strings.TrimSpace(visBuf.String())), nil
				}
			}
			return fmt.Sprintf("[Image file: %s (%d bytes)]", absPath, len(data)), nil
		}

		content := string(data)
		if args.StartLine > 0 || args.EndLine > 0 {
			lines := strings.Split(content, "\n")
			start := args.StartLine
			if start < 1 {
				start = 1
			}
			end := args.EndLine
			if end < start || end > len(lines) {
				end = len(lines)
			}
			var sb strings.Builder
			for i := start - 1; i < end; i++ {
				sb.WriteString(fmt.Sprintf("%d: %s\n", i+1, lines[i]))
			}
			return sb.String(), nil
		}
		if len(content) > 8000 {
			content = content[:8000] + "\n… (truncated, file too large)"
		}
		return content, nil

	case "write_file":
		var args struct {
			Path            string `json:"path"`
			OldContent      string `json:"old_content"`
			NewContent      string `json:"new_content"`
			CreateIfMissing bool   `json:"create_if_missing"`
			Content         string `json:"content"`
			StartLine       int    `json:"start_line"`
			EndLine         int    `json:"end_line"`
		}
		json.Unmarshal(call.Arguments, &args)

		absPath, err := filepath.Abs(args.Path)
		if err != nil {
			return "", fmt.Errorf("invalid path: %w", err)
		}

		if args.OldContent == "" && args.NewContent == "" && args.Content != "" {
			args.NewContent = args.Content
			args.CreateIfMissing = true
		}

		var fileExists bool
		var currentContent string
		if data, err := os.ReadFile(absPath); err == nil {
			currentContent = string(data)
			fileExists = true
		}

		var updatedContent string
		if !fileExists {
			if !args.CreateIfMissing {
				return "", fmt.Errorf("file does not exist and create_if_missing is false")
			}
			if args.OldContent != "" {
				return "", fmt.Errorf("cannot verify old_content since file does not exist")
			}
			updatedContent = args.NewContent
		} else {
			if args.OldContent == "" {
				updatedContent = args.NewContent
			} else {
				count := strings.Count(currentContent, args.OldContent)
				if count == 0 {
					trimmedOld := strings.TrimSpace(args.OldContent)
					if trimmedOld != "" && strings.Contains(currentContent, trimmedOld) {
						return "", fmt.Errorf("old_content not found (exact match failed due to whitespace/indentation differences). Try inspecting exact indentation or reading the file with line ranges first")
					}
					oldLines := strings.Split(trimmedOld, "\n")
					if len(oldLines) > 0 {
						firstLine := strings.TrimSpace(oldLines[0])
						for idx, fLine := range strings.Split(currentContent, "\n") {
							if strings.Contains(fLine, firstLine) {
								return "", fmt.Errorf("old_content not found. However, line %d looks similar (%q). Verify line contents and surrounding context", idx+1, strings.TrimSpace(fLine))
							}
						}
					}
					return "", fmt.Errorf("old_content not found in file. Ensure exact match including whitespace and indentation")
				}
				if count > 1 {
					if args.StartLine > 0 && args.EndLine > 0 {
						lines := strings.Split(currentContent, "\n")
						start := max(1, args.StartLine) - 1
						end := min(len(lines), args.EndLine)
						rangeStr := strings.Join(lines[start:end], "\n")
						if !strings.Contains(rangeStr, args.OldContent) {
							return "", fmt.Errorf("old_content not found within specified line range %d-%d", args.StartLine, args.EndLine)
						}
						replacedRange := strings.Replace(rangeStr, args.OldContent, args.NewContent, 1)
						linesCopy := make([]string, len(lines))
						copy(linesCopy, lines)
						newRangeLines := strings.Split(replacedRange, "\n")
						updatedLines := append(linesCopy[:start], append(newRangeLines, linesCopy[end:]...)...)
						updatedContent = strings.Join(updatedLines, "\n")
					} else {
						return "", fmt.Errorf("old_content matches multiple times (%d) in file. Please specify start_line and end_line or a larger unique surrounding context block", count)
					}
				} else {
					updatedContent = strings.Replace(currentContent, args.OldContent, args.NewContent, 1)
				}
			}
		}

		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return "", fmt.Errorf("write_file mkdir: %w", err)
		}

		tmpFile, err := os.CreateTemp(filepath.Dir(absPath), "goo-atomic-*.tmp")
		if err != nil {
			return "", fmt.Errorf("failed to create temp file: %w", err)
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		if _, err := tmpFile.WriteString(updatedContent); err != nil {
			tmpFile.Close()
			return "", fmt.Errorf("failed to write to temp file: %w", err)
		}
		tmpFile.Close()

		if err := os.Rename(tmpPath, absPath); err != nil {
			return "", fmt.Errorf("atomic write rename failed: %w", err)
		}

		return fmt.Sprintf("Successfully updated file: %s", args.Path), nil

	case "list_dir":
		var args struct {
			Dir string `json:"dir"`
		}
		json.Unmarshal(call.Arguments, &args)
		if args.Dir == "" {
			args.Dir = "."
		}
		absDir, err := filepath.Abs(args.Dir)
		if err != nil {
			absDir = args.Dir
		}
		entries, err := os.ReadDir(absDir)
		if err != nil {
			return "", fmt.Errorf("list_dir: %w", err)
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Directory listing for %s:\n", absDir))
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			if e.IsDir() {
				sb.WriteString(fmt.Sprintf("[DIR]  %-30s\n", e.Name()+"/"))
			} else {
				sb.WriteString(fmt.Sprintf("[FILE] %-30s (%d bytes)\n", e.Name(), info.Size()))
			}
		}
		return sb.String(), nil

	case "grep_search":
		var args struct {
			Query      string `json:"query"`
			Path       string `json:"path"`
			IsRegex    bool   `json:"is_regex"`
			IgnoreCase bool   `json:"ignore_case"`
		}
		json.Unmarshal(call.Arguments, &args)
		if args.Path == "" {
			args.Path = "."
		}
		absPath, err := filepath.Abs(args.Path)
		if err != nil {
			absPath = args.Path
		}

		queryPattern := args.Query
		if !args.IsRegex {
			queryPattern = regexp.QuoteMeta(queryPattern)
		}
		if args.IgnoreCase {
			queryPattern = "(?i)" + queryPattern
		}
		re, err := regexp.Compile(queryPattern)
		if err != nil {
			return "", fmt.Errorf("invalid grep pattern: %w", err)
		}

		var results []string
		matchCount := 0
		err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				if info != nil && info.IsDir() && (info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "dist" || info.Name() == "build") && path != absPath {
					return filepath.SkipDir
				}
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				if re.MatchString(line) {
					rel, relErr := filepath.Rel(absPath, path)
					if relErr != nil {
						rel = path
					}
					results = append(results, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
					matchCount++
					if matchCount >= 100 {
						results = append(results, "… (more than 100 matches found, truncating)")
						return filepath.SkipAll
					}
				}
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("grep_search: %w", err)
		}
		if len(results) == 0 {
			return "No matches found.", nil
		}
		return strings.Join(results, "\n"), nil

	case "delete_file":
		var args struct {
			Path string `json:"path"`
		}
		json.Unmarshal(call.Arguments, &args)
		absPath, err := filepath.Abs(args.Path)
		if err != nil {
			return "", fmt.Errorf("invalid path: %w", err)
		}
		if err := os.Remove(absPath); err != nil {
			return "", fmt.Errorf("delete_file: %w", err)
		}
		return fmt.Sprintf("Successfully deleted file: %s", args.Path), nil

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
		if strings.HasPrefix(call.Name, "mcp_") {
			return GlobalMCPHub.ExecuteToolCall(call.Name, call.Arguments)
		}
		return "", fmt.Errorf("unknown tool: %s", call.Name)
	}
}
