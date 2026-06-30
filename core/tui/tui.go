package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kingjethro999/goo/memory"
	"github.com/kingjethro999/goo/tools/ai"
	"github.com/muesli/reflow/wordwrap"
)

// Available models to cycle through with Tab
var availableModels = []string{
	"llama-3.3-70b-versatile",
	"llama-3.1-70b-versatile",
	"llama3-groq-70b-8192-tool-use-preview",
	"mixtral-8x7b-32768",
	"gemma2-9b-it",
}

var (
	styleHeader    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).PaddingLeft(1)
	styleSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleUser      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styleAssistant = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	styleSystem    = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("244"))
	styleError     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	styleStatus    = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	styleHint      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleModel     = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	styleToolOk    = lipgloss.NewStyle().Foreground(lipgloss.Color("76"))
	styleToolCall  = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
)

type DisplayMessage struct {
	Role    string
	Content string
}

type Model struct {
	session      *memory.Session
	store        *memory.Store
	groq         *ai.GroqClient
	ctx          *memory.ContextBuilder
	viewport     viewport.Model
	input        textarea.Model
	spinner      spinner.Model
	messages     []DisplayMessage
	width        int
	height       int
	status       string // "idle" | "streaming" | "tool_execution"
	err          error
	tools        []ai.Tool
	dispatcher   func(*ai.ToolCall) (string, error)
	modelIndex   int
	inputHistory []string
	historyPos   int
	savedDraft   string
}

// Message types for the tea update loop
type StreamChunkMsg struct {
	Chunk string
	Next  tea.Cmd
}
type StreamDoneMsg struct{ Full string }
type StreamErrMsg struct{ Err error }
type ToolCallMsg struct{ Call *ai.ToolCall }
type ToolResultMsg struct {
	Result string
	CallID string
	Name   string
}

// New creates a new TUI model.
func New(session *memory.Session, store *memory.Store, groq *ai.GroqClient, tools []ai.Tool, dispatcher func(*ai.ToolCall) (string, error)) *Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message…"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 10000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent(styleSystem.Render("  Goo is ready. Start typing below.\n  Shift+Enter = newline  ·  Tab = cycle model  ·  ↑↓ = history"))

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styleStatus

	return &Model{
		session:    session,
		store:      store,
		groq:       groq,
		ctx:        memory.NewContextBuilder(session, store),
		input:      ta,
		viewport:   vp,
		spinner:    s,
		status:     "idle",
		tools:      tools,
		dispatcher: dispatcher,
		historyPos: -1,
	}
}

func (m *Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEsc:
			if m.input.Value() == "" {
				return m, tea.Quit
			}

		case tea.KeyTab:
			// Cycle through models
			if m.status == "idle" {
				m.modelIndex = (m.modelIndex + 1) % len(availableModels)
				m.groq.SetModel(availableModels[m.modelIndex])
				m.messages = append(m.messages, DisplayMessage{
					Role:    "system",
					Content: fmt.Sprintf("⟳ Model switched to: %s", availableModels[m.modelIndex]),
				})
				m.updateViewport()
			}
			return m, nil

		case tea.KeyUp:
			// Navigate input history (only when idle and cursor is at top)
			if m.status == "idle" && len(m.inputHistory) > 0 {
				if m.historyPos == -1 {
					m.savedDraft = m.input.Value()
				}
				next := m.historyPos + 1
				if next < len(m.inputHistory) {
					m.historyPos = next
					m.input.SetValue(m.inputHistory[len(m.inputHistory)-1-m.historyPos])
				}
				return m, nil
			}

		case tea.KeyDown:
			// Navigate input history downward
			if m.status == "idle" && m.historyPos > -1 {
				m.historyPos--
				if m.historyPos == -1 {
					m.input.SetValue(m.savedDraft)
				} else {
					m.input.SetValue(m.inputHistory[len(m.inputHistory)-1-m.historyPos])
				}
				return m, nil
			}

		case tea.KeyEnter:
			// Shift+Enter inserts a newline; plain Enter submits
			if msg.Alt {
				// Alt+Enter also inserts newline for compatibility
				break
			}
			if strings.Contains(msg.String(), "shift") {
				break
			}
			v := strings.TrimSpace(m.input.Value())
			if v == "" || m.status != "idle" {
				return m, nil
			}
			m.input.Reset()
			m.historyPos = -1
			m.savedDraft = ""
			// Add to history (avoid duplicates at top)
			if len(m.inputHistory) == 0 || m.inputHistory[len(m.inputHistory)-1] != v {
				m.inputHistory = append(m.inputHistory, v)
				if len(m.inputHistory) > 100 {
					m.inputHistory = m.inputHistory[1:]
				}
			}
			m.status = "streaming"
			m.messages = append(m.messages, DisplayMessage{Role: "user", Content: v})
			m.updateViewport()

			m.saveMessage(memory.Message{Role: "user", Content: v, SessionID: m.session.ID})
			messages := m.ctx.Build(v)
			return m, tea.Batch(m.spinner.Tick, streamCmd(m.groq, messages, m.tools))
		}

	case StreamChunkMsg:
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
			m.messages[len(m.messages)-1].Content += msg.Chunk
		} else {
			m.messages = append(m.messages, DisplayMessage{Role: "assistant", Content: msg.Chunk})
		}
		m.updateViewport()
		return m, msg.Next

	case StreamDoneMsg:
		m.status = "idle"
		m.saveMessage(memory.Message{Role: "assistant", Content: msg.Full, SessionID: m.session.ID})
		return m, nil

	case ToolCallMsg:
		m.status = "tool_execution"
		m.messages = append(m.messages, DisplayMessage{
			Role:    "tool_call",
			Content: fmt.Sprintf("⚙  Calling: %s", msg.Call.Name),
		})
		m.updateViewport()

		return m, func() tea.Msg {
			result, err := m.dispatcher(msg.Call)
			if err != nil {
				result = fmt.Sprintf("tool error: %v", err)
			}
			return ToolResultMsg{Result: result, CallID: msg.Call.ID, Name: msg.Call.Name}
		}

	case ToolResultMsg:
		callMsg := memory.Message{Role: "assistant", ToolCallID: msg.CallID, ToolName: msg.Name, Content: "", SessionID: m.session.ID}
		resMsg := memory.Message{Role: "tool", ToolCallID: msg.CallID, ToolName: msg.Name, Content: msg.Result, SessionID: m.session.ID}
		m.saveMessage(callMsg)
		m.saveMessage(resMsg)
		m.messages = append(m.messages, DisplayMessage{
			Role:    "tool_result",
			Content: fmt.Sprintf("✔  %s → %s", msg.Name, truncate(msg.Result, 200)),
		})
		m.updateViewport()
		m.status = "streaming"
		messages := m.ctx.BuildFollowUp()
		return m, streamCmd(m.groq, messages, m.tools)

	case StreamErrMsg:
		m.status = "idle"
		m.err = msg.Err
		m.messages = append(m.messages, DisplayMessage{Role: "error", Content: msg.Err.Error()})
		m.updateViewport()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerH := 2
		footerH := 6
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerH - footerH
		m.input.SetWidth(msg.Width)
		m.updateViewport()
	}

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	if m.status != "idle" {
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateViewport() {
	var sb strings.Builder
	wrapStyle := lipgloss.NewStyle().Width(m.viewport.Width)

	for _, msg := range m.messages {
		content := wordwrap.String(msg.Content, m.viewport.Width-4)
		switch msg.Role {
		case "user":
			sb.WriteString(styleUser.Render("YOU"))
			sb.WriteByte('\n')
			sb.WriteString(wrapStyle.Render(content))
			sb.WriteString("\n\n")
		case "assistant":
			sb.WriteString(styleAssistant.Render("GOO"))
			sb.WriteByte('\n')
			sb.WriteString(wrapStyle.Render(content))
			sb.WriteString("\n\n")
		case "system":
			sb.WriteString(styleSystem.Render("  " + wrapStyle.Render(content)))
			sb.WriteString("\n\n")
		case "tool_call":
			sb.WriteString(styleToolCall.Render("  " + wrapStyle.Render(content)))
			sb.WriteString("\n")
		case "tool_result":
			sb.WriteString(styleToolOk.Render("  " + wrapStyle.Render(content)))
			sb.WriteString("\n\n")
		case "error":
			sb.WriteString(styleError.Render("  ✖  Error: " + wrapStyle.Render(content)))
			sb.WriteString("\n\n")
		default:
			sb.WriteString(styleSystem.Render(strings.ToUpper(msg.Role)))
			sb.WriteByte('\n')
			sb.WriteString(wrapStyle.Render(content))
			sb.WriteString("\n\n")
		}
	}
	m.viewport.SetContent(sb.String())
	m.viewport.GotoBottom()
}

func (m *Model) saveMessage(msg memory.Message) {
	_ = m.store.SaveMessage(msg)

	go func() {
		count, err := m.store.CountMessages(m.session.ID)
		if err == nil && count > 0 && count%40 == 0 {
			_ = memory.SummariseSession(m.session.ID, m.store, m.groq)
		}
	}()
}

func (m *Model) View() string {
	var b strings.Builder

	model := availableModels[m.modelIndex]
	if m.groq != nil {
		model = m.groq.Model()
	}

	title := fmt.Sprintf(" Goo  ·  session %s", m.session.ID[:8])
	if m.session.Title != "" && m.session.Title != "New Chat" {
		title = fmt.Sprintf(" Goo  ·  %s", m.session.Title)
	}
	modelTag := styleModel.Render("  [" + shortModelName(model) + "]")
	header := styleHeader.Render(title) + modelTag
	fmt.Fprintf(&b, "%s\n", header)
	fmt.Fprintf(&b, "%s\n", styleSeparator.Render(strings.Repeat("─", m.width)))
	fmt.Fprintf(&b, "%s\n", m.viewport.View())
	fmt.Fprintf(&b, "%s\n", styleSeparator.Render(strings.Repeat("─", m.width)))

	if m.status != "idle" {
		label := "Thinking"
		if m.status == "tool_execution" {
			label = "Running tool"
		}
		fmt.Fprintf(&b, "%s\n", styleStatus.Render(fmt.Sprintf("  %s %s…", m.spinner.View(), label)))
	} else {
		hint := styleHint.Render("  Enter=send  Shift+Enter=newline  Tab=model  ↑↓=history  Ctrl+C=quit")
		fmt.Fprintf(&b, "%s\n", hint)
	}

	b.WriteString(m.input.View())
	return b.String()
}

// shortModelName trims verbose model names for the status bar.
func shortModelName(model string) string {
	replacer := strings.NewReplacer(
		"llama-3.3-70b-versatile", "llama3.3-70b",
		"llama-3.1-70b-versatile", "llama3.1-70b",
		"llama3-groq-70b-8192-tool-use-preview", "llama3-tool-70b",
		"mixtral-8x7b-32768", "mixtral-8x7b",
		"gemma2-9b-it", "gemma2-9b",
		"gpt-4o", "gpt-4o",
		"gpt-4o-mini", "gpt-4o-mini",
		"claude-3-5-sonnet-20241022", "claude-3.5-sonnet",
		"deepseek-chat", "deepseek-chat",
	)
	return replacer.Replace(model)
}

// writerAdapter pipes streaming text chunks into a buffered channel.
type writerAdapter struct {
	chunks chan<- string
}

func (w *writerAdapter) Write(p []byte) (int, error) {
	w.chunks <- string(p)
	return len(p), nil
}

// streamCmd starts the Groq streaming request in a goroutine.
func streamCmd(groq *ai.GroqClient, messages []memory.Message, tools []ai.Tool) tea.Cmd {
	chunks := make(chan string, 100)
	errs := make(chan error, 1)
	full := &strings.Builder{}

	type goroutineResult struct {
		tc *ai.ToolCall
	}
	resChan := make(chan goroutineResult, 1)

	go func() {
		w := &writerAdapter{chunks: chunks}
		tc, err := groq.StreamChatWithTools(context.Background(), messages, io.MultiWriter(w, full), tools)
		if err != nil {
			errs <- err
			return
		}
		close(chunks)
		resChan <- goroutineResult{tc: tc}
	}()

	var readLoop func() tea.Msg
	readLoop = func() tea.Msg {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				res := <-resChan
				if res.tc != nil {
					return ToolCallMsg{Call: res.tc}
				}
				return StreamDoneMsg{Full: full.String()}
			}
			return StreamChunkMsg{Chunk: chunk, Next: readLoop}
		case err := <-errs:
			return StreamErrMsg{Err: err}
		}
	}

	return readLoop
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
