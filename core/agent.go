package core

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kingjethro999/goo/config"
	"github.com/kingjethro999/goo/memory"
	"github.com/kingjethro999/goo/tools/ai"
	"github.com/kingjethro999/goo/tools/github"
	"github.com/kingjethro999/goo/tools/search"
	"github.com/kingjethro999/goo/tools/tasks"
)

type StepKind string

const (
	StepFileRead   StepKind = "FileRead"
	StepFileWrite  StepKind = "FileWrite"
	StepFileDelete StepKind = "FileDelete"
	StepShellExec  StepKind = "ShellExec"
	StepSearch     StepKind = "Search"
)

type RiskTier string

const (
	TierSafe     RiskTier = "SAFE"
	TierLow      RiskTier = "LOW"
	TierModerate RiskTier = "MODERATE"
	TierHigh     RiskTier = "HIGH"
)

type Step struct {
	ID          string   `json:"id"`
	Kind        StepKind `json:"kind"`
	Description string   `json:"description"`
	Payload     any      `json:"payload"`
	RiskTier    RiskTier `json:"risk_tier"`
	DependsOn   []string `json:"depends_on,omitempty"`
}

type StepResult struct {
	StepID   string        `json:"step_id"`
	Success  bool          `json:"success"`
	Output   string        `json:"output"`
	ErrorMsg string        `json:"error_msg,omitempty"`
	Duration time.Duration `json:"duration"`
}

type StepEntry struct {
	Timestamp     time.Time `json:"timestamp"`
	Kind          string    `json:"kind"`
	Description   string    `json:"description"`
	Path          string    `json:"path,omitempty"`
	PrevContent   string    `json:"prev_content,omitempty"`
	Command       string    `json:"command,omitempty"`
	GitCheckpoint string    `json:"git_checkpoint,omitempty"`
	Undone        bool      `json:"undone"`
}

func getSessionLogPath(sessionID string) string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "goo", "sessions")
	_ = os.MkdirAll(dir, 0700)
	return filepath.Join(dir, sessionID+".json")
}

func loadSessionLog(sessionID string) (*SessionLog, error) {
	path := getSessionLogPath(sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SessionLog{SessionID: sessionID}, nil
		}
		return nil, err
	}
	var log SessionLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, err
	}
	return &log, nil
}

func saveSessionLog(log *SessionLog) error {
	path := getSessionLogPath(log.SessionID)
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func AddStepToLog(sessionID string, step StepEntry) error {
	log, err := loadSessionLog(sessionID)
	if err != nil {
		return err
	}
	log.Steps = append(log.Steps, step)
	return saveSessionLog(log)
}

func FindMostRecentSessionLog() (string, error) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "goo", "sessions")
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var latestFile string
	var latestTime time.Time
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			info, err := f.Info()
			if err == nil {
				if latestFile == "" || info.ModTime().After(latestTime) {
					latestFile = filepath.Join(dir, f.Name())
					latestTime = info.ModTime()
				}
			}
		}
	}
	if latestFile == "" {
		return "", fmt.Errorf("no recent agent sessions found to undo")
	}
	return latestFile, nil
}

func UndoLastSession() error {
	logPath, err := FindMostRecentSessionLog()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		return err
	}
	var log SessionLog
	if err := json.Unmarshal(data, &log); err != nil {
		return err
	}

	if len(log.Steps) == 0 {
		fmt.Println("No actions to undo in the most recent session.")
		return nil
	}

	undoneAny := false
	for i := len(log.Steps) - 1; i >= 0; i-- {
		step := &log.Steps[i]
		if step.Undone {
			continue
		}
		if step.GitCheckpoint != "" {
			fmt.Printf("⏳ Restoring Git Checkpoint %s...\n", step.GitCheckpoint)
			cmd := exec.Command("git", "stash", "apply", step.GitCheckpoint)
			if step.Path != "" {
				cmd.Dir = filepath.Dir(step.Path)
			}
			if out, err := cmd.CombinedOutput(); err == nil {
				fmt.Printf("✓ Restored Git Checkpoint %s\n", step.GitCheckpoint)
				step.Undone = true
				undoneAny = true
				continue
			} else {
				fmt.Printf("⚠️ Could not apply git stash %s: %s\n", step.GitCheckpoint, string(out))
			}
		}

		switch step.Kind {
		case "write_file":
			fmt.Printf("⏳ Reverting FileWrite on %s...\n", step.Path)
			if step.PrevContent == "" {
				if err := os.Remove(step.Path); err != nil {
					fmt.Printf("✗ Error deleting file %s: %v\n", step.Path, err)
				} else {
					fmt.Printf("✓ Deleted new file %s\n", step.Path)
					step.Undone = true
					undoneAny = true
				}
			} else {
				if err := os.WriteFile(step.Path, []byte(step.PrevContent), 0644); err != nil {
					fmt.Printf("✗ Error writing content to %s: %v\n", step.Path, err)
				} else {
					fmt.Printf("✓ Restored original content of %s\n", step.Path)
					step.Undone = true
					undoneAny = true
				}
			}
		case "run_command":
			fmt.Printf("⚠️  Cannot undo ShellExec command: %s\n", step.Command)
		}
	}

	if undoneAny {
		newData, _ := json.MarshalIndent(log, "", "  ")
		_ = os.WriteFile(logPath, newData, 0600)
		fmt.Println("✓ Undo operation complete.")
	} else {
		fmt.Println("No reversible actions left to undo.")
	}
	return nil
}

func RunAgentSession(session *memory.Session, store *memory.Store, initialInstruction string, dir string, imagePath string) error {
	groqClient, err := ai.NewGroqClient()
	if err != nil {
		return err
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("invalid project directory: %w", err)
	}

	if err := os.MkdirAll(absDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	sessionRoot := absDir
	fmt.Printf("goo chat · %s\n\n", sessionRoot)

	// Load local MCP tools
	mcpTools, _ := GlobalMCPHub.LoadAndInitialize(sessionRoot)
	allActiveTools := append([]ai.Tool{}, AllTools...)
	allActiveTools = append(allActiveTools, mcpTools...)

	sessLog := &SessionLog{SessionID: session.ID}

	// Prepare image URL if provided
	var imageURL string
	if imagePath != "" {
		if strings.HasPrefix(imagePath, "http://") || strings.HasPrefix(imagePath, "https://") {
			imageURL = imagePath
		} else {
			absImg, err := filepath.Abs(imagePath)
			if err == nil {
				data, err := os.ReadFile(absImg)
				if err == nil {
					ext := strings.TrimPrefix(filepath.Ext(absImg), ".")
					if ext == "" || ext == "jpg" {
						ext = "jpeg"
					}
					b64 := base64.StdEncoding.EncodeToString(data)
					imageURL = fmt.Sprintf("data:image/%s;base64,%s", ext, b64)
					fmt.Printf("✓ Loaded visual image context: %s\n", absImg)
				}
			}
		}
	}

	if initialInstruction != "" {
		err = runAgentLoop(session, store, groqClient, initialInstruction, sessionRoot, sessLog, allActiveTools, imageURL)
		if err != nil {
			return err
		}
		printFinalSummary(sessLog)
		return nil
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("goo [%s]> ", filepath.Base(sessionRoot))
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "/exit" || input == "/quit" {
			break
		}
		if input == "/undo" {
			err := UndoLastSession()
			if err != nil {
				fmt.Printf("✗ Undo error: %v\n", err)
			}
			continue
		}
		if input == "/commit" {
			_ = exec.Command("git", "add", "-A").Run()
			diffOut, _ := exec.Command("git", "diff", "--cached").CombinedOutput()
			if len(diffOut) == 0 {
				fmt.Println("No staged changes to commit.")
				continue
			}
			fmt.Println("⏳ Generating commit message...")
			commitMsg, _ := groqClient.Complete(context.Background(), "Generate a concise 1-line git commit message for these changes:\n"+string(diffOut), "")
			commitMsg = strings.TrimSpace(commitMsg)
			if commitMsg == "" {
				commitMsg = "Update codebase via Goo CLI agent"
			}
			_ = exec.Command("git", "commit", "-m", commitMsg).Run()
			fmt.Printf("✓ Committed changes: %s\n", commitMsg)
			continue
		}
		if input == "/review" {
			diffOut, _ := exec.Command("git", "diff").CombinedOutput()
			if len(diffOut) == 0 {
				fmt.Println("No git diff found to review.")
				continue
			}
			fmt.Println("⏳ Reviewing git diff...")
			review, _ := groqClient.Complete(context.Background(), "Review this code diff for bugs, safety issues, and performance optimizations:\n"+string(diffOut), "")
			fmt.Println("\n--- Code Review Feedback ---")
			fmt.Println(review)
			continue
		}
		if input == "/diff" {
			out, _ := exec.Command("git", "diff").CombinedOutput()
			if len(out) == 0 {
				fmt.Println("No git changes detected.")
			} else {
				fmt.Println(string(out))
			}
			continue
		}
		if input == "/mcp" {
			fmt.Println(GlobalMCPHub.ListActiveServers())
			continue
		}
		if input == "/compact" {
			_ = store.UpdateSessionSummary(session.ID, "Compacted session context")
			fmt.Println("✓ Conversation context compacted.")
			continue
		}
		if strings.HasPrefix(input, "/") {
			if input == "/help" {
				fmt.Println("Available commands:\n  cd <dir> — change current working directory & scope\n  $ <cmd>  — execute shell command directly\n  /commit  — auto-generate commit message & commit staged changes\n  /review  — perform code quality & security review on git diff\n  /diff    — show current git diff\n  /mcp     — list active local MCP servers and tools\n  /compact — compact chat history context\n  /undo    — revert recent file edits and git checkpoints\n  /clear   — clear terminal screen\n  /exit    — exit chat session")
				continue
			}
			if input == "/clear" {
				fmt.Print("\033[H\033[2J")
				continue
			}
			fmt.Printf("Unknown command: %s (type /help for help)\n", input)
			continue
		}

		// Handle Warp-like direct terminal navigation (cd <path>)
		if input == "cd" || strings.HasPrefix(input, "cd ") {
			target := strings.TrimSpace(strings.TrimPrefix(input, "cd"))
			var targetDir string
			if target == "" || target == "~" {
				targetDir, _ = os.UserHomeDir()
			} else if strings.HasPrefix(target, "~/") {
				home, _ := os.UserHomeDir()
				targetDir = filepath.Join(home, target[2:])
			} else {
				targetDir = filepath.Join(sessionRoot, target)
			}

			absTarget, err := filepath.Abs(targetDir)
			if err == nil {
				if info, statErr := os.Stat(absTarget); statErr == nil && info.IsDir() {
					_ = os.Chdir(absTarget)
					sessionRoot = absTarget
					fmt.Printf("📂 Scope changed to: %s\n\n", sessionRoot)

					// Refresh local MCP tools for new directory
					mcpTools, _ := GlobalMCPHub.LoadAndInitialize(sessionRoot)
					allActiveTools = append([]ai.Tool{}, AllTools...)
					allActiveTools = append(allActiveTools, mcpTools...)
					continue
				} else {
					fmt.Printf("✗ Directory not found: %s\n\n", target)
					continue
				}
			}
		}

		// Handle direct terminal command execution (! <cmd> or $ <cmd>)
		if strings.HasPrefix(input, "$ ") || strings.HasPrefix(input, "! ") {
			shCmd := strings.TrimSpace(input[2:])
			fmt.Printf("⚡ Executing: %s\n", shCmd)
			cmd := exec.Command("bash", "-c", shCmd)
			cmd.Dir = sessionRoot
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			_ = cmd.Run()
			fmt.Println()
			continue
		}

		err = runAgentLoop(session, store, groqClient, input, sessionRoot, sessLog, allActiveTools, imageURL)
		if err != nil {
			fmt.Printf("✗ Agent error: %v\n", err)
		}
		printFinalSummary(sessLog)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stdin read error: %w", err)
	}
	return nil
}

func buildAgentSystemPrompt(sessionRoot string) string {
	fp, err := GenerateFingerprint(sessionRoot)
	var fpStr string
	if err == nil && fp != nil {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Project type: %s\n", fp.ProjectType))
		if fp.Framework != "" {
			sb.WriteString(fmt.Sprintf("Framework: %s\n", fp.Framework))
		}
		sb.WriteString("Directory structure:\n")
		for _, s := range fp.Structure {
			sb.WriteString(fmt.Sprintf("  - %s\n", s))
		}
		fpStr = sb.String()
	}

	// Read standing context (GOO.md)
	var gooRules string
	gooPaths := []string{
		filepath.Join(sessionRoot, "GOO.md"),
		filepath.Join(sessionRoot, ".goo", "GOO.md"),
	}
	for _, p := range gooPaths {
		if data, err := os.ReadFile(p); err == nil {
			gooRules = string(data)
			break
		}
	}

	policy := config.Get("agent.safety_policy")
	if policy == "" {
		policy = "always_confirm"
	}

	var sb strings.Builder
	sb.WriteString("You are Goo, an expert terminal AI coding agent running on the user's local machine in Agent Mode.\n")
	sb.WriteString(fmt.Sprintf("Session root directory: %s\n", sessionRoot))
	sb.WriteString("All file reads/writes and command execution are relative to this root.\n\n")
	if fpStr != "" {
		sb.WriteString("Current Project Fingerprint:\n")
		sb.WriteString(fpStr)
		sb.WriteString("\n")
	}
	if gooRules != "" {
		sb.WriteString("Project Standing Instructions (GOO.md):\n")
		sb.WriteString(gooRules)
		sb.WriteString("\n\n")
	}

	// Load and inject Agent Skills (.goo/skills, .claude/skills, ~/.config/goo/skills)
	skills, _ := LoadSkills(sessionRoot)
	if len(skills) > 0 {
		sb.WriteString(BuildSkillsPromptBlock(skills))
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Current date/time: %s\n\n", time.Now().Format("Monday, Jan 2 2006 15:04 MST")))
	sb.WriteString(`Instructions:
1. You can call multiple tools to read, edit, delete files, or run command-line commands.
2. For editing files, use the 'write_file' tool. Specify the exact 'old_content' matching a block in the file and provide the 'new_content' replacement block. This ensures edits are patch-based and not full-file overwrites.
3. Be proactive: run builds or tests to verify your changes, read file logs or error messages to debug, etc.
4. DO NOT USE ANY EMOJIS in your output text or step summaries. Use simple text markers (✓, ✗, ⏳, ✎, ?) instead.
5. If a command fails or a file edit fails, re-plan and try another approach. Do not repeat the same failing action without changing the arguments.
6. When the user's task is fully complete, report back a summary of what you did.
`)
	return sb.String()
}

func buildAgentMessages(session *memory.Session, store *memory.Store, userInput string, sessionRoot string) []memory.Message {
	var messages []memory.Message
	messages = append(messages, memory.Message{
		Role:    "system",
		Content: buildAgentSystemPrompt(sessionRoot),
	})

	history, _ := store.GetMessages(session.ID, 100)
	messages = append(messages, history...)

	if userInput != "" {
		messages = append(messages, memory.Message{
			Role:    "user",
			Content: userInput,
		})
	}
	return messages
}

func runAgentLoop(session *memory.Session, store *memory.Store, groq *ai.GroqClient, userInput string, sessionRoot string, sessLog *SessionLog, tools []ai.Tool, imageURL string) error {
	if userInput != "" {
		msg := memory.Message{Role: "user", Content: userInput, ImageURL: imageURL, SessionID: session.ID}
		_ = store.SaveMessage(msg)
	}

	turnCount := 0
	for {
		messages := buildAgentMessages(session, store, "", sessionRoot)

		turnStart := time.Now()
		fmt.Printf("\n✦ Thinking...\n")

		opts := ai.StreamOptions{}
		if turnCount == 0 {
			opts.ReasoningEffort = "high"
			opts.ReasoningFormat = "parsed"
		} else {
			opts.ReasoningEffort = "low"
			opts.ReasoningFormat = "hidden"
		}

		if imageURL != "" {
			opts.Model = "meta-llama/llama-4-scout-17b-16e-instruct"
		}

		var textBuf strings.Builder
		toolCall, err := groq.StreamChatWithToolsEx(context.Background(), messages, io.MultiWriter(os.Stdout, &textBuf), tools, opts)
		if err != nil {
			return err
		}

		thoughtDur := time.Since(turnStart).Round(100 * time.Millisecond)
		if thoughtDur > 0 {
			fmt.Printf("  Thought for %v >\n", thoughtDur)
		}

		assistantMsgContent := textBuf.String()
		if assistantMsgContent != "" {
			fmt.Println()
		}

		if toolCall == nil {
			if assistantMsgContent != "" {
				_ = store.SaveMessage(memory.Message{Role: "assistant", Content: assistantMsgContent, SessionID: session.ID})
			}
			break
		}

		actionDesc := ai.FormatToolAction(toolCall)
		fmt.Printf("  %s\n", actionDesc)

		tier := classifyToolCall(toolCall, sessionRoot)
		approved, editedArgs, err := gateToolCall(toolCall, tier)
		if err != nil {
			return err
		}
		if !approved {
			fmt.Printf("✗ Gated: User declined tool execution.\n\n")
			resMsg := memory.Message{
				Role:       "tool",
				ToolCallID: toolCall.ID,
				ToolName:   toolCall.Name,
				Content:    "Error: execution gated and declined by user.",
				SessionID:  session.ID,
			}
			callMsg := memory.Message{
				Role:       "assistant",
				ToolCallID: toolCall.ID,
				ToolName:   toolCall.Name,
				Content:    assistantMsgContent,
				SessionID:  session.ID,
			}
			_ = store.SaveMessage(callMsg)
			_ = store.SaveMessage(resMsg)
			continue
		}

		if editedArgs != nil {
			toolCall.Arguments = editedArgs
		}

		stepEntry := StepEntry{
			Timestamp:   time.Now(),
			Kind:        toolCall.Name,
			Description: fmt.Sprintf("Execute %s", toolCall.Name),
		}

		// Git Checkpoint for MODERATE or HIGH tier ops
		if tier == TierModerate || tier == TierHigh {
			cmd := exec.Command("git", "stash", "create")
			cmd.Dir = sessionRoot
			if hashBytes, err := cmd.CombinedOutput(); err == nil {
				hash := strings.TrimSpace(string(hashBytes))
				if hash != "" {
					stepEntry.GitCheckpoint = hash
				}
			}
		}

		switch toolCall.Name {
		case "write_file":
			var args struct {
				Path string `json:"path"`
			}
			_ = json.Unmarshal(toolCall.Arguments, &args)
			absPath, _ := filepath.Abs(args.Path)
			stepEntry.Path = absPath
			if data, err := os.ReadFile(absPath); err == nil {
				stepEntry.PrevContent = string(data)
			} else {
				stepEntry.PrevContent = ""
			}
		case "run_command":
			var args struct {
				Command string `json:"command"`
			}
			_ = json.Unmarshal(toolCall.Arguments, &args)
			stepEntry.Command = args.Command
		}

		startTime := time.Now()

		toolDeps := ToolDeps{
			Tavily: search.NewClient(),
			Tasks:  tasks.NewManager(),
			GitHub: func() *github.Client { c, _ := github.NewClient(); return c }(),
		}

		output, err := ExecuteToolCall(toolCall, toolDeps)
		duration := time.Since(startTime)

		var success bool
		var errorMsg string
		if err != nil {
			success = false
			errorMsg = err.Error()
			fmt.Printf("✗ Failed in %v: %v\n\n", duration.Round(time.Millisecond), err)
		} else {
			success = true
			fmt.Printf("✓ Done in %v\n\n", duration.Round(time.Millisecond))
		}

		if success {
			sessLog.Steps = append(sessLog.Steps, stepEntry)
			_ = AddStepToLog(session.ID, stepEntry)
		}

		callMsg := memory.Message{
			Role:       "assistant",
			ToolCallID: toolCall.ID,
			ToolName:   toolCall.Name,
			Content:    assistantMsgContent,
			SessionID:  session.ID,
		}

		resContent := output
		if !success {
			resContent = fmt.Sprintf("Error: %s", errorMsg)
		}

		resMsg := memory.Message{
			Role:       "tool",
			ToolCallID: toolCall.ID,
			ToolName:   toolCall.Name,
			Content:    resContent,
			SessionID:  session.ID,
		}

		_ = store.SaveMessage(callMsg)
		_ = store.SaveMessage(resMsg)
		turnCount++
	}

	return nil
}

func isPathInRoot(path string, root string) bool {
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(root, path)
	}
	var err error
	absPath, err = filepath.Abs(absPath)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	if strings.HasPrefix(rel, "..") {
		return false
	}
	return true
}

func classifyShellCommand(command string, cwd string, sessionRoot string) RiskTier {
	if !isPathInRoot(cwd, sessionRoot) {
		return TierHigh
	}

	cmdLower := strings.ToLower(command)
	
	highKeywords := []string{
		"sudo ", "rm -rf", "rm -f", "rm ", "git push", "env", "printenv", ".env",
		"npm install -g", "npm i -g", "yarn global", "pip install -U", "pip install --user",
	}
	for _, kw := range highKeywords {
		if strings.Contains(cmdLower, kw) {
			return TierHigh
		}
	}

	safeCommands := []string{
		"git status", "git diff", "ls", "npm list", "pwd", "git branch", "git log",
	}
	for _, sc := range safeCommands {
		if strings.HasPrefix(cmdLower, sc) || strings.Contains(cmdLower, " "+sc) {
			return TierSafe
		}
	}

	lowCommands := []string{
		"npm install", "npm i", "yarn install", "go get", "go build", "npm run build", "git add",
	}
	for _, lc := range lowCommands {
		if strings.HasPrefix(cmdLower, lc) || strings.Contains(cmdLower, " "+lc) {
			return TierLow
		}
	}

	modCommands := []string{
		"npm run dev", "npm dev", "git commit", "go run", "python ", "node ",
	}
	for _, mc := range modCommands {
		if strings.HasPrefix(cmdLower, mc) || strings.Contains(cmdLower, " "+mc) {
			return TierModerate
		}
	}

	return TierModerate
}

func classifyToolCall(call *ai.ToolCall, sessionRoot string) RiskTier {
	switch call.Name {
	case "read_file", "find_files", "list_dir", "grep_search", "search_web", "list_tasks", "get_github_prs", "get_github_stats", "get_github_repos":
		return TierSafe

	case "write_file":
		var args struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		if !isPathInRoot(args.Path, sessionRoot) {
			return TierHigh
		}
		return TierLow

	case "delete_file":
		var args struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		if !isPathInRoot(args.Path, sessionRoot) {
			return TierHigh
		}
		return TierModerate

	case "run_command":
		var args struct {
			Command string `json:"command"`
			Cwd     string `json:"cwd"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		cwd := args.Cwd
		if cwd == "" {
			cwd = sessionRoot
		} else {
			absCwd, err := filepath.Abs(cwd)
			if err == nil {
				cwd = absCwd
			}
		}
		return classifyShellCommand(args.Command, cwd, sessionRoot)

	default:
		return TierModerate
	}
}

func checkAllowlist(call *ai.ToolCall) bool {
	allowlist := config.GetAgentAllowlist()
	if len(allowlist) == 0 {
		allowlist = []string{"npm install", "npm run *", "git add", "git status", "git diff", "write_file"}
	}

	if call.Name == "write_file" {
		for _, item := range allowlist {
			if item == "write_file" {
				return true
			}
		}
	}

	if call.Name == "run_command" {
		var args struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		cmd := strings.TrimSpace(args.Command)
		
		for _, item := range allowlist {
			if item == "write_file" {
				continue
			}
			if strings.HasSuffix(item, "*") {
				prefix := strings.TrimSuffix(item, "*")
				if strings.HasPrefix(cmd, prefix) {
					return true
				}
			} else {
				if cmd == item {
					return true
				}
			}
		}
	}

	return false
}

func gateToolCall(call *ai.ToolCall, tier RiskTier) (approved bool, editedArgs json.RawMessage, err error) {
	if tier == TierSafe {
		return true, nil, nil
	}

	policy := config.Get("agent.safety_policy")
	if policy == "" {
		policy = "always_confirm"
	}

	if tier == TierHigh {
		fmt.Printf("⚠️  WARNING: HIGH risk operation detected outside sandbox or matching dangerous pattern!\n")
		return confirmPrompt(call)
	}

	if policy == "allowlist" {
		if checkAllowlist(call) {
			fmt.Printf("✓ Auto-running allowlisted operation: %s\n", call.Name)
			return true, nil, nil
		}
	}

	if policy == "diff_approve" {
		if call.Name == "write_file" {
			return diffAndApproveWrite(call)
		}
		return confirmPrompt(call)
	}

	return confirmPrompt(call)
}

func confirmPrompt(call *ai.ToolCall) (bool, json.RawMessage, error) {
	var promptMsg string
	switch call.Name {
	case "write_file":
		var args struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		promptMsg = fmt.Sprintf("Write/Edit file %s", args.Path)
	case "run_command":
		var args struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		promptMsg = fmt.Sprintf("Run command: %s", args.Command)
	default:
		promptMsg = fmt.Sprintf("Execute tool %s", call.Name)
	}

	fmt.Printf("? %s\n", promptMsg)
	fmt.Print("  Confirm execution? [y/N/e to edit] ")
	
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "y" || input == "yes" {
		return true, nil, nil
	}
	if input == "e" || input == "edit" {
		return editToolCall(call)
	}

	return false, nil, nil
}

func diffAndApproveWrite(call *ai.ToolCall) (bool, json.RawMessage, error) {
	var args struct {
		Path       string `json:"path"`
		OldContent string `json:"old_content"`
		NewContent string `json:"new_content"`
	}
	_ = json.Unmarshal(call.Arguments, &args)
	absPath, _ := filepath.Abs(args.Path)

	var oldText string
	if data, err := os.ReadFile(absPath); err == nil {
		oldText = string(data)
	}

	var newText string
	if oldText == "" {
		newText = args.NewContent
	} else {
		if args.OldContent == "" {
			newText = args.NewContent
		} else {
			newText = strings.Replace(oldText, args.OldContent, args.NewContent, 1)
		}
	}

	fmt.Printf("✎ FileWrite  %s\n", args.Path)
	fmt.Println("  ┌─────────────────────────────────")
	diffStr := LineDiff(oldText, newText)
	diffLines := strings.Split(diffStr, "\n")
	for _, l := range diffLines {
		fmt.Printf("  │ %s\n", l)
	}
	fmt.Println("  └─────────────────────────────────")

	fmt.Print("  Apply this diff? [y/N/e to edit] ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "y" || input == "yes" {
		return true, nil, nil
	}
	if input == "e" || input == "edit" {
		edited, err := editContent(newText)
		if err != nil {
			return false, nil, err
		}
		args.OldContent = oldText
		args.NewContent = edited
		newArgsBytes, _ := json.Marshal(args)
		return true, newArgsBytes, nil
	}

	return false, nil, nil
}

func editToolCall(call *ai.ToolCall) (bool, json.RawMessage, error) {
	if call.Name == "run_command" {
		var args struct {
			Command string `json:"command"`
			Cwd     string `json:"cwd"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		
		fmt.Printf("Current command: %s\n", args.Command)
		fmt.Print("Enter new command: ")
		reader := bufio.NewReader(os.Stdin)
		newCmd, _ := reader.ReadString('\n')
		newCmd = strings.TrimSpace(newCmd)
		if newCmd == "" {
			return false, nil, nil
		}
		args.Command = newCmd
		newArgsBytes, _ := json.Marshal(args)
		return true, newArgsBytes, nil
	}
	
	if call.Name == "write_file" {
		var args struct {
			Path            string `json:"path"`
			OldContent      string `json:"old_content"`
			NewContent      string `json:"new_content"`
			CreateIfMissing bool   `json:"create_if_missing"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		
		edited, err := editContent(args.NewContent)
		if err != nil {
			return false, nil, err
		}
		args.NewContent = edited
		newArgsBytes, _ := json.Marshal(args)
		return true, newArgsBytes, nil
	}
	
	return true, nil, nil
}

func editContent(content string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}
	tmpFile, err := os.CreateTemp("", "goo-edit-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())
	
	if _, err := tmpFile.WriteString(content); err != nil {
		return "", err
	}
	tmpFile.Close()
	
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	
	edited, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}
	return string(edited), nil
}

func printFinalSummary(log *SessionLog) {
	if len(log.Steps) == 0 {
		return
	}

	var created []string
	var modified []string
	var ran []string

	for _, step := range log.Steps {
		switch step.Kind {
		case "write_file":
			if step.PrevContent == "" {
				created = append(created, step.Path)
			} else {
				modified = append(modified, step.Path)
			}
		case "run_command":
			ran = append(ran, step.Command)
		}
	}

	fmt.Println("\nSummary")
	fmt.Printf("  Steps executed: %d\n", len(log.Steps))

	if len(created) > 0 {
		fmt.Println("\n  Created")
		for _, c := range created {
			fmt.Printf("    + %s\n", c)
		}
	}

	if len(modified) > 0 {
		fmt.Println("\n  Modified")
		for _, m := range modified {
			fmt.Printf("    ~ %s\n", m)
		}
	}

	if len(ran) > 0 {
		fmt.Println("\n  Ran")
		for _, r := range ran {
			fmt.Printf("    • %s\n", r)
		}
	}
	fmt.Println()
}

func LineDiff(oldStr, newStr string) string {
	oldLines := strings.Split(oldStr, "\n")
	newLines := strings.Split(newStr, "\n")
	
	m := len(oldLines)
	n := len(newLines)
	
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}
	
	var diff []string
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			diff = append(diff, "  "+oldLines[i-1])
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			diff = append(diff, "+ "+newLines[j-1])
			j--
		} else if i > 0 && (j == 0 || dp[i-1][j] > dp[i][j-1]) {
			diff = append(diff, "- "+oldLines[i-1])
			i--
		}
	}
	
	for k := 0; k < len(diff)/2; k++ {
		diff[k], diff[len(diff)-1-k] = diff[len(diff)-1-k], diff[k]
	}
	
	var contextDiff []string
	const contextWindow = 3
	
	type hunk struct {
		start int
		end   int
	}
	var hunks []hunk
	inChange := false
	var currentHunk hunk
	
	for idx, line := range diff {
		isChange := strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-")
		if isChange {
			if !inChange {
				inChange = true
				currentHunk.start = max(0, idx-contextWindow)
			}
			currentHunk.end = min(len(diff)-1, idx+contextWindow)
		} else {
			if inChange && idx > currentHunk.end {
				inChange = false
				hunks = append(hunks, currentHunk)
			}
		}
	}
	if inChange {
		hunks = append(hunks, currentHunk)
	}
	
	var mergedHunks []hunk
	for _, h := range hunks {
		if len(mergedHunks) == 0 {
			mergedHunks = append(mergedHunks, h)
		} else {
			last := &mergedHunks[len(mergedHunks)-1]
			if h.start <= last.end {
				last.end = max(last.end, h.end)
			} else {
				mergedHunks = append(mergedHunks, h)
			}
		}
	}
	
	if len(mergedHunks) == 0 {
		return "No changes."
	}
	
	for idx, h := range mergedHunks {
		if idx > 0 {
			contextDiff = append(contextDiff, "...")
		}
		for k := h.start; k <= h.end; k++ {
			contextDiff = append(contextDiff, diff[k])
		}
	}
	
	return strings.Join(contextDiff, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type SessionLog struct {
	SessionID string       `json:"session_id"`
	Steps     []StepEntry  `json:"steps"`
}
