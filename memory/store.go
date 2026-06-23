package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kingjethro999/goo/config"
	"github.com/kingjethro999/goo/tools/tasks"
)

// Store manages conversation history using JSON files (no CGO dependency).
// Data is stored in ~/.config/goo/history.json
type Store struct {
	mu   sync.RWMutex
	path string
	data storeData
}

type storeData struct {
	Sessions []Session         `json:"sessions"`
	Messages []storedMessage   `json:"messages"`
	Context  map[string]string `json:"context"` // "sessionID:key" → value
}

type storedMessage struct {
	ID        int       `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	ToolName  string    `json:"tool_name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// NewStore opens (or creates) the history store.
func NewStore() (*Store, error) {
	dir := config.GooConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "history.json")
	s := &Store{
		path: path,
		data: storeData{Context: map[string]string{}},
	}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("loading history: %w", err)
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.data)
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

// NewSession creates a new session.
func (s *Store) NewSession(mode string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := Session{
		ID:        newUUID(),
		StartedAt: time.Now(),
		Mode:      mode,
	}
	s.data.Sessions = append(s.data.Sessions, sess)
	return &sess, s.save()
}

// GetSession retrieves a session by ID.
func (s *Store) GetSession(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.data.Sessions {
		if s.data.Sessions[i].ID == id {
			sess := s.data.Sessions[i]
			return &sess, nil
		}
	}
	return nil, fmt.Errorf("session not found: %s", id)
}

// ListSessions returns the most recent limit sessions ordered by start time desc.
func (s *Store) ListSessions(limit int) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessions := make([]*Session, len(s.data.Sessions))
	for i := range s.data.Sessions {
		sess := s.data.Sessions[i]
		sessions[i] = &sess
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

// SetSessionTitle sets the auto-generated title for a session.
func (s *Store) SetSessionTitle(sessionID, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Sessions {
		if s.data.Sessions[i].ID == sessionID {
			s.data.Sessions[i].Title = title
			return s.save()
		}
	}
	return nil
}

// UpdateSessionSummary stores the AI-generated summary for a session.
func (s *Store) UpdateSessionSummary(sessionID, summary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Sessions {
		if s.data.Sessions[i].ID == sessionID {
			s.data.Sessions[i].Summary = summary
			return s.save()
		}
	}
	return nil
}

// SaveMessage persists a message.
func (s *Store) SaveMessage(msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sm := storedMessage{
		ID:        len(s.data.Messages) + 1,
		SessionID: msg.SessionID,
		Role:      msg.Role,
		Content:   msg.Content,
		ToolName:  msg.ToolName,
		CreatedAt: time.Now(),
	}
	s.data.Messages = append(s.data.Messages, sm)
	return s.save()
}

// GetMessages returns the last limit messages for a session, oldest first.
func (s *Store) GetMessages(sessionID string, limit int) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var filtered []storedMessage
	for _, m := range s.data.Messages {
		if m.SessionID == sessionID {
			filtered = append(filtered, m)
		}
	}
	// Keep last `limit` messages
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	msgs := make([]Message, len(filtered))
	for i, m := range filtered {
		msgs[i] = Message{
			ID:        m.ID,
			SessionID: m.SessionID,
			Role:      m.Role,
			Content:   m.Content,
			ToolName:  m.ToolName,
			CreatedAt: m.CreatedAt,
		}
	}
	return msgs, nil
}

// GetMessagesPage returns messages. limit -N means "all except last N".
func (s *Store) GetMessagesPage(sessionID string, offset, limit int) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var filtered []storedMessage
	for _, m := range s.data.Messages {
		if m.SessionID == sessionID {
			filtered = append(filtered, m)
		}
	}
	if limit < 0 {
		end := len(filtered) + limit
		if end <= 0 {
			return nil, nil
		}
		filtered = filtered[:end]
	} else {
		if offset >= len(filtered) {
			return nil, nil
		}
		filtered = filtered[offset:]
		if limit < len(filtered) {
			filtered = filtered[:limit]
		}
	}
	msgs := make([]Message, len(filtered))
	for i, m := range filtered {
		msgs[i] = Message{
			ID:        m.ID,
			SessionID: m.SessionID,
			Role:      m.Role,
			Content:   m.Content,
			ToolName:  m.ToolName,
			CreatedAt: m.CreatedAt,
		}
	}
	return msgs, nil
}

// CountMessages returns the total number of messages in a session.
func (s *Store) CountMessages(sessionID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, m := range s.data.Messages {
		if m.SessionID == sessionID {
			count++
		}
	}
	return count, nil
}

// DeleteSession removes a session and all its messages.
func (s *Store) DeleteSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	newSessions := s.data.Sessions[:0]
	for _, sess := range s.data.Sessions {
		if sess.ID != sessionID {
			newSessions = append(newSessions, sess)
		}
	}
	s.data.Sessions = newSessions
	newMessages := s.data.Messages[:0]
	for _, m := range s.data.Messages {
		if m.SessionID != sessionID {
			newMessages = append(newMessages, m)
		}
	}
	s.data.Messages = newMessages
	return s.save()
}

// ClearAll removes all sessions and messages.
func (s *Store) ClearAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = storeData{Context: map[string]string{}}
	return s.save()
}

func contextKey(sessionID, key string) string {
	return sessionID + ":" + key
}

// SetContextValue stores a key/value pair for a session.
func (s *Store) SetContextValue(sessionID, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Context == nil {
		s.data.Context = map[string]string{}
	}
	s.data.Context[contextKey(sessionID, key)] = value
	return s.save()
}

// GetContextValue retrieves a stored context value.
func (s *Store) GetContextValue(sessionID, key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.Context == nil {
		return ""
	}
	return s.data.Context[contextKey(sessionID, key)]
}

// SetGitHubContext stores the latest GitHub context.
func (s *Store) SetGitHubContext(sessionID, context string) error {
	return s.SetContextValue(sessionID, "github", context)
}

// GetGitHubContext returns the stored GitHub context string.
func (s *Store) GetGitHubContext(sessionID string) string {
	return s.GetContextValue(sessionID, "github")
}

// SetSearchContext stores the latest search results.
func (s *Store) SetSearchContext(sessionID, context string) error {
	return s.SetContextValue(sessionID, "search", context)
}

// GetSearchContext returns the stored search context string.
func (s *Store) GetSearchContext(sessionID string) string {
	return s.GetContextValue(sessionID, "search")
}

func (s *Store) GetRecentTaskSummary() string {
	mgr := tasks.NewManager()
	stats, _ := mgr.Stats()
	if stats.Total == 0 {
		return ""
	}
	overdue, _ := mgr.List(tasks.TaskFilters{Overdue: true})
	urgent, _ := mgr.List(tasks.TaskFilters{Priority: "urgent", Status: "todo"})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tasks: %d total, %d todo, %d done", stats.Total, stats.Todo, stats.Done))
	if stats.Overdue > 0 {
		sb.WriteString(fmt.Sprintf(", %d OVERDUE", stats.Overdue))
		for _, t := range overdue {
			if t.DueDate != nil {
				sb.WriteString(fmt.Sprintf("\n  - [OVERDUE] %s (due %s)", t.Title, t.DueDate.Format("Jan 2")))
			}
		}
	}
	if len(urgent) > 0 {
		sb.WriteString("\nUrgent:")
		for _, t := range urgent {
			sb.WriteString(fmt.Sprintf("\n  - %s", t.Title))
		}
	}
	return sb.String()
}

// SetTaskSummary stores a task summary at global scope.
func (s *Store) SetTaskSummary(summary string) error {
	return s.SetContextValue("global", "task_summary", summary)
}

// ExportSession returns a markdown representation of a session.
func (s *Store) ExportSession(sessionID string) (string, error) {
	sess, err := s.GetSession(sessionID)
	if err != nil {
		return "", err
	}
	messages, err := s.GetMessages(sessionID, 10000)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	title := sess.Title
	if title == "" {
		title = sess.ID
	}
	sb.WriteString(fmt.Sprintf("# Session: %s\n", title))
	sb.WriteString(fmt.Sprintf("*Started: %s*\n\n---\n\n", sess.StartedAt.Format(time.RFC1123)))
	for _, m := range messages {
		switch m.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("**You:** %s\n\n", m.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("**Goo:** %s\n\n", m.Content))
		case "tool":
			sb.WriteString(fmt.Sprintf("> *Tool call: %s*\n\n", m.ToolName))
		}
	}
	return sb.String(), nil
}

// GenerateTitleHeuristic creates a short session title from the first message.
func GenerateTitleHeuristic(firstMessage string) string {
	words := strings.Fields(firstMessage)
	if len(words) > 8 {
		words = words[:8]
	}
	title := strings.Join(words, " ")
	if len(title) > 50 {
		title = title[:47] + "..."
	}
	return title
}
