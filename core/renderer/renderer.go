package renderer

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/kingjethro999/goo/memory"
)

// Colours (ANSI escape codes)
const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	italic  = "\033[3m"
	cyan    = "\033[36m"
	magenta = "\033[35m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	red     = "\033[31m"
	blue    = "\033[34m"
	gray    = "\033[90m"
)

// Renderer handles all terminal output for Goo.
type Renderer struct {
	noColor bool
}

// New creates a Renderer. Detects NO_COLOR env var.
func New() *Renderer {
	_, noColor := os.LookupEnv("NO_COLOR")
	return &Renderer{noColor: noColor}
}

func (r *Renderer) color(code string) string {
	if r.noColor {
		return ""
	}
	return code
}

// PrintSessionHeader prints the session start header.
func (r *Renderer) PrintSessionHeader(session *memory.Session) {
	line := strings.Repeat("─", 60)
	fmt.Printf("%s%s%s\n", r.color(cyan), line, r.color(reset))
	fmt.Printf("  %s%sGoo AI CLI%s  ·  session %s%s%s\n",
		r.color(bold), r.color(cyan), r.color(reset),
		r.color(dim), session.ID[:8], r.color(reset))
	fmt.Printf("  %s%s%s\n", r.color(dim),
		session.StartedAt.Format("Mon Jan 2 2006, 15:04"), r.color(reset))
	fmt.Printf("%s%s%s\n", r.color(cyan), line, r.color(reset))
}

// PrintHint prints a dimmed hint line.
func (r *Renderer) PrintHint(hint string) {
	fmt.Printf("  %s%s%s\n\n", r.color(dim), hint, r.color(reset))
}

// PrintPrompt prints the user input prompt.
func (r *Renderer) PrintPrompt() {
	fmt.Printf("%s%s❯%s ", r.color(bold), r.color(green), r.color(reset))
}

// PrintAILabel prints the AI response label.
func (r *Renderer) PrintAILabel() {
	fmt.Printf("\n%s%s✦ Goo%s  ", r.color(bold), r.color(cyan), r.color(reset))
}

// PrintInfo prints an info message.
func (r *Renderer) PrintInfo(msg string) {
	fmt.Printf("%s%s  %s%s\n", r.color(blue), "ℹ", msg, r.color(reset))
}

// PrintSuccess prints a success message.
func (r *Renderer) PrintSuccess(msg string) {
	fmt.Printf("%s%s  %s%s\n", r.color(green), "✓", msg, r.color(reset))
}

// PrintError prints an error message.
func (r *Renderer) PrintError(err error) {
	fmt.Printf("%s%s  Error: %v%s\n", r.color(red), "✗", err, r.color(reset))
}

// PrintWarning prints a warning message.
func (r *Renderer) PrintWarning(msg string) {
	fmt.Printf("%s%s  %s%s\n", r.color(yellow), "⚠", msg, r.color(reset))
}

// StreamWriter returns an io.Writer that writes directly to stdout.
func (r *Renderer) StreamWriter(out io.Writer) io.Writer {
	return out
}

// PrintFollowUp prints a follow-up hint below the AI response.
func (r *Renderer) PrintFollowUp(suggestion string) {
	if suggestion == "" {
		return
	}
	fmt.Printf("  %s%s · %s%s\n", r.color(dim), r.color(italic), suggestion, r.color(reset))
}

// PrintFollowUpAuto prints an auto-prompt follow-up signal (the AI's question, highlighted).
func (r *Renderer) PrintFollowUpAuto() {
	fmt.Printf("  %s%s↳ (responding to above question)%s\n", r.color(magenta), r.color(italic), r.color(reset))
}

// PrintSessionTable prints a list of sessions in a formatted table.
func (r *Renderer) PrintSessionTable(sessions []*memory.Session) {
	if len(sessions) == 0 {
		r.PrintInfo("No past sessions found. Start one with: goo chat")
		return
	}
	line := strings.Repeat("─", 72)
	fmt.Printf("%s%s%s\n", r.color(dim), line, r.color(reset))
	fmt.Printf("  %-10s  %-20s  %-35s\n",
		r.color(bold)+"ID"+r.color(reset),
		r.color(bold)+"Started"+r.color(reset),
		r.color(bold)+"Title"+r.color(reset))
	fmt.Printf("%s%s%s\n", r.color(dim), line, r.color(reset))
	for _, s := range sessions {
		title := s.Title
		if title == "" {
			title = r.color(dim) + "(untitled)" + r.color(reset)
		}
		if len(title) > 35 {
			title = title[:32] + "..."
		}
		fmt.Printf("  %-10s  %-20s  %s\n",
			s.ID[:8],
			s.StartedAt.Format("Jan 2 15:04"),
			title)
	}
	fmt.Printf("%s%s%s\n", r.color(dim), line, r.color(reset))
}

// PrintResumeHeader prints the session resume greeting.
func (r *Renderer) PrintResumeHeader(session *memory.Session, lastMsgs []memory.Message) {
	line := strings.Repeat("─", 60)
	fmt.Printf("%s%s%s\n", r.color(cyan), line, r.color(reset))
	title := session.Title
	if title == "" {
		title = session.ID[:8]
	}
	fmt.Printf("  %s%sResuming:%s %s\n", r.color(bold), r.color(cyan), r.color(reset), title)
	fmt.Printf("  %sStarted: %s%s\n",
		r.color(dim), session.StartedAt.Format("Mon Jan 2 2006, 15:04"), r.color(reset))
	if len(lastMsgs) > 0 {
		last := lastMsgs[len(lastMsgs)-1]
		preview := last.Content
		if len(preview) > 60 {
			preview = preview[:57] + "..."
		}
		fmt.Printf("  %sLast message: \"%s\"%s\n", r.color(dim), preview, r.color(reset))
	}
	fmt.Printf("%s%s%s\n", r.color(cyan), line, r.color(reset))
}

// PrintSearchResults renders web search results.
func (r *Renderer) PrintSearchResults(query string, results []SearchResult, answer string) {
	line := strings.Repeat("─", 60)
	fmt.Printf("\n%s%s%s\n", r.color(cyan), line, r.color(reset))
	fmt.Printf("  %s%s Web Search: \"%s\"%s\n", r.color(bold), r.color(cyan), query, r.color(reset))
	fmt.Printf("%s%s%s\n", r.color(cyan), line, r.color(reset))

	if answer != "" {
		fmt.Printf("\n  %s%sAI Summary:%s %s\n\n", r.color(bold), r.color(green), r.color(reset), answer)
	}

	for i, result := range results {
		if i >= 5 {
			break
		}
		fmt.Printf("  %s%d.%s %s%s%s\n", r.color(bold), i+1, r.color(reset), r.color(bold), result.Title, r.color(reset))
		fmt.Printf("     %s%s%s\n", r.color(dim), result.URL, r.color(reset))
		snippet := result.Content
		if len(snippet) > 150 {
			snippet = snippet[:147] + "..."
		}
		fmt.Printf("     %s\n\n", snippet)
	}
	fmt.Printf("%s%s%s\n\n", r.color(dim), line, r.color(reset))
}

// PrintTaskTable renders a task list.
func (r *Renderer) PrintTaskTable(tasks []TaskRow) {
	if len(tasks) == 0 {
		r.PrintInfo("No tasks found. Add one with: goo task add \"title\"")
		return
	}
	line := strings.Repeat("─", 72)
	fmt.Printf("%s%s%s\n", r.color(dim), line, r.color(reset))
	fmt.Printf("  %-4s  %-38s  %-8s  %-8s  %-10s\n",
		r.color(bold)+"ID"+r.color(reset),
		r.color(bold)+"Title"+r.color(reset),
		r.color(bold)+"Priority"+r.color(reset),
		r.color(bold)+"Status"+r.color(reset),
		r.color(bold)+"Due"+r.color(reset))
	fmt.Printf("%s%s%s\n", r.color(dim), line, r.color(reset))
	for _, t := range tasks {
		priorityColor := r.color(reset)
		switch t.Priority {
		case "urgent":
			priorityColor = r.color(red)
		case "high":
			priorityColor = r.color(yellow)
		case "medium":
			priorityColor = r.color(blue)
		case "low":
			priorityColor = r.color(dim)
		}
		statusColor := r.color(reset)
		if t.Status == "done" {
			statusColor = r.color(dim)
		}
		due := ""
		if !t.DueDate.IsZero() {
			due = t.DueDate.Format("Jan 2")
			if t.DueDate.Before(time.Now()) && t.Status != "done" {
				due = r.color(red) + due + r.color(reset)
			}
		}
		title := t.Title
		if len(title) > 38 {
			title = title[:35] + "..."
		}
		fmt.Printf("  %-4d  %-38s  %s%-8s%s  %s%-8s%s  %-10s\n",
			t.ID, title,
			priorityColor, t.Priority, r.color(reset),
			statusColor, t.Status, r.color(reset),
			due)
	}
	fmt.Printf("%s%s%s\n", r.color(dim), line, r.color(reset))
}

// SearchResult is a single search result for rendering.
type SearchResult struct {
	Title   string
	URL     string
	Content string
	Score   float64
}

// TaskRow is a simplified task for rendering.
type TaskRow struct {
	ID       int
	Title    string
	Priority string
	Status   string
	DueDate  time.Time
}

// Divider prints a full-width divider.
func (r *Renderer) Divider() {
	fmt.Printf("%s%s%s\n", r.color(dim), strings.Repeat("─", 60), r.color(reset))
}
