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

    "github.com/yourusername/goo/config"
    "github.com/yourusername/goo/memory"
)

const groqBaseURL = "https://api.groq.com/openai/v1"

type GroqClient struct {
    httpClient *http.Client
    model      string
}

func NewGroqClient() (*GroqClient, error) {
    // Key is decrypted here — never stored in the struct
    model := config.Get("general.default_model")
    if model == "" {
        model = "llama-3.3-70b-versatile"
    }
    return &GroqClient{
        httpClient: &http.Client{},
        model:      model,
    }, nil
}

type chatRequest struct {
    Model    string             `json:"model"`
    Messages []groqMessage      `json:"messages"`
    Stream   bool               `json:"stream"`
    MaxTok   int                `json:"max_tokens"`
}

type groqMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

// StreamChat sends a chat request to Groq and streams the response to out.
func (c *GroqClient) StreamChat(ctx context.Context, messages []memory.Message, out io.Writer) error {
    apiKey, err := config.GetAPIKey("groq")
    if err != nil {
        return fmt.Errorf("groq key not found: run 'goo config set-key groq'")
    }

    gMsgs := make([]groqMessage, len(messages))
    for i, m := range messages {
        gMsgs[i] = groqMessage{Role: m.Role, Content: m.Content}
    }

    body, err := json.Marshal(chatRequest{
        Model:    c.model,
        Messages: gMsgs,
        Stream:   true,
        MaxTok:   4096,
    })
    if err != nil {
        return err
    }

    req, err := http.NewRequestWithContext(ctx, "POST", groqBaseURL+"/chat/completions", bytes.NewReader(body))
    if err != nil {
        return err
    }
    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("groq API error %d: %s", resp.StatusCode, string(body))
    }

    // Parse SSE stream
    scanner := bufio.NewScanner(resp.Body)
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
                    Content string `json:"content"`
                } `json:"delta"`
                FinishReason string `json:"finish_reason"`
            } `json:"choices"`
        }
        if err := json.Unmarshal([]byte(data), &event); err != nil {
            continue
        }
        if len(event.Choices) > 0 {
            fmt.Fprint(out, event.Choices[0].Delta.Content)
        }
    }
    return scanner.Err()
}
