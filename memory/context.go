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

	// 5. Trim history to token budget and append
	history = c.trimToTokenBudget(history, userInput)
	messages = append(messages, history...)

	// 6. Append the user's current question as the final message
	messages = append(messages, Message{
		Role:    "user",
		Content: userInput,
	})

	return messages
}


// BuildFollowUp constructs the messages array for a follow-up after tool execution.
// Unlike Build(), it does NOT append a user message — the conversation already ends
// with a tool result message and the AI should respond to that directly.
func (c *ContextBuilder) BuildFollowUp() []Message {
	var messages []Message

	messages = append(messages, Message{
		Role:    "system",
		Content: c.buildSystemPrompt(),
	})

	history, _ := c.store.GetMessages(c.session.ID, 1000)
	totalCount, _ := c.store.CountMessages(c.session.ID)

	if totalCount > len(history) && c.session.Summary != "" {
		messages = append(messages, Message{
			Role:    "system",
			Content: "[Summary of earlier conversation: " + c.session.Summary + "]",
		})
	}

	history = c.trimToTokenBudget(history, "")
	messages = append(messages, history...)
	return messages
}

func (c *ContextBuilder) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are Goo, a powerful terminal AI assistant running on the user's Linux machine.\n")
	sb.WriteString("You can search the web, run shell commands, read/write files, manage tasks, and query GitHub.\n\n")
	sb.WriteString(fmt.Sprintf("Current date/time: %s\n", time.Now().Format("Monday, Jan 2 2006 15:04 MST")))
	sb.WriteString(fmt.Sprintf("Session ID: %s\n", c.session.ID))
	sb.WriteString("OS: Linux\n")

	if tasks := c.store.GetRecentTaskSummary(); tasks != "" {
		sb.WriteString(fmt.Sprintf("\nOpen tasks:\n%s\n", tasks))
	}
	if ghCtx := c.store.GetGitHubContext(c.session.ID); ghCtx != "" {
		sb.WriteString(fmt.Sprintf("\nGitHub context:\n%s\n", ghCtx))
	}
	if searchCtx := c.store.GetSearchContext(c.session.ID); searchCtx != "" {
		sb.WriteString(fmt.Sprintf("\nLatest search results:\n%s\n", searchCtx))
	}

	sb.WriteString(`
Available tools and when to use them:
- search_web: real-time info, news, prices, people, anything recent
- run_command: execute ANY shell command (apt install, git, cd && make, etc.)
- read_file: read a file's contents
- write_file: create or overwrite a file with new content
- find_files: locate files by name or content on the machine
- list_tasks / add_task / complete_task: manage the task list
- get_github_stats: GitHub contribution stats for a user
- get_github_prs: open pull requests for the configured GitHub user
- get_github_repos: list repositories for a GitHub user

Rules:
- For installs, code changes, or system ops — use run_command / write_file
- When the user says "cd /some/path and do X" — use run_command with that cwd
- Be proactive: chain multiple tool calls if needed to fully answer the user
- Never give up. If one approach fails, try another.
- Be concise but complete.
`)
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
