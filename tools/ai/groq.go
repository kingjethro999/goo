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
	ProviderGroq:     {"https://api.groq.com/openai/v1", "llama-3.3-70b-versatile"},
	ProviderOpenAI:   {"https://api.openai.com/v1", "gpt-4o-mini"},
	ProviderClaude:   {"https://api.anthropic.com/v1", "claude-3-5-sonnet-20241022"},
	ProviderDeepSeek: {"https://api.deepseek.com/v1", "deepseek-chat"},
}

const groqBaseURL = "https://api.groq.com/openai/v1"

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

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	MaxTok      int           `json:"max_completion_tokens"`
	Temperature float64       `json:"temperature,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
	Tools       []Tool        `json:"tools,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
}

type groqMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content"`
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
	_, err := c.streamChatInternal(ctx, messages, out, nil)
	return err
}

// StreamChatWithTools sends a chat request with tool definitions.
// Returns a *ToolCall if the model wants to invoke a tool, nil otherwise.
func (c *GroqClient) StreamChatWithTools(ctx context.Context, messages []memory.Message, out io.Writer, tools []Tool) (*ToolCall, error) {
	// Claude has a different API — handle separately
	if c.provider == ProviderClaude {
		return c.streamChatClaude(ctx, messages, out, tools)
	}
	return c.streamChatInternal(ctx, messages, out, tools)
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

func (c *GroqClient) streamChatInternal(ctx context.Context, messages []memory.Message, out io.Writer, tools []Tool) (*ToolCall, error) {
	apiKey, err := c.getAPIKey()
	if err != nil {
		return nil, err
	}

	maxTok := config.GetInt("ai.max_tokens")
	if maxTok == 0 {
		maxTok = 4096
	}

	gMsgs := make([]groqMessage, 0, len(messages))
	for _, m := range messages {
		gm := groqMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		// For assistant messages that represent a tool call, attach the tool_calls
		// array so Groq can match the subsequent tool result message.
		if m.Role == "assistant" && m.ToolCallID != "" {
			gm.Content = "" // must be empty or null for tool-call assistant messages
			entry := toolCallEntry{ID: m.ToolCallID, Type: "function"}
			entry.Function.Name = m.ToolName
			entry.Function.Arguments = "{}"
			gm.ToolCalls = []toolCallEntry{entry}
		}
		gMsgs = append(gMsgs, gm)
	}

	body, err := json.Marshal(chatRequest{
		Model:    c.model,
		Messages: gMsgs,
		Stream:   true,
		MaxTok:   maxTok,
		Tools:    tools,
	})
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
	if maxTok == 0 {
		maxTok = 4096
	}

	payload := map[string]interface{}{
		"model":      c.model,
		"max_tokens": maxTok,
		"stream":     true,
		"messages":   claudeMsgs,
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

// Tool is a function definition for Groq tool calling.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
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
	Function: ToolFunction{
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
