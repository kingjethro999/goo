package memory

import (
    "fmt"
    "strings"
    "time"
)

const (
    MaxTokens         = 6000   // conservative limit for context window
    ApproxCharsPerTok = 4
)

type ContextBuilder struct {
    store      *Store
    session    *Session
    systemBase string
}

func NewContextBuilder(session *Session, store *Store) *ContextBuilder {
    return &ContextBuilder{
        store:   store,
        session: session,
    }
}

// Build constructs the messages array for the next AI request.
func (c *ContextBuilder) Build(userInput string) []Message {
    messages := []Message{}

    // 1. System prompt
    messages = append(messages, Message{
        Role:    "system",
        Content: c.buildSystemPrompt(),
    })

    // 2. Load history
    history, _ := c.store.GetMessages(c.session.ID, 100)

    // 3. Trim to fit token budget
    history = c.trimToTokenBudget(history, userInput)

    // 4. If we had to trim, prepend a summary
    if len(history) < c.totalMessageCount() {
        summary := c.session.Summary
        if summary != "" {
            messages = append(messages, Message{
                Role:    "system",
                Content: fmt.Sprintf("[Earlier conversation summary: %s]", summary),
            })
        }
    }

    messages = append(messages, history...)
    return messages
}

func (c *ContextBuilder) buildSystemPrompt() string {
    var sb strings.Builder
    sb.WriteString("You are Goo, a terminal AI assistant. Be concise, direct, and helpful.\n\n")
    sb.WriteString(fmt.Sprintf("Current date/time: %s\n", time.Now().Format("Monday, Jan 2 2006 15:04 MST")))
    sb.WriteString(fmt.Sprintf("Session ID: %s\n", c.session.ID))

    // Inject tool context if available
    if tasks := c.store.GetRecentTaskSummary(); tasks != "" {
        sb.WriteString(fmt.Sprintf("\nOpen tasks:\n%s\n", tasks))
    }
    if ghCtx := c.store.GetGitHubContext(c.session.ID); ghCtx != "" {
        sb.WriteString(fmt.Sprintf("\nGitHub context:\n%s\n", ghCtx))
    }

    sb.WriteString("\nWhen uncertain, ask ONE focused clarifying question.\n")
    sb.WriteString("Never give up — always provide a best-effort answer.\n")
    return sb.String()
}

func (c *ContextBuilder) trimToTokenBudget(messages []Message, newInput string) []Message {
    budget := MaxTokens - len(newInput)/ApproxCharsPerTok - 500 // reserve for system + new input
    var total int
    var result []Message

    // Walk from newest to oldest, keep until budget runs out
    for i := len(messages) - 1; i >= 0; i-- {
        tokens := len(messages[i].Content) / ApproxCharsPerTok
        if total+tokens > budget {
            break
        }
        total += tokens
        result = append([]Message{messages[i]}, result...)
    }
    return result
}

func (c *ContextBuilder) totalMessageCount() int {
    count, _ := c.store.CountMessages(c.session.ID)
    return count
}

func (c *ContextBuilder) RecentMessages() []Message {
    msgs, _ := c.store.GetMessages(c.session.ID, 10)
    return msgs
}
