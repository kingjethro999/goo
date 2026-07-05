package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kingjethro999/goo/config"
	"github.com/kingjethro999/goo/memory"
)

// ─── Providers ────────────────────────────────────────────────────────────────

type Provider string

const (
	ProviderGroq     Provider = "groq"
	ProviderOpenAI   Provider = "openai"
	ProviderClaude   Provider = "claude"
	ProviderDeepSeek Provider = "deepseek"
)

// ProviderDefaults maps provider → (baseURL, default model)
var ProviderDefaults = map[Provider]struct {
	BaseURL      string
	DefaultModel string
}{
	ProviderGroq:     {"https://api.groq.com/openai/v1", "openai/gpt-oss-120b"},
	ProviderOpenAI:   {"https://api.openai.com/v1", "gpt-4o-mini"},
	ProviderClaude:   {"https://api.anthropic.com/v1", "claude-3-5-sonnet-20241022"},
	ProviderDeepSeek: {"https://api.deepseek.com/v1", "deepseek-chat"},
}

// ─── GroqClient ───────────────────────────────────────────────────────────────

// GroqClient talks to Groq (and other OpenAI-compatible providers) with streaming support.
type GroqClient struct {
	httpClient *http.Client
	model      string
	provider   Provider
	baseURL    string
}

// NewGroqClient creates a client using the configured model & provider.
func NewGroqClient() (*GroqClient, error) {
	// Determine provider
	providerStr := config.Get("general.default_provider")
	provider := Provider(providerStr)
	if provider == "" {
		provider = ProviderGroq
	}

	defaults, ok := ProviderDefaults[provider]
	if !ok {
		provider = ProviderGroq
		defaults = ProviderDefaults[ProviderGroq]
	}

	model := config.Get("general.default_model")
	if model == "" {
		model = defaults.DefaultModel
	}

	// Map deprecated models to recommended replacement
	if provider == ProviderGroq && (model == "llama-3.3-70b-versatile" || model == "llama-3.1-70b-versatile" || model == "llama3-groq-70b-8192-tool-use-preview") {
		model = "openai/gpt-oss-120b"
	}

	return &GroqClient{
		httpClient: &http.Client{},
		model:      model,
		provider:   provider,
		baseURL:    defaults.BaseURL,
	}, nil
}

// Model returns the active model name.
func (c *GroqClient) Model() string { return c.model }

// SetModel changes the model for the current session.
func (c *GroqClient) SetModel(model string) { c.model = model }

// Provider returns the active provider.
func (c *GroqClient) Provider() Provider { return c.provider }

// ─── API types ────────────────────────────────────────────────────────────────

type StreamOptions struct {
	ReasoningEffort string `json:"reasoning_effort,omitempty"` // "high", "medium", "low", "none"
	ReasoningFormat string `json:"reasoning_format,omitempty"` // "parsed", "raw", "hidden"
	Model           string `json:"model,omitempty"`
}

type chatRequest struct {
	Model           string        `json:"model"`
	Messages        []groqMessage `json:"messages"`
	Stream          bool          `json:"stream"`
	MaxTok          int           `json:"max_completion_tokens,omitempty"`
	Temperature     float64       `json:"temperature,omitempty"`
	TopP            float64       `json:"top_p,omitempty"`
	Tools           []Tool        `json:"tools,omitempty"`
	Stop            []string      `json:"stop,omitempty"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
	ReasoningFormat string        `json:"reasoning_format,omitempty"`
}

type contentPart struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	ImageURL *contentImageURL `json:"image_url,omitempty"`
}

type contentImageURL struct {
	URL string `json:"url"`
}

type groqMessage struct {
	Role       string          `json:"role"`
	Content    interface{}     `json:"content"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
	ToolCalls  []toolCallEntry `json:"tool_calls,omitempty"`
}

// toolCallEntry represents a tool call made by the assistant in the message history.
type toolCallEntry struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ─── Public methods ───────────────────────────────────────────────────────────

// StreamChat sends a chat request and streams the response to out.
func (c *GroqClient) StreamChat(ctx context.Context, messages []memory.Message, out io.Writer) error {
	_, err := c.streamChatInternal(ctx, messages, out, nil, StreamOptions{})
	return err
}

// StreamChatWithTools sends a chat request with tool definitions.
// Returns a *ToolCall if the model wants to invoke a tool, nil otherwise.
func (c *GroqClient) StreamChatWithTools(ctx context.Context, messages []memory.Message, out io.Writer, tools []Tool) (*ToolCall, error) {
	return c.StreamChatWithToolsEx(ctx, messages, out, tools, StreamOptions{})
}

// StreamChatWithToolsEx sends a chat request with tool definitions and extra streaming options.
func (c *GroqClient) StreamChatWithToolsEx(ctx context.Context, messages []memory.Message, out io.Writer, tools []Tool, opts StreamOptions) (*ToolCall, error) {
	if c.provider == ProviderClaude {
		return c.streamChatClaude(ctx, messages, out, tools)
	}
	return c.streamChatInternal(ctx, messages, out, tools, opts)
}

// Complete sends a one-shot non-streaming request and returns the full response.
func (c *GroqClient) Complete(ctx context.Context, prompt, model string) (string, error) {
	if model == "" {
		model = c.model
	}
	apiKey, err := c.getAPIKey()
	if err != nil {
		return "", err
	}

	gMsgs := []groqMessage{
		{Role: "user", Content: prompt},
	}
	body, err := json.Marshal(chatRequest{
		Model:    model,
		Messages: gMsgs,
		Stream:   false,
		MaxTok:   512,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	c.setAuthHeader(req, apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return result.Choices[0].Message.Content, nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (c *GroqClient) getAPIKey() (string, error) {
	key, err := config.GetAPIKey(string(c.provider))
	if err != nil {
		return "", fmt.Errorf("%s key not found: run 'goo config set-key %s'", c.provider, c.provider)
	}
	return key, nil
}

func (c *GroqClient) setAuthHeader(req *http.Request, apiKey string) {
	switch c.provider {
	case ProviderClaude:
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	default:
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

func (c *GroqClient) streamChatInternal(ctx context.Context, messages []memory.Message, out io.Writer, tools []Tool, opts StreamOptions) (*ToolCall, error) {
	apiKey, err := c.getAPIKey()
	if err != nil {
		return nil, err
	}

	maxTok := config.GetInt("ai.max_tokens")
	if maxTok == 0 || maxTok > 2048 {
		maxTok = 2048
	}

	modelToUse := c.model
	if opts.Model != "" {
		modelToUse = opts.Model
	}

	gMsgs := make([]groqMessage, 0, len(messages))
	for _, m := range messages {
		var contentVal interface{} = m.Content
		if m.ImageURL != "" {
			contentVal = []contentPart{
				{Type: "text", Text: m.Content},
				{Type: "image_url", ImageURL: &contentImageURL{URL: m.ImageURL}},
			}
		}

		gm := groqMessage{
			Role:       m.Role,
			Content:    contentVal,
			ToolCallID: m.ToolCallID,
		}
		if m.Role == "assistant" && m.ToolCallID != "" {
			gm.Content = ""
			entry := toolCallEntry{ID: m.ToolCallID, Type: "function"}
			entry.Function.Name = m.ToolName
			entry.Function.Arguments = "{}"
			gm.ToolCalls = []toolCallEntry{entry}
		}
		gMsgs = append(gMsgs, gm)
	}

	reqBody := chatRequest{
		Model:           modelToUse,
		Messages:        gMsgs,
		Stream:          true,
		MaxTok:          maxTok,
		Tools:           tools,
		ReasoningEffort: opts.ReasoningEffort,
		ReasoningFormat: opts.ReasoningFormat,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req, apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("groq API error %d: %s", resp.StatusCode, string(b))
	}

	// Parse SSE stream
	scanner := bufio.NewScanner(resp.Body)
	var pendingToolCall *ToolCall
	var toolArgsBuilder strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event struct {
			Choices []struct {
				Delta struct {
					Content   string     `json:"content"`
					ToolCalls []toolCall `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if len(event.Choices) == 0 {
			continue
		}

		choice := event.Choices[0]

		if choice.Delta.Content != "" {
			fmt.Fprint(out, choice.Delta.Content)
		}

		if len(choice.Delta.ToolCalls) > 0 {
			tc := choice.Delta.ToolCalls[0]
			if pendingToolCall == nil {
				pendingToolCall = &ToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
				}
			}
			toolArgsBuilder.WriteString(tc.Function.Arguments)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if pendingToolCall != nil {
		pendingToolCall.Arguments = json.RawMessage(toolArgsBuilder.String())
	}

	return pendingToolCall, nil
}

// ─── Claude-specific streaming ─────────────────────────────────────────────────

func (c *GroqClient) streamChatClaude(ctx context.Context, messages []memory.Message, out io.Writer, tools []Tool) (*ToolCall, error) {
	apiKey, err := c.getAPIKey()
	if err != nil {
		return nil, err
	}

	// Build Claude message format (system prompt separate)
	var systemContent string
	var claudeMsgs []map[string]interface{}
	for _, m := range messages {
		if m.Role == "system" {
			systemContent += m.Content + "\n"
			continue
		}
		role := m.Role
		if role == "tool" {
			role = "user"
		}
		claudeMsgs = append(claudeMsgs, map[string]interface{}{
			"role":    role,
			"content": m.Content,
		})
	}

	maxTok := config.GetInt("ai.max_tokens")
	if maxTok == 0 || maxTok > 2048 {
		maxTok = 2048
	}

	payload := map[string]interface{}{
		"model":      c.model,
		"max_tokens": maxTok,
		"stream":     true,
		"messages":   claudeMsgs,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	if systemContent != "" {
		payload["system"] = strings.TrimSpace(systemContent)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("claude API error %d: %s", resp.StatusCode, string(b))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if event.Type == "content_block_delta" && event.Delta.Text != "" {
			fmt.Fprint(out, event.Delta.Text)
		}
	}
	return nil, scanner.Err()
}

// ─── Tool definitions ─────────────────────────────────────────────────────────

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Tool is a function or remote MCP definition for Groq tool calling.
type Tool struct {
	Type        string        `json:"type"`
	Function    *ToolFunction `json:"function,omitempty"`
	ConnectorID string        `json:"connector_id,omitempty"`
	ServerLabel string        `json:"server_label,omitempty"`
}

// NewFunctionTool creates a standard function tool.
func NewFunctionTool(name, description string, params json.RawMessage) Tool {
	return Tool{
		Type: "function",
		Function: &ToolFunction{
			Name:        name,
			Description: description,
			Parameters:  params,
		},
	}
}

// NewRemoteMCPTool creates a Groq remote MCP connector tool.
func NewRemoteMCPTool(connectorID, serverLabel string) Tool {
	return Tool{
		Type:        "mcp",
		ConnectorID: connectorID,
		ServerLabel: serverLabel,
	}
}

// ToolFunction describes a callable function.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall represents a tool invocation returned by the AI.
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

// SearchWebTool is the Groq tool definition for web search.
var SearchWebTool = Tool{
	Type: "function",
	Function: &ToolFunction{
		Name:        "search_web",
		Description: "Search the web for current information. Use when the user asks about recent events, news, prices, or anything that may not be in training data.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The search query"
				}
			},
			"required": ["query"]
		}`),
	},
}

// FormatToolAction creates a concise, human-readable action description with icons and target line ranges
// matching top agentic CLI standards (e.g. Antigravity, Cursor, Claude Code).
func FormatToolAction(call *ToolCall) string {
	if call == nil {
		return "Thinking..."
	}
	switch call.Name {
	case "read_file":
		var args struct {
			Path      string `json:"path"`
			StartLine int    `json:"start_line"`
			EndLine   int    `json:"end_line"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		path := args.Path
		if path == "" {
			path = "file"
		}
		if args.StartLine > 0 && args.EndLine > 0 {
			return fmt.Sprintf("Analyzed 📄 %s #L%d-%d", path, args.StartLine, args.EndLine)
		} else if args.StartLine > 0 {
			return fmt.Sprintf("Analyzed 📄 %s #L%d+", path, args.StartLine)
		}
		return fmt.Sprintf("Analyzed 📄 %s", path)

	case "write_file":
		var args struct {
			Path      string `json:"path"`
			StartLine int    `json:"start_line"`
			EndLine   int    `json:"end_line"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		path := args.Path
		if path == "" {
			path = "file"
		}
		if args.StartLine > 0 && args.EndLine > 0 {
			return fmt.Sprintf("Updated ✏️ %s #L%d-%d", path, args.StartLine, args.EndLine)
		}
		return fmt.Sprintf("Updated ✏️ %s", path)

	case "list_dir":
		var args struct {
			Dir string `json:"dir"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		dir := args.Dir
		if dir == "" {
			dir = "."
		}
		return fmt.Sprintf("Explored 📁 %s", dir)

	case "grep_search":
		var args struct {
			Query string `json:"query"`
			Path  string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		q := args.Query
		if len(q) > 30 {
			q = q[:27] + "..."
		}
		p := args.Path
		if p == "" {
			p = "."
		}
		return fmt.Sprintf("Searched 🔍 %q in %s", q, p)

	case "find_files":
		var args struct {
			Pattern string `json:"pattern"`
			Dir     string `json:"dir"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		return fmt.Sprintf("Found 🔍 pattern %q in %s", args.Pattern, args.Dir)

	case "run_command":
		var args struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		cmdStr := args.Command
		if len(cmdStr) > 40 {
			cmdStr = cmdStr[:37] + "..."
		}
		return fmt.Sprintf("Run ⚡ %s", cmdStr)

	case "delete_file":
		var args struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		return fmt.Sprintf("Deleted 🗑 %s", args.Path)

	case "search_web":
		var args struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		return fmt.Sprintf("Web searched 🌐 %q", args.Query)

	default:
		return fmt.Sprintf("Executed ⚙ %s", call.Name)
	}
}
