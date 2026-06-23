package memory

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Session represents a single conversation session.
type Session struct {
	ID        string
	StartedAt time.Time
	Mode      string
	Title     string
	Summary   string
}

// Message is a single conversation turn stored in history.
type Message struct {
	ID         int
	Role       string // "system" | "user" | "assistant" | "tool"
	Content    string
	ToolName   string
	ToolCallID string
	SessionID  string
	CreatedAt  time.Time
}

// newUUID generates a random UUID v4-style hex string.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
