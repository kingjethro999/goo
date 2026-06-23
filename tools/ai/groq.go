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

const groqBaseURL = "https://api.groq.com/openai/v1"

// GroqClient talks to the Groq AI API with streaming support.
type GroqClient struct {
	httpClient *http.Client
	model      string
}

// NewGroqClient creates a Groq client using the configured model.
func NewGroqClient() (*GroqClient, error) {
	model := config.Get("general.default_model")
	if model == "" {
		model = "llama-3.3-70b-versatile"
	}
	return &GroqClient{
		httpClient: &http.Client{},
		model:      model,
	}, nil
}

// Model returns the active model name.
func (c *GroqClient) Model() string { return c.model }

// SetModel changes the model for the current session.
func (c *GroqClient) SetModel(model string) { c.model = model }

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
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
}

// StreamChat sends a chat request and streams the response to out.
func (c *GroqClient) StreamChat(ctx context.Context, messages []memory.Message, out io.Writer) error {
	_, err := c.streamChatInternal(ctx, messages, out, nil)
	return err
}

// StreamChatWithTools sends a chat request with tool definitions.
// Returns a *ToolCall if the model wants to invoke a tool, nil otherwise.
func (c *GroqClient) StreamChatWithTools(ctx context.Context, messages []memory.Message, out io.Writer, tools []Tool) (*ToolCall, error) {
	return c.streamChatInternal(ctx, messages, out, tools)
}

// Complete sends a one-shot non-streaming request and returns the full response.
func (c *GroqClient) Complete(ctx context.Context, prompt, model string) (string, error) {
	if model == "" {
		model = c.model
	}
	apiKey, err := config.GetAPIKey("groq")
	if err != nil {
		return "", fmt.Errorf("groq key not found: run 'goo config set-key groq'")
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

	req, err := http.NewRequestWithContext(ctx, "POST", groqBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
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

func (c *GroqClient) streamChatInternal(ctx context.Context, messages []memory.Message, out io.Writer, tools []Tool) (*ToolCall, error) {
	apiKey, err := config.GetAPIKey("groq")
	if err != nil {
		return nil, fmt.Errorf("groq key not found: run 'goo config set-key groq'")
	}

	maxTok := config.GetInt("ai.max_tokens")
	if maxTok == 0 {
		maxTok = 4096
	}

	gMsgs := make([]groqMessage, len(messages))
	for i, m := range messages {
		gMsgs[i] = groqMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
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

	req, err := http.NewRequestWithContext(ctx, "POST", groqBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
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

		// Normal text streaming
		if choice.Delta.Content != "" {
			fmt.Fprint(out, choice.Delta.Content)
		}

		// Tool call streaming
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

// --- Tool definitions ---

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
