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
)

var (
	styleHeader    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).PaddingLeft(1)
	styleSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleUser      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styleAssistant = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	styleSystem    = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("244"))
	styleError     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	styleStatus    = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
)

type DisplayMessage struct {
	Role    string
	Content string
}

type Model struct {
	session    *memory.Session
	store      *memory.Store
	groq       *ai.GroqClient
	ctx        *memory.ContextBuilder
	viewport   viewport.Model
	input      textarea.Model
	spinner    spinner.Model
	messages   []DisplayMessage
	width      int
	height     int
	status     string // "idle" | "streaming" | "tool_execution"
	err        error
	tools      []ai.Tool
	dispatcher func(*ai.ToolCall) (string, error)
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

// New creates a new TUI model wired to the given session, store, Groq client,
// tool definitions and dispatcher function.
func New(session *memory.Session, store *memory.Store, groq *ai.GroqClient, tools []ai.Tool, dispatcher func(*ai.ToolCall) (string, error)) *Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message… (Enter to send, Alt+Enter for newline)"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 10000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent(styleSystem.Render("Goo is ready. Start typing to begin a conversation."))

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
			// Esc only quits when input is empty to avoid losing text accidentally
			if m.input.Value() == "" {
				return m, tea.Quit
			}
		case tea.KeyEnter:
			// Alt+Enter inserts a newline; plain Enter submits
			if msg.Alt {
				break
			}
			v := strings.TrimSpace(m.input.Value())
			if v == "" || m.status != "idle" {
				return m, nil
			}
			m.input.Reset()
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
			Role:    "system",
			Content: fmt.Sprintf("⚙ Calling tool: %s", msg.Call.Name),
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
		// Save the tool call message and the tool result message to the store
		callMsg := memory.Message{Role: "assistant", ToolCallID: msg.CallID, Content: fmt.Sprintf("(called %s)", msg.Name), SessionID: m.session.ID}
		resMsg := memory.Message{Role: "tool", ToolCallID: msg.CallID, Content: msg.Result, SessionID: m.session.ID}
		m.saveMessage(callMsg)
		m.saveMessage(resMsg)
		m.messages = append(m.messages, DisplayMessage{
			Role:    "system",
			Content: fmt.Sprintf("✔ %s returned: %s", msg.Name, truncate(msg.Result, 120)),
		})
		m.updateViewport()
		m.status = "streaming"
		// Stream the follow-up after injecting tool results into context
		messages := m.ctx.Build("")
		return m, streamCmd(m.groq, messages, m.tools)

	case StreamErrMsg:
		m.status = "idle"
		m.err = msg.Err
		m.messages = append(m.messages, DisplayMessage{Role: "error", Content: "Error: " + msg.Err.Error()})
		m.updateViewport()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerH := 2
		footerH := 5
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
	for _, msg := range m.messages {
		var label string
		var content string
		switch msg.Role {
		case "user":
			label = styleUser.Render("YOU")
			content = msg.Content
		case "assistant":
			label = styleAssistant.Render("GOO")
			content = msg.Content
		case "system":
			sb.WriteString(styleSystem.Render("  ", msg.Content))
			sb.WriteString("\n\n")
			continue
		case "error":
			sb.WriteString(styleError.Render("  ✖ ", msg.Content))
			sb.WriteString("\n\n")
			continue
		default:
			label = styleSystem.Render(strings.ToUpper(msg.Role))
			content = msg.Content
		}
		sb.WriteString(label)
		sb.WriteByte('\n')
		sb.WriteString(content)
		sb.WriteString("\n\n")
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

	title := fmt.Sprintf("Goo  ·  session %s", m.session.ID[:8])
	if m.session.Title != "" && m.session.Title != "New Chat" {
		title = fmt.Sprintf("Goo  ·  %s", m.session.Title)
	}
	fmt.Fprintf(&b, "%s\n", styleHeader.Render(title))
	fmt.Fprintf(&b, "%s\n", styleSeparator.Render(strings.Repeat("─", m.width)))
	fmt.Fprintf(&b, "%s\n", m.viewport.View())
	fmt.Fprintf(&b, "%s\n", styleSeparator.Render(strings.Repeat("─", m.width)))

	if m.status != "idle" {
		label := "Thinking"
		if m.status == "tool_execution" {
			label = "Running tool"
		}
		fmt.Fprintf(&b, "%s\n", styleStatus.Render(fmt.Sprintf(" %s %s…", m.spinner.View(), label)))
	} else {
		fmt.Fprintf(&b, "%s\n", styleSystem.Render(" Ctrl+C to quit  ·  Alt+Enter for newline"))
	}

	b.WriteString(m.input.View())
	return b.String()
}

// writerAdapter pipes streaming text chunks into a buffered channel so they
// can be forwarded to the tea update loop one-by-one.
type writerAdapter struct {
	chunks chan<- string
}

func (w *writerAdapter) Write(p []byte) (int, error) {
	w.chunks <- string(p)
	return len(p), nil
}

// streamCmd starts the Groq streaming request in a goroutine and returns the
// first tea.Cmd in a recursive chain that drains one chunk per update tick.
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
