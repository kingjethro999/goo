package memory

import (
	"context"
	"fmt"
	"strings"
)

// SummariseSession generates a summary for older messages in a session to
// maintain context when the conversation gets long.
func SummariseSession(sessionID string, store *Store, client ChatClient) error {
	// Get all messages older than the last 20
	messages, err := store.GetMessagesPage(sessionID, 0, -20)
	if err != nil || len(messages) == 0 {
		return err
	}

	var transcript strings.Builder
	for _, m := range messages {
		if m.Role == "user" || m.Role == "assistant" {
			transcript.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
		}
	}

	summaryPrompt := fmt.Sprintf(
		`Summarise the following conversation in 3-5 sentences. Focus on:
- The main topics discussed
- Key decisions or conclusions reached
- Any tasks, files, or repos mentioned

Transcript:
%s`, transcript.String())

	summary, err := client.Complete(context.Background(), summaryPrompt, "llama-3.3-70b-versatile")
	if err != nil {
		return err
	}

	return store.UpdateSessionSummary(sessionID, strings.TrimSpace(summary))
}
