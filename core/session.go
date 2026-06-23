package core

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kingjethro999/goo/core/renderer"
	"github.com/kingjethro999/goo/core/tui"
	"github.com/kingjethro999/goo/memory"
	"github.com/kingjethro999/goo/tools/ai"
	"github.com/kingjethro999/goo/tools/github"
	"github.com/kingjethro999/goo/tools/search"
	"github.com/kingjethro999/goo/tools/tasks"
)

// RunChatSession starts and manages an interactive chat session using the Bubbletea TUI.
func RunChatSession(session *memory.Session, store *memory.Store) error {
	groqClient, err := ai.NewGroqClient()
	if err != nil {
		return err
	}

	if session.Title == "" {
		_ = store.SetSessionTitle(session.ID, "New Chat")
	}

	toolDeps := ToolDeps{
		Tavily: search.NewClient(),
		Tasks:  tasks.NewManager(),
		GitHub: func() *github.Client { c, _ := github.NewClient(); return c }(),
	}

	dispatcher := func(call *ai.ToolCall) (string, error) {
		return ExecuteToolCall(call, toolDeps)
	}

	p := tea.NewProgram(
		tui.New(session, store, groqClient, AllTools, dispatcher),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}

// RunAskOnce sends a single question and prints the streamed response.
func RunAskOnce(question string, store *memory.Store) error {
	r := renderer.New()
	groqClient, err := ai.NewGroqClient()
	if err != nil {
		return err
	}

	session, err := store.NewSession("ask")
	if err != nil {
		return err
	}

	ctx := memory.NewContextBuilder(session, store)
	messages := ctx.Build(question)

	r.PrintAILabel()
	var fullResponse strings.Builder
	if err := groqClient.StreamChat(context.Background(), messages, &fullResponse); err != nil {
		return fmt.Errorf("AI error: %w", err)
	}
	fmt.Println()
	return nil
}

func handleSlashCommand(input string, session *memory.Session, store *memory.Store, r *renderer.Renderer, groq *ai.GroqClient) error {
	parts := strings.SplitN(strings.TrimPrefix(input, "/"), " ", 2)
	cmd := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	switch cmd {
	case "help":
		printHelp(r)
	case "context":
		msgs, _ := store.GetMessages(session.ID, 10)
		r.PrintInfo(fmt.Sprintf("Context: %d messages in session", len(msgs)))
	case "search":
		if args == "" {
			r.PrintWarning("Usage: /search <query>")
			return nil
		}
		client := search.NewClient()
		resp, err := client.Search(args)
		if err != nil {
			return err
		}
		r.PrintInfo(search.FormatForAI(resp))
	case "clear":
		fmt.Print("\033[H\033[2J")
		r.PrintSessionHeader(session)
	case "model":
		if args == "" {
			r.PrintInfo(fmt.Sprintf("Current model: %s", groq.Model()))
			return nil
		}
		groq.SetModel(args)
		r.PrintSuccess(fmt.Sprintf("Switched to model: %s", args))
	case "summary":
		sess, err := store.GetSession(session.ID)
		if err != nil || sess.Summary == "" {
			r.PrintInfo("No summary yet (sessions are summarised after 40 messages).")
		} else {
			r.PrintInfo("Session summary:\n" + sess.Summary)
		}
	case "history":
		sessions, err := store.ListSessions(10)
		if err != nil {
			return err
		}
		r.PrintSessionTable(sessions)
	case "export":
		md, err := store.ExportSession(session.ID)
		if err != nil {
			return err
		}
		fname := "goo-session-" + session.ID[:8] + ".md"
		if err := os.WriteFile(fname, []byte(md), 0644); err != nil {
			return err
		}
		r.PrintSuccess(fmt.Sprintf("Exported to %s", fname))
	default:
		r.PrintWarning(fmt.Sprintf("Unknown command: /%s  (type /help for help)", cmd))
	}
	return nil
}

func printHelp(r *renderer.Renderer) {
	help := `
  Slash commands (also available inside the TUI):
    /search <query>   — web search (Tavily)
    /model <name>     — switch AI model
    /history          — list recent sessions
    /summary          — show session summary
    /context          — show context window info
    /export           — save session to markdown file
    /clear            — clear the screen
    /help             — show this help
    /exit             — end session
`
	fmt.Println(help)
}
