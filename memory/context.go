package memory

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	MaxTokens         = 6000
	ApproxCharsPerTok = 4
)

// ContextBuilder constructs the messages array for each AI request.
type ContextBuilder struct {
	store   *Store
	session *Session
}

// NewContextBuilder creates a context builder for the given session.
func NewContextBuilder(session *Session, store *Store) *ContextBuilder {
	return &ContextBuilder{store: store, session: session}
}

// Build constructs the messages array for the next AI request.
func (c *ContextBuilder) Build(userInput string) []Message {
	var messages []Message

	// 1. System prompt
	messages = append(messages, Message{
		Role:    "system",
		Content: c.buildSystemPrompt(),
	})

	// 2. Load history
	history, _ := c.store.GetMessages(c.session.ID, 1000)
	totalCount, _ := c.store.CountMessages(c.session.ID)

	// 3. If we have a summary and trimmed history, prepend it
	if totalCount > len(history) && c.session.Summary != "" {
		messages = append(messages, Message{
			Role:    "system",
			Content: "[Summary of earlier conversation: " + c.session.Summary + "]",
		})
	}

	// 4. Detect topic shift
	if detectTopicShift(userInput, history) {
		messages = append(messages, Message{
			Role:    "system",
			Content: "[Note: The user appears to be changing topic. Acknowledge the shift naturally if helpful, but don't call it out explicitly every time.]",
		})
	}

	// 5. Trim history to token budget
	history = c.trimToTokenBudget(history, userInput)
	messages = append(messages, history...)
	return messages
}

func (c *ContextBuilder) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are Goo, a terminal AI assistant. Be concise, direct, and helpful.\n\n")
	sb.WriteString(fmt.Sprintf("Current date/time: %s\n", time.Now().Format("Monday, Jan 2 2006 15:04 MST")))
	sb.WriteString(fmt.Sprintf("Session ID: %s\n", c.session.ID))

	if tasks := c.store.GetRecentTaskSummary(); tasks != "" {
		sb.WriteString(fmt.Sprintf("\nOpen tasks:\n%s\n", tasks))
	}
	if ghCtx := c.store.GetGitHubContext(c.session.ID); ghCtx != "" {
		sb.WriteString(fmt.Sprintf("\nGitHub context:\n%s\n", ghCtx))
	}
	if searchCtx := c.store.GetSearchContext(c.session.ID); searchCtx != "" {
		sb.WriteString(fmt.Sprintf("\nLatest search results:\n%s\n", searchCtx))
	}

	sb.WriteString("\nYou have access to tools: search_web, list_tasks, add_task, complete_task, get_github_prs, get_github_stats.\n")
	sb.WriteString("When uncertain, ask ONE focused clarifying question.\n")
	sb.WriteString("Never give up — always provide a best-effort answer.\n")
	return sb.String()
}

func (c *ContextBuilder) trimToTokenBudget(messages []Message, newInput string) []Message {
	budget := MaxTokens - len(newInput)/ApproxCharsPerTok - 500
	var total int
	var result []Message

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

// RecentMessages returns the last 10 messages for follow-up analysis.
func (c *ContextBuilder) RecentMessages() []Message {
	msgs, _ := c.store.GetMessages(c.session.ID, 10)
	return msgs
}

// --- topic shift detection ---

func detectTopicShift(newMessage string, recentMessages []Message) bool {
	if len(recentMessages) < 3 {
		return false
	}
	newKeywords := extractKeywords(newMessage)
	var recentText strings.Builder
	start := len(recentMessages) - 3
	if start < 0 {
		start = 0
	}
	for _, m := range recentMessages[start:] {
		recentText.WriteString(m.Content)
		recentText.WriteByte(' ')
	}
	recentKeywords := extractKeywords(recentText.String())
	overlap := keywordOverlap(newKeywords, recentKeywords)

	continuationPhrases := []string{"also", "and", "what about", "additionally", "follow up", "another"}
	lower := strings.ToLower(newMessage)
	for _, phrase := range continuationPhrases {
		if strings.HasPrefix(lower, phrase) {
			return false
		}
	}
	return overlap < 0.10
}

func extractKeywords(text string) map[string]bool {
	stopwords := map[string]bool{
		"the": true, "and": true, "for": true, "that": true,
		"this": true, "with": true, "from": true, "have": true,
		"what": true, "how": true, "when": true, "where": true,
		"will": true, "your": true, "just": true, "about": true,
	}
	words := strings.Fields(strings.ToLower(text))
	keywords := map[string]bool{}
	for _, w := range words {
		w = strings.Trim(w, ".,!?;:\"'")
		if len(w) > 4 && !stopwords[w] {
			keywords[w] = true
		}
	}
	return keywords
}

func keywordOverlap(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	shared := 0
	for k := range a {
		if b[k] {
			shared++
		}
	}
	smaller := len(a)
	if len(b) < smaller {
		smaller = len(b)
	}
	return float64(shared) / float64(smaller)
}

// ChatClient is the interface for AI providers.
type ChatClient interface {
	StreamChat(ctx context.Context, messages []Message, out io.Writer) error
	Complete(ctx context.Context, prompt, model string) (string, error)
}
