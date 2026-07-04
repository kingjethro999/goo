# Goo AI CLI — updates.md
> Detailed integration guide for versions v0.2 through v1.0.
> Each section covers what to build, how it integrates with existing systems, and the exact code patterns to follow.

---

## v0.2 — Memory & Follow-ups

The goal of v0.2 is to make Goo feel like it actually *remembers* you. By the end of this version, a user can close their terminal, come back the next day, and pick up exactly where they left off — with the AI having full awareness of prior context.

**Prerequisites:** v0.1 must be complete. `goo ask` and `goo chat` must be working with basic Groq streaming. The SQLite `history.db` file must already be created by `memory/store.go`.

---

### Session persistence and resume (`goo history`)

**What it does:**
Every `goo chat` session is assigned a UUID and written to `history.db`. Sessions have a human-readable title (auto-generated from the first user message), a start timestamp, the mode (`chat`, `ask`, etc.), and an optional summary. The `goo history` command surfaces these to the user and allows full resumption.

**Files touched:**
- `memory/store.go` — add `ListSessions`, `GetSession`, `GetMessages`
- `memory/schema.sql` — already defined in CLAUDE.md, run migration
- `cmd/history.go` — new command with subcommands
- `core/session.go` — extend `RunChatSession` to accept an existing session

**Integration steps:**

Step 1 — on every new `goo chat` invocation, create a session row:

```go
// memory/store.go
func (s *Store) NewSession(mode string) (*Session, error) {
    id := uuid.New().String()
    now := time.Now()
    _, err := s.db.Exec(
        `INSERT INTO sessions (id, started_at, mode) VALUES (?, ?, ?)`,
        id, now, mode,
    )
    if err != nil {
        return nil, err
    }
    return &Session{ID: id, StartedAt: now, Mode: mode}, nil
}
```

Step 2 — auto-generate a title from the first user message. Do this lazily: after the first message is saved, run a cheap heuristic (first 8 words, truncated to 50 chars) or a one-shot Groq call with `max_tokens: 8` asking for a 4-word title. The Groq call approach is better UX; use the heuristic as fallback if the key isn't set yet.

```go
// memory/store.go
func (s *Store) SetSessionTitle(sessionID, title string) error {
    _, err := s.db.Exec(
        `UPDATE sessions SET title = ? WHERE id = ?`, title, sessionID,
    )
    return err
}

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
```

Step 3 — implement `goo history show`:

```go
// cmd/history.go
var historyShowCmd = &cobra.Command{
    Use:   "show",
    Short: "List all past sessions",
    RunE: func(cmd *cobra.Command, args []string) error {
        store, _ := memory.NewStore()
        sessions, err := store.ListSessions(50)
        if err != nil {
            return err
        }
        // render as lipgloss table: ID (truncated) | Started | Title | Msgs
        renderer.PrintSessionTable(sessions)
        return nil
    },
}
```

Step 4 — implement `goo history resume <session-id>`. This is the critical path:

```go
var historyResumeCmd = &cobra.Command{
    Use:   "resume <session-id>",
    Short: "Resume a past session with full context",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        store, _ := memory.NewStore()
        session, err := store.GetSession(args[0])
        if err != nil {
            return fmt.Errorf("session not found: %s", args[0])
        }
        // Load last message for the resume greeting
        msgs, _ := store.GetMessages(session.ID, 1)
        renderer.PrintResumeHeader(session, msgs)
        // Launch chat session with existing session — NOT a new one
        return core.RunChatSession(session, store)
    },
}
```

The `RunChatSession` function already accepts a `*Session` — it will load existing history into the context window via `ContextBuilder.Build()`. No changes needed there. The only new behaviour is the **resume greeting**: inject a special system message at the start of the resumed session:

```
[System note: This is a resumed session. The user is continuing from a previous conversation.
 Last user message was: "{last_user_message}". Greet them briefly and offer to continue.]
```

This makes the AI's first response feel aware and warm, not like it forgot everything.

Step 5 — `goo history clear` deletes all rows from `sessions` and `messages`. Prompt for confirmation first. `goo history export <id>` walks all messages for that session and writes a markdown file to the current directory.

```go
// Export format
func (s *Store) ExportSession(sessionID string) (string, error) {
    session, _ := s.GetSession(sessionID)
    messages, _ := s.GetMessages(sessionID, 10000)
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("# Session: %s\n", session.Title))
    sb.WriteString(fmt.Sprintf("*Started: %s*\n\n---\n\n", session.StartedAt.Format(time.RFC1123)))
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
```

**How it connects to the rest of the system:**
Session IDs are the foreign key for everything — messages, tool call logs, and the GitHub/task context injected into the context window. Getting this right in v0.2 means v0.3 tool integrations just need to write their results to the same store with a `session_id` and they automatically appear in context.

---

### Follow-up detector

**What it does:**
After every AI response, a lightweight analyser scans the text for signals that the conversation needs to continue — unanswered questions, uncertainty, shallow answers. It surfaces hints to the user rather than letting the conversation dead-end.

**Files touched:**
- `tools/ai/followup.go` — already scaffolded in CLAUDE.md, now fully implement
- `core/session.go` — wire analyser output into the chat loop render step

**Integration steps:**

Step 1 — extend the signal types. The CLAUDE.md scaffold has three types. Add two more:

```go
const (
    SignalQuestion   = "question"    // AI ended with a question
    SignalUncertain  = "uncertainty" // AI expressed doubt
    SignalShallow    = "shallow"     // response too short for question complexity
    SignalReference  = "reference"   // AI mentioned something that could be acted on
    SignalConflict   = "conflict"    // AI response conflicts with recent history
)
```

`SignalReference` fires when the AI mentions a task name, filename, repo, or URL that doesn't exist in the current context. Example: AI says "you could create a task for that" — detect the implication and offer `Do you want me to add that task now?`

`SignalConflict` fires when the AI's current response contradicts something said in the last 5 messages. Use simple keyword overlap: if the AI said "X is not possible" and a prior assistant message said "X works well for this", surface a gentle note.

Step 2 — render follow-up signals distinctly from the main response. In `core/renderer.go`:

```go
func (r *Renderer) PrintFollowUpHint(signal FollowUpSignal) {
    // Auto-prompt signals render inline, below the response, in magenta
    // Manual signals render as a subtle one-liner the user can ignore
    switch signal.AutoPrompt {
    case true:
        // Print the AI's own question, highlighted
        style := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6")).Italic(true)
        fmt.Println(style.Render("  ↳ " + extractQuestion(signal.Suggestion)))
    case false:
        // Print a dim suggestion
        style := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
        fmt.Println(style.Render("  · " + signal.Suggestion))
    }
}
```

Step 3 — wire into the chat loop in `core/session.go`. After writing the assistant message to store and printing the response:

```go
signals := followup.Analyse(fullResponse.String(), ctx.RecentMessages())
for _, sig := range signals {
    r.PrintFollowUpHint(sig)
}
// If any signal was AutoPrompt, the prompt line is already shown.
// The user's next input naturally continues from here — no special handling needed.
```

**Important:** Do not block on follow-up signals. The user can always just type their next message. Follow-ups are hints, not gates.

Step 4 — make follow-up behaviour configurable:

```toml
# config.toml
[ai]
auto_followup       = true    # show follow-up hints at all
followup_shallow    = true    # enable shallow-response detection
followup_reference  = true    # enable reference detection
followup_conflict   = false   # disable conflict detection (can feel pedantic)
```

---

### Session summariser (long-session compression)

**What it does:**
Once a session exceeds a threshold (default: 40 messages), older messages are summarised into a single paragraph and stored in `sessions.summary`. This paragraph is prepended to the context window instead of the raw old messages, keeping the AI context fresh without losing the thread.

**Files touched:**
- `memory/summariser.go` — new file
- `memory/context.go` — use summary when available
- `memory/store.go` — add `UpdateSessionSummary`

**Integration steps:**

Step 1 — the summariser runs asynchronously. After saving each message, check if the session has crossed the threshold. If yes, trigger summarisation in a goroutine so it doesn't block the response:

```go
// core/session.go — in the message save block
if err := store.SaveMessage(assistantMsg); err != nil {
    return err
}
go func() {
    count, _ := store.CountMessages(session.ID)
    if count > 0 && count%40 == 0 {
        if err := memory.SummariseSession(session.ID, store, groqClient); err != nil {
            // log error, don't surface to user
            _ = err
        }
    }
}()
```

Step 2 — the summariser calls Groq with a low-token, high-speed request. Use the fastest available model for this (override config):

```go
// memory/summariser.go
func SummariseSession(sessionID string, store *Store, client ai.ChatClient) error {
    // Get all messages older than the last 20
    messages, err := store.GetMessagesPage(sessionID, 0, -20) // all except last 20
    if err != nil || len(messages) == 0 {
        return err
    }

    // Build a summarisation prompt
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
- The user's apparent goals

Conversation:
%s

Summary:`, transcript.String())

    // Use llama-3.1-8b-instant for speed — summary doesn't need the big model
    summary, err := client.Complete(context.Background(), summaryPrompt, "llama-3.1-8b-instant")
    if err != nil {
        return err
    }

    return store.UpdateSessionSummary(sessionID, summary)
}
```

Step 3 — in `memory/context.go`, prefer the summary over raw old messages:

```go
func (c *ContextBuilder) Build(userInput string) []Message {
    messages := []Message{}
    messages = append(messages, Message{Role: "system", Content: c.buildSystemPrompt()})

    history, _ := c.store.GetMessages(c.session.ID, 100)
    totalCount, _ := c.store.CountMessages(c.session.ID)

    // If we have more history than we're loading, and a summary exists — use it
    if totalCount > len(history) && c.session.Summary != "" {
        messages = append(messages, Message{
            Role: "system",
            Content: "[Summary of earlier conversation: " + c.session.Summary + "]",
        })
    }

    // Trim remaining history to token budget
    history = c.trimToTokenBudget(history, userInput)
    messages = append(messages, history...)
    return messages
}
```

This means a 200-message session still fits inside the context window — the AI has the summary of the first 180 messages and the full detail of the last 20.

---

### Topic shift detection

**What it does:**
When the user abruptly changes subject, the AI should acknowledge the shift rather than awkwardly trying to connect unrelated things. Topic shift detection uses lightweight heuristics (keyword overlap, named entity overlap) to detect when the new message is semantically distant from recent history.

**Files touched:**
- `memory/context.go` — add shift detection, inject shift note
- `tools/ai/followup.go` — optionally surface shift as a follow-up signal

**Integration steps:**

Step 1 — implement a simple overlap scorer. No ML required — keyword overlap is enough for a CLI tool:

```go
// memory/context.go
func detectTopicShift(newMessage string, recentMessages []Message) bool {
    if len(recentMessages) < 3 {
        return false // not enough history to detect a shift
    }

    // Extract keywords from new message (words > 4 chars, not stopwords)
    newKeywords := extractKeywords(newMessage)

    // Extract keywords from last 3 messages combined
    var recentText strings.Builder
    for _, m := range recentMessages[max(0, len(recentMessages)-3):] {
        recentText.WriteString(m.Content + " ")
    }
    recentKeywords := extractKeywords(recentText.String())

    // Compute overlap ratio
    overlap := keywordOverlap(newKeywords, recentKeywords)

    // If < 10% overlap and new message is not a continuation phrase, it's a shift
    continuationPhrases := []string{"also", "and", "what about", "additionally", "follow up", "another"}
    for _, phrase := range continuationPhrases {
        if strings.HasPrefix(strings.ToLower(newMessage), phrase) {
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
```

Step 2 — when a shift is detected, inject a system note *before* the user's message so the AI handles it gracefully:

```go
// memory/context.go — in Build()
if detectTopicShift(userInput, c.RecentMessages()) {
    messages = append(messages, Message{
        Role:    "system",
        Content: "[Note: The user appears to be changing topic. Acknowledge the shift naturally if helpful, but don't call it out explicitly every time.]",
    })
}
```

Step 3 — optionally, surface the shift to the user as well. If the shift is detected and `auto_followup` is enabled, after the AI's first response on the new topic, show:

```
  · Switched topic — previous context archived. Type /back to return.
```

`/back` is a slash command that restores the prior topic's injected context (retrieved from the last N messages before the detected shift point).

---

## v0.3 — Tools

v0.3 is where Goo becomes genuinely useful beyond chat. The three tools (tasks, GitHub, Tavily) are standalone but also deeply connected to the AI: the AI can read and invoke all three, and their results live in the same memory store that feeds the context window.

**Prerequisites:** v0.2 complete. Session persistence must work. The context builder must support injecting non-message content (used by all three tools).

---

### Task manager with SQLite backend

**What it does:**
Full CRUD task management persisted in `~/.config/goo/tasks.db`. Tasks have title, description, status, priority, tags, due date, project grouping, and notes. The AI can read and write tasks during chat sessions.

**Files touched:**
- `tools/tasks/store.go` — SQLite operations
- `tools/tasks/manager.go` — business logic, filtering, formatting
- `tools/tasks/reminder.go` — deadline logic
- `cmd/task.go` — cobra subcommands

**Integration steps:**

Step 1 — initialise the tasks database separately from history. On first run, `tools/tasks/store.go` creates the schema:

```go
// tools/tasks/store.go
const taskSchema = `
CREATE TABLE IF NOT EXISTS tasks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT NOT NULL,
    description TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'todo',
    priority    TEXT NOT NULL DEFAULT 'medium',
    tags        TEXT DEFAULT '[]',
    due_date    DATETIME,
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL,
    project     TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS task_notes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id    INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    content    TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tasks_status   ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
CREATE INDEX IF NOT EXISTS idx_tasks_due      ON tasks(due_date);
`
```

Step 2 — implement all task operations in `tools/tasks/manager.go`:

```go
type Manager struct{ store *TaskStore }

func (m *Manager) Add(title, desc, priority, project string, tags []string, due *time.Time) (*Task, error)
func (m *Manager) List(filters TaskFilters) ([]Task, error)
func (m *Manager) Get(id int) (*Task, error)
func (m *Manager) Complete(id int) error
func (m *Manager) Delete(id int) error
func (m *Manager) Edit(id int, updates TaskUpdate) error
func (m *Manager) AddNote(taskID int, content string) error
func (m *Manager) Stats() (TaskStats, error)

type TaskFilters struct {
    Status   string
    Priority string
    Project  string
    Tag      string
    Overdue  bool
    Due      *time.Time
}

type TaskStats struct {
    Total     int
    Todo      int
    InProgress int
    Done      int
    Overdue   int
    ByPriority map[string]int
}
```

Step 3 — wire all subcommands in `cmd/task.go`:

```go
// goo task add "Title" --priority high --due 2025-07-15 --tag work --project "goo"
// goo task list
// goo task list --status todo --priority high --overdue
// goo task list --project "goo"
// goo task done 3
// goo task delete 5
// goo task edit 3 --title "New title" --priority low
// goo task note 3 "Call vendor before proceeding"
// goo task stats
```

Step 4 — inject task context into the AI. In `memory/store.go`, add `GetRecentTaskSummary()`:

```go
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
            sb.WriteString(fmt.Sprintf("\n  - [OVERDUE] %s (due %s)", t.Title, t.DueDate.Format("Jan 2")))
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
```

This gets prepended to every AI request automatically via the context builder, so the AI always knows your task state.

Step 5 — enable the AI to add tasks. When the AI response contains patterns like `[TASK: ...]` or `I'll create a task for that`, the response parser in `core/session.go` intercepts them:

```go
// core/session.go — after streaming completes
if task := parseImpliedTask(fullResponse.String()); task != nil {
    r.PrintInfo(fmt.Sprintf("AI suggested a task: \"%s\"", task.Title))
    r.PrintPrompt("Add this task? [y/N] ")
    if userConfirms(scanner) {
        mgr.Add(task.Title, task.Description, task.Priority, "", nil, nil)
        r.PrintSuccess("Task added ✓")
    }
}
```

---

### GitHub tool (stats, PRs, issues)

**What it does:**
Authenticated GitHub REST API client that surfaces your activity: open PRs, assigned issues, commit streaks, contribution stats. The AI can query all of this and reason about it.

**Files touched:**
- `tools/github/client.go` — HTTP client with token auth
- `tools/github/stats.go` — contribution stats and streaks
- `tools/github/prs.go` — PRs and issues
- `cmd/gh.go` — cobra subcommands

**Integration steps:**

Step 1 — the GitHub client wraps `net/http` with the token injected from the encrypted keystore:

```go
// tools/github/client.go
type Client struct {
    http     *http.Client
    baseURL  string
    username string
}

func NewClient() (*Client, error) {
    token, err := config.GetAPIKey("github")
    if err != nil {
        return nil, fmt.Errorf("GitHub token not set. Run: goo config set-key github")
    }
    return &Client{
        http:    &http.Client{Timeout: 15 * time.Second},
        baseURL: "https://api.github.com",
        // token is stored in closure, not in the struct
        // injected via a RoundTripper
    }, nil
}

// Use a custom RoundTripper to inject the auth header on every request
// This keeps the token out of the struct fields (no accidental logging)
type authTransport struct {
    token string
    base  http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    req = req.Clone(req.Context())
    req.Header.Set("Authorization", "Bearer "+t.token)
    req.Header.Set("Accept", "application/vnd.github+json")
    req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
    return t.base.RoundTrip(req)
}
```

Step 2 — implement `tools/github/stats.go` for commit streaks and contribution data:

```go
// GetContributionStats returns stats for the last 30 days
func (c *Client) GetContributionStats(username string) (*ContributionStats, error) {
    // GitHub REST doesn't expose contribution graphs directly.
    // Use the events API: GET /users/{username}/events
    // Filter for PushEvent, PullRequestEvent, IssuesEvent
    // Group by date to compute streak
    events, err := c.getUserEvents(username)
    if err != nil {
        return nil, err
    }

    stats := &ContributionStats{Username: username}
    dayMap := map[string]bool{}

    for _, e := range events {
        date := e.CreatedAt.Format("2006-01-02")
        dayMap[date] = true
        switch e.Type {
        case "PushEvent":
            stats.Commits += e.Payload.Commits
        case "PullRequestEvent":
            if e.Payload.Action == "opened" {
                stats.PRsOpened++
            } else if e.Payload.Action == "closed" && e.Payload.PullRequest.Merged {
                stats.PRsMerged++
            }
        case "PullRequestReviewEvent":
            stats.PRsReviewed++
        case "IssuesEvent":
            if e.Payload.Action == "closed" {
                stats.IssuesClosed++
            }
        }
    }

    // Calculate streak: consecutive days with any activity
    stats.CurrentStreak = calculateStreak(dayMap)
    return stats, nil
}

func calculateStreak(dayMap map[string]bool) int {
    streak := 0
    today := time.Now()
    for i := 0; i < 365; i++ {
        day := today.AddDate(0, 0, -i).Format("2006-01-02")
        if !dayMap[day] {
            break
        }
        streak++
    }
    return streak
}
```

Step 3 — implement `tools/github/prs.go`:

```go
func (c *Client) GetMyPRs(repo string) ([]PullRequest, error) {
    // GET /search/issues?q=is:pr+is:open+assignee:{username}
    // or GET /repos/{owner}/{repo}/pulls?assignee={username}
    // Returns PRs with: number, title, repo, created_at, review_requested, labels
}

func (c *Client) GetMyIssues(label string) ([]Issue, error) {
    // GET /search/issues?q=is:issue+is:open+assignee:{username}+label:{label}
}

func (c *Client) PostReview(repo string, prNumber int, body string) error {
    // POST /repos/{owner}/{repo}/pulls/{pull_number}/reviews
    // body: { "event": "COMMENT", "body": body }
}
```

Step 4 — inject GitHub context into the AI. Similar to tasks, store the last GitHub fetch result in the session's context store:

```go
// After any gh command runs, update context
store.SetGitHubContext(session.ID, formatGHContext(prs, issues, stats))

// memory/store.go
func (s *Store) SetGitHubContext(sessionID, context string) error {
    _, err := s.db.Exec(
        `INSERT OR REPLACE INTO session_context (session_id, key, value, updated_at)
         VALUES (?, 'github', ?, ?)`,
        sessionID, context, time.Now(),
    )
    return err
}
```

Add a `session_context` table to `schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS session_context (
    session_id TEXT NOT NULL,
    key        TEXT NOT NULL,
    value      TEXT NOT NULL,
    updated_at DATETIME NOT NULL,
    PRIMARY KEY (session_id, key)
);
```

---

### Tavily web search

**What it does:**
Real-time web search via the Tavily API. Usable standalone (`goo search "query"`) or AI-triggered during chat. Results are summarised and injected into context so the AI can reason about them.

**Files touched:**
- `tools/search/tavily.go` — API client
- `cmd/search.go` — standalone search command
- `core/session.go` — handle `/search` slash command and AI-triggered search

**Integration steps:**

Step 1 — Tavily client:

```go
// tools/search/tavily.go
const tavilyBaseURL = "https://api.tavily.com"

type Client struct{ httpClient *http.Client }

type SearchRequest struct {
    APIKey        string   `json:"api_key"`
    Query         string   `json:"query"`
    SearchDepth   string   `json:"search_depth"`   // "basic" | "advanced"
    MaxResults    int      `json:"max_results"`
    IncludeAnswer bool     `json:"include_answer"`  // Tavily's own AI summary
    IncludeDomains []string `json:"include_domains,omitempty"`
    ExcludeDomains []string `json:"exclude_domains,omitempty"`
}

type SearchResult struct {
    Title   string  `json:"title"`
    URL     string  `json:"url"`
    Content string  `json:"content"`
    Score   float64 `json:"score"`
}

type SearchResponse struct {
    Answer  string         `json:"answer"`  // Tavily's AI-generated answer
    Results []SearchResult `json:"results"`
}

func (c *Client) Search(query string) (*SearchResponse, error) {
    apiKey, err := config.GetAPIKey("tavily")
    if err != nil {
        return nil, fmt.Errorf("Tavily key not set. Run: goo config set-key tavily")
    }
    // POST https://api.tavily.com/search
    // Return SearchResponse
}
```

Step 2 — present results cleanly:

```go
// core/renderer.go
func (r *Renderer) PrintSearchResults(query string, resp *search.SearchResponse) {
    // Print header box with query
    // Print Tavily's AI answer if present (highlighted differently)
    // Print up to 5 results: number, title, URL, first 150 chars of content
    // Dim URL beneath each result
}
```

Step 3 — AI-triggered search. This is done via Groq's tool/function calling. In the Groq request, define a `search_web` tool:

```go
// tools/ai/groq.go
type Tool struct {
    Type     string       `json:"type"`
    Function ToolFunction `json:"function"`
}

type ToolFunction struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Parameters  json.RawMessage `json:"parameters"`
}

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
```

Include tools in the Groq request body. After receiving a response, check if `finish_reason` is `"tool_calls"`. If yes, extract the tool call, execute it, append the result as a `tool` role message, and send the request again:

```go
// core/session.go — tool call handling loop
for {
    var fullResponse strings.Builder
    toolCall, err := groqClient.StreamChatWithTools(ctx, messages, r.StreamWriter(&fullResponse), allTools)
    if err != nil {
        return err
    }
    if toolCall == nil {
        // Normal text response — we're done
        break
    }
    // Execute the tool
    result, err := executeToolCall(toolCall, tavilyClient, taskManager, ghClient)
    if err != nil {
        result = fmt.Sprintf("Tool error: %s", err)
    }
    // Append the tool result to messages and loop
    messages = append(messages, memory.Message{
        Role:       "tool",
        Content:    result,
        ToolName:   toolCall.Name,
        ToolCallID: toolCall.ID,
    })
}
```

---

### AI tool invocation (function calling via Groq)

**What it does:**
The AI can invoke tasks, GitHub, and search tools autonomously during a chat session. This is what allows natural requests like "add a reminder to review the PR tomorrow" or "what's trending in Go this week" to work end-to-end.

**Files touched:**
- `tools/ai/groq.go` — extend `StreamChat` to support tools
- `core/router.go` — centralised tool executor
- `core/session.go` — tool call loop (shown above)

**Tool definitions to register with Groq:**

```go
// core/router.go
var AllTools = []ai.Tool{
    ai.SearchWebTool,
    {
        Type: "function",
        Function: ai.ToolFunction{
            Name:        "list_tasks",
            Description: "List the user's current tasks. Use when asked about tasks, todos, reminders, or what's pending.",
            Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
        },
    },
    {
        Type: "function",
        Function: ai.ToolFunction{
            Name:        "add_task",
            Description: "Add a new task for the user.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "title":    {"type":"string","description":"Task title"},
                    "priority": {"type":"string","enum":["low","medium","high","urgent"]},
                    "due_date": {"type":"string","description":"ISO 8601 date, optional"}
                },
                "required": ["title"]
            }`),
        },
    },
    {
        Type: "function",
        Function: ai.ToolFunction{
            Name:        "complete_task",
            Description: "Mark a task as done by ID.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "task_id": {"type":"integer"}
                },
                "required": ["task_id"]
            }`),
        },
    },
    {
        Type: "function",
        Function: ai.ToolFunction{
            Name:        "get_github_prs",
            Description: "Get open pull requests assigned to or created by the user.",
            Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
        },
    },
    {
        Type: "function",
        Function: ai.ToolFunction{
            Name:        "get_github_stats",
            Description: "Get the user's GitHub contribution stats including commit streak.",
            Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
        },
    },
}
```

**Tool executor:**

```go
// core/router.go
func ExecuteToolCall(call *ai.ToolCall, deps ToolDeps) (string, error) {
    switch call.Name {
    case "search_web":
        var args struct{ Query string `json:"query"` }
        json.Unmarshal(call.Arguments, &args)
        resp, err := deps.Tavily.Search(args.Query)
        if err != nil {
            return "", err
        }
        return formatSearchForAI(resp), nil

    case "list_tasks":
        tasks, err := deps.Tasks.List(tasks.TaskFilters{Status: "todo"})
        if err != nil {
            return "", err
        }
        return formatTasksForAI(tasks), nil

    case "add_task":
        var args struct {
            Title    string `json:"title"`
            Priority string `json:"priority"`
            DueDate  string `json:"due_date"`
        }
        json.Unmarshal(call.Arguments, &args)
        task, err := deps.Tasks.Add(args.Title, "", args.Priority, "", nil, parseDueDate(args.DueDate))
        if err != nil {
            return "", err
        }
        return fmt.Sprintf("Task created: #%d \"%s\"", task.ID, task.Title), nil

    case "complete_task":
        var args struct{ TaskID int `json:"task_id"` }
        json.Unmarshal(call.Arguments, &args)
        return "Task marked complete", deps.Tasks.Complete(args.TaskID)

    case "get_github_prs":
        prs, err := deps.GitHub.GetMyPRs("")
        if err != nil {
            return "", err
        }
        return formatPRsForAI(prs), nil

    case "get_github_stats":
        stats, err := deps.GitHub.GetContributionStats(config.Get("github.username"))
        if err != nil {
            return "", err
        }
        return formatStatsForAI(stats), nil

    default:
        return "", fmt.Errorf("unknown tool: %s", call.Name)
    }
}
```

---

## v0.4 — UX

v0.4 is about the experience becoming excellent. The logic is all in place — now the interface should feel polished, fast, and delightful to use in a terminal every day.

**Prerequisites:** v0.3 complete. All tools must be functional. The focus here is rendering and interaction, not new features.

---

### Bubbletea interactive chat UI

**What it does:**
Replaces the raw `bufio.Scanner` input loop with a proper Bubbletea TUI. This gives: real-time input editing, history navigation with ↑/↓, multiline input with Shift+Enter, a visible session header, and clean keyboard shortcuts.

**Files touched:**
- `core/session.go` — replace scanner loop with Bubbletea model
- `core/tui/model.go` — new Bubbletea model
- `core/tui/keys.go` — keybindings

**Integration steps:**

Step 1 — define the Bubbletea model:

```go
// core/tui/model.go
type Model struct {
    session    *memory.Session
    store      *memory.Store
    groq       *ai.GroqClient
    followup   *ai.FollowUpAnalyser
    ctx        *memory.ContextBuilder

    viewport   viewport.Model    // scrollable chat history display
    input      textarea.Model    // multiline text input
    messages   []DisplayMessage  // rendered messages for the viewport
    status     string            // "idle" | "thinking" | "streaming"
    spinner    spinner.Model
    width      int
    height     int
    err        error
}

type DisplayMessage struct {
    Role    string
    Content string
    Signals []ai.FollowUpSignal
}
```

Step 2 — handle the stream as a channel of strings. Bubbletea works with messages, so streaming token output needs to come in as a command:

```go
// core/tui/model.go
type StreamChunkMsg struct{ Chunk string }
type StreamDoneMsg  struct{ Full string }
type StreamErrMsg   struct{ Err error }

func streamCmd(groq *ai.GroqClient, messages []memory.Message) tea.Cmd {
    return func() tea.Msg {
        pr, pw := io.Pipe()
        var full strings.Builder
        go func() {
            err := groq.StreamChat(context.Background(), messages, io.MultiWriter(pw, &full))
            pw.CloseWithError(err)
        }()
        // Read chunks and send as tea.Msg via a sub-process
        // Use tea.Batch to send chunk-by-chunk
        // ...
        return StreamDoneMsg{Full: full.String()}
    }
}
```

Note: true streaming into Bubbletea requires a goroutine that sends `tea.Cmd` updates. The standard pattern is using a `chan tea.Msg` and a `tea.Program.Send()` call. Follow the Bubbletea streaming example pattern.

Step 3 — layout. The TUI has three regions:

```
┌─────────────────────────────────────────────────────────────┐
│  Header: "Goo · session a3f2c1 · llama-3.3-70b"  [Ctrl+C] │ ← fixed, 1 line
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Viewport: scrollable message history                       │ ← fills remaining
│  (rendered with Glamour markdown)                           │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  > |                                                        │ ← textarea, 3 lines
│  [Enter: send] [Shift+Enter: newline] [↑: history]         │ ← hint line
└─────────────────────────────────────────────────────────────┘
```

Step 4 — history navigation. Store sent messages in a `[]string` within the model. ↑ in the input replaces the textarea content with the prior sent message (cycling backward); ↓ cycles forward. This mirrors standard terminal readline behaviour.

---

### Lipgloss / Glamour rendering

**What it does:**
All output — messages, tables, headers, hints, errors — uses Lipgloss styles. AI responses are rendered as terminal markdown via Glamour. Tables for tasks and GitHub use the Lipgloss table renderer.

**Files touched:**
- `core/renderer.go` — centralise all style definitions here
- Remove any raw `fmt.Println` calls from `cmd/` files — all output goes through renderer

**Integration steps:**

Step 1 — define the style system in `core/renderer.go`. Use a `Renderer` struct that holds all style definitions. Detect dark/light mode from `$COLORTERM` or `$TERM_PROGRAM` and apply appropriate palette:

```go
type Renderer struct {
    glamour *glamour.TermRenderer
    styles  Styles
}

type Styles struct {
    AILabel    lipgloss.Style
    UserLabel  lipgloss.Style
    ToolOutput lipgloss.Style
    FollowUp   lipgloss.Style
    Error      lipgloss.Style
    Success    lipgloss.Style
    Info       lipgloss.Style
    Dim        lipgloss.Style
    Header     lipgloss.Style
    SessionID  lipgloss.Style
}

func New() *Renderer {
    dark := detectDarkMode()
    glamourStyle := "dark"
    if !dark {
        glamourStyle = "light"
    }
    tr, _ := glamour.NewTermRenderer(
        glamour.WithStylePath(glamourStyle),
        glamour.WithWordWrap(80),
    )
    return &Renderer{
        glamour: tr,
        styles:  buildStyles(dark),
    }
}
```

Step 2 — AI responses go through Glamour:

```go
func (r *Renderer) PrintAIResponse(content string) {
    rendered, err := r.glamour.Render(content)
    if err != nil {
        // Fallback: plain text
        fmt.Println(content)
        return
    }
    fmt.Print(rendered)
}
```

Step 3 — task and GitHub tables use `github.com/charmbracelet/lipgloss/table`:

```go
func (r *Renderer) PrintTaskTable(tasks []tasks.Task) {
    t := table.New().
        Border(lipgloss.NormalBorder()).
        BorderStyle(r.styles.Dim).
        Headers("ID", "Title", "Priority", "Status", "Due").
        StyleFunc(func(row, col int) lipgloss.Style {
            if row == 0 { return r.styles.Header }
            task := tasks[row-1]
            if task.Status == "done" { return r.styles.Dim }
            if task.IsOverdue()     { return r.styles.Error }
            return lipgloss.NewStyle()
        })
    for _, task := range tasks {
        t.Row(
            strconv.Itoa(task.ID),
            truncate(task.Title, 40),
            task.Priority,
            task.Status,
            formatDue(task.DueDate),
        )
    }
    fmt.Println(t.Render())
}
```

---

### Slash commands in chat

**What it does:**
Within an active `goo chat` session, prefix a message with `/` to invoke a command directly without leaving the session. The result is rendered inline.

**Files touched:**
- `core/tui/model.go` — add slash command dispatcher in `Update`
- `core/slashcmds.go` — new file: command registry and executor

**Full slash command list:**

```
/tasks                           → show task list inline
/tasks add "Title" [--priority]  → add a task inline
/tasks done <id>                 → complete a task
/gh prs                          → show open PRs
/gh issues                       → show assigned issues
/gh stats                        → show contribution stats
/search <query>                  → web search inline (bypasses AI)
/model <model-name>              → switch model mid-session
/clear                           → clear visible chat (history preserved)
/history                         → show recent sessions
/export                          → export session to markdown
/summary                         → show current session summary
/context                         → show what's in the AI context window (debug)
/help                            → list slash commands
/exit                            → end session
```

**Integration steps:**

```go
// core/slashcmds.go
type SlashCmd struct {
    Name    string
    Args    string
    Execute func(deps SlashDeps) (string, error) // returns rendered output or error
}

func ParseSlashCmd(input string) (*SlashCmd, bool) {
    if !strings.HasPrefix(input, "/") {
        return nil, false
    }
    parts := strings.SplitN(strings.TrimPrefix(input, "/"), " ", 2)
    name := parts[0]
    args := ""
    if len(parts) > 1 {
        args = parts[1]
    }
    return &SlashCmd{Name: name, Args: args}, true
}

// In core/tui/model.go Update():
if cmd, ok := slashcmds.ParseSlashCmd(input); ok {
    output, err := slashcmds.Execute(cmd, m.deps)
    if err != nil {
        m.messages = append(m.messages, DisplayMessage{Role: "error", Content: err.Error()})
    } else {
        m.messages = append(m.messages, DisplayMessage{Role: "tool", Content: output})
    }
    return m, nil // don't send to AI
}
```

---

### In-session task and GitHub commands

**What it does:**
The `/tasks` and `/gh` slash commands provide rich inline output — the same rendered tables as the standalone commands — without ever leaving the chat flow.

**Integration steps:**

Step 1 — `/tasks` shows a compact table. `compact` flag trims to 10 rows with a "and N more..." footer.

Step 2 — `/tasks add` goes directly to the task manager and prints a success confirmation. The task is also immediately available in the context window for the AI (the context builder re-reads task state on each turn).

Step 3 — `/gh prs` fetches and renders the same output as `goo gh prs` but inline. Since GitHub API calls can take 1-2 seconds, show a spinner:

```go
// In the slash command executor
r.StartSpinner("Fetching PRs...")
prs, err := deps.GitHub.GetMyPRs("")
r.StopSpinner()
if err != nil {
    return "", err
}
return deps.Renderer.RenderPRTable(prs), nil
```

Step 4 — results from inline GitHub/task commands update the session context. After executing `/gh prs`, call `store.SetGitHubContext(...)` with the fresh data. The AI's very next message will have this context available.

---

## v0.5 — Extensibility

v0.5 makes Goo an open platform. Users who want OpenAI, Anthropic, Ollama, or their own private API can add it in two commands.

**Prerequisites:** v0.4 complete. The custom provider system is built on top of the existing Groq client architecture.

---

### Custom API provider registry

**What it does:**
A named registry of alternative AI providers stored in `config.toml`. Each provider specifies a base URL, model, auth header template, and optional request body overrides. Goo uses the OpenAI-compatible chat completions format for all providers (works with OpenAI, Anthropic via proxy, Mistral, Together, Groq, Ollama, etc.).

**Files touched:**
- `tools/custom/registry.go` — provider CRUD
- `tools/custom/executor.go` — HTTP client that uses any registered provider
- `config/config.go` — read/write provider list
- `cmd/config.go` — `add-provider`, `remove-provider`, `list-providers`

**Integration steps:**

Step 1 — provider struct:

```go
// tools/custom/registry.go
type Provider struct {
    Name        string            `toml:"name"`
    BaseURL     string            `toml:"base_url"`
    Model       string            `toml:"model"`
    AuthHeader  string            `toml:"auth_header"`  // e.g. "Authorization: Bearer {key}"
    MaxTokens   int               `toml:"max_tokens"`
    ExtraBody   map[string]any    `toml:"extra_body"`   // additional request fields
    Stream      bool              `toml:"stream"`
}
```

Step 2 — the `add-provider` command:

```
$ goo config add-provider openai \
    --base-url https://api.openai.com/v1 \
    --model gpt-4o \
    --auth-header "Authorization: Bearer {key}"

→ Provider 'openai' registered. Set your key with:
  goo config set-key custom.openai

$ goo config add-provider ollama \
    --base-url http://localhost:11434/v1 \
    --model mistral \
    --no-auth

→ Provider 'ollama' registered. No key required (local).
```

Step 3 — the custom executor maps Goo's `[]memory.Message` to the OpenAI format and calls the provider. Streaming works via SSE if `stream: true`:

```go
// tools/custom/executor.go
type Executor struct {
    provider Provider
}

func (e *Executor) StreamChat(ctx context.Context, messages []memory.Message, out io.Writer) error {
    // 1. Derive auth header value (decrypt key, substitute {key} placeholder)
    // 2. Build OpenAI-compatible request body
    // 3. POST to {base_url}/chat/completions
    // 4. Parse SSE stream identically to groq.go
    // 5. Write delta.content to out
}
```

Step 4 — providers are selectable per session:

```
goo chat --provider openai
goo ask "Hello" --provider ollama
goo config set general.default_provider ollama   # persist
```

The `core/session.go` `RunChatSession` function receives a `ChatClient` interface, not a concrete `*GroqClient`. This interface is already defined — both `GroqClient` and `custom.Executor` implement it:

```go
// tools/ai/client.go
type ChatClient interface {
    StreamChat(ctx context.Context, messages []memory.Message, out io.Writer) error
    Complete(ctx context.Context, prompt string, model string) (string, error)
}
```

---

### Ollama / OpenAI-compatible providers

**What it does:**
First-party tested profiles for the most common providers. These are pre-configured templates users can enable with one command, rather than specifying all parameters manually.

**Built-in provider templates:**

```go
// tools/custom/registry.go
var BuiltinProviders = map[string]Provider{
    "openai": {
        BaseURL:    "https://api.openai.com/v1",
        Model:      "gpt-4o",
        AuthHeader: "Authorization: Bearer {key}",
        MaxTokens:  4096,
        Stream:     true,
    },
    "anthropic": {
        BaseURL:    "https://api.anthropic.com/v1",
        Model:      "claude-sonnet-4-6",
        AuthHeader: "x-api-key: {key}",
        ExtraBody:  map[string]any{"anthropic_version": "2023-06-01"},
        MaxTokens:  4096,
        Stream:     true,
    },
    "ollama": {
        BaseURL:    "http://localhost:11434/v1",
        Model:      "mistral",
        AuthHeader: "",   // no auth
        MaxTokens:  4096,
        Stream:     true,
    },
    "together": {
        BaseURL:    "https://api.together.xyz/v1",
        Model:      "meta-llama/Llama-3-70b-chat-hf",
        AuthHeader: "Authorization: Bearer {key}",
        MaxTokens:  4096,
        Stream:     true,
    },
}
```

Enable with:

```
goo config use-provider ollama      # sets from builtin template, no key needed
goo config use-provider anthropic   # sets from builtin template, prompts for key
```

**Anthropic compatibility note:** Anthropic's API uses a different request format from OpenAI (different field names for the system message, different SSE event structure). Handle this with a provider-specific adapter flag:

```go
type Provider struct {
    // ... existing fields
    Adapter string `toml:"adapter"` // "" (default = OpenAI-compat) | "anthropic"
}
```

---

### Plugin system (future)

**What it does:**
A lightweight plugin interface for community-built Goo extensions. Plugins are standalone Go binaries that communicate with the main Goo process via stdin/stdout using a simple JSON protocol. This keeps the core binary small while allowing unlimited extensibility.

**Design (to be fully implemented in a post-v1.0 release):**

```
Plugin interface:
  goo will call the plugin binary with a JSON payload on stdin
  The plugin writes a JSON response to stdout
  Error output goes to stderr

Plugin manifest (~/.config/goo/plugins/{name}/plugin.json):
{
    "name": "jira",
    "version": "1.0.0",
    "description": "Jira issue integration for Goo",
    "binary": "~/.config/goo/plugins/jira/goo-plugin-jira",
    "tools": [
        {
            "name": "list_jira_issues",
            "description": "List open Jira issues assigned to the user",
            "parameters": { ... }
        }
    ],
    "slash_commands": ["jira"]
}
```

Goo discovers plugins by scanning `~/.config/goo/plugins/` at startup and registers their declared tools with the Groq tool list and their slash commands with the slash command dispatcher.

For v0.5, implement the scaffolding only:
- `tools/custom/plugin.go` — define the `Plugin` interface and JSON protocol
- `cmd/config.go` — add `goo plugin install <binary>`, `goo plugin list`, `goo plugin remove`
- No actual plugin execution yet — that's post-v1.0

---

## v1.0 — Release

v1.0 is about correctness, coverage, and distribution. No new features — only hardening, polish, and making it trivial for anyone to install Goo.

---

### Full test suite (>80% coverage)

**What to test:**

Every package needs a `_test.go` file. The 80% target is measured by `go test ./... -coverprofile=coverage.out`. Priority order for coverage:

| Package                  | Target | Focus                                                    |
|--------------------------|--------|----------------------------------------------------------|
| `config/keys.go`         | 95%    | Encrypt/decrypt round trips, wrong passphrase, salt      |
| `memory/context.go`      | 90%    | Window trimming, summary injection, topic shift          |
| `memory/summariser.go`   | 85%    | Summary trigger, Groq mock, store interaction            |
| `tools/ai/followup.go`   | 90%    | All signal types, edge cases (empty response, short)     |
| `tools/tasks/manager.go` | 90%    | All CRUD ops, filter combinations, overdue logic         |
| `core/router.go`         | 85%    | Tool dispatch, all tool types, error paths               |
| `tools/custom/executor.go`| 80%   | Provider request format, streaming, auth injection       |
| `tools/ai/groq.go`       | 75%    | Mock HTTP server for streaming SSE                       |
| `cmd/`                   | 60%    | Command wiring, flag parsing (lower priority)            |

**Mock patterns:**

```go
// testdata/mocks/groq_server.go
func NewMockGroqServer(responses []string) *httptest.Server {
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        for _, chunk := range responses {
            fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n",
                chunk)
            time.Sleep(10 * time.Millisecond)
        }
        fmt.Fprintln(w, "data: [DONE]")
    }))
}
```

**Run coverage check in CI:**

```yaml
# .github/workflows/ci.yml
- name: Test with coverage
  run: |
    go test ./... -coverprofile=coverage.out -covermode=atomic
    go tool cover -func=coverage.out | tail -1
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | tr -d '%')
    if (( $(echo "$COVERAGE < 80" | bc -l) )); then
      echo "Coverage ${COVERAGE}% is below 80%"
      exit 1
    fi
```

---

### Cross-platform releases (goreleaser)

**What it does:**
A single `goreleaser release` command (triggered by a git tag) builds binaries for Linux amd64/arm64, macOS amd64/arm64, and Windows amd64, packages them as tarballs/zip, computes checksums, and publishes a GitHub Release.

**.goreleaser.yaml:**

```yaml
version: 2

before:
  hooks:
    - go mod tidy
    - go generate ./...

builds:
  - binary: goo
    main: ./
    env:
      - CGO_ENABLED=1     # required for go-sqlite3
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}

archives:
  - format: tar.gz
    name_template: "goo_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip
    files:
      - README.md
      - LICENSE

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"

release:
  github:
    owner: yourusername
    name: goo
  draft: false
  prerelease: auto
```

**CGO note:** `go-sqlite3` requires CGO. For cross-compilation, use `zig cc` as the C cross-compiler (goreleaser supports this via `CC` environment variable per target). Add to `.goreleaser.yaml`:

```yaml
builds:
  - env:
      - CGO_ENABLED=1
    overrides:
      - goos: linux
        goarch: arm64
        env:
          - CGO_ENABLED=1
          - CC=zig cc -target aarch64-linux-musl
```

Alternatively: switch to `modernc.org/sqlite` (pure Go SQLite, no CGO). Swap the import in `memory/store.go` and `tools/tasks/store.go`. Slight performance cost but eliminates all CGO complexity.

---

### Man page generation

**What it does:**
Generate a Unix man page (`goo.1`) from the cobra command tree automatically. Installed to `/usr/local/share/man/man1/` so `man goo` works.

**Files touched:**
- `cmd/docs.go` — hidden `goo docs` command that writes man pages
- `Makefile` — `make man` target

**Integration steps:**

```go
// cmd/docs.go
import "github.com/spf13/cobra/doc"

var docsCmd = &cobra.Command{
    Use:    "docs",
    Short:  "Generate documentation",
    Hidden: true,
    RunE: func(cmd *cobra.Command, args []string) error {
        dir := "./man"
        os.MkdirAll(dir, 0755)
        header := &doc.GenManHeader{
            Title:   "GOO",
            Section: "1",
            Source:  "Goo AI CLI",
        }
        return doc.GenManTree(rootCmd, header, dir)
    },
}
```

```makefile
man:
	go run . docs
	@echo "Man pages written to ./man/"

install-man: man
	install -d /usr/local/share/man/man1
	install -m 644 man/goo*.1 /usr/local/share/man/man1/
	mandb 2>/dev/null || true
```

---

### Homebrew formula

**What it does:**
Allows macOS (and Linux) users to install Goo with `brew install yourusername/tap/goo`.

**Steps:**

Step 1 — create a Homebrew tap repository: `github.com/yourusername/homebrew-tap`

Step 2 — after a goreleaser release, the formula is auto-generated if configured:

```yaml
# .goreleaser.yaml — add brews section
brews:
  - name: goo
    repository:
      owner: yourusername
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    homepage: "https://github.com/yourusername/goo"
    description: "Context-aware AI CLI assistant for your terminal"
    license: "MIT"
    install: |
      bin.install "goo"
      man1.install Dir["man/*.1"]
    test: |
      system "#{bin}/goo version"
```

Step 3 — add a GitHub Actions secret `HOMEBREW_TAP_TOKEN` (a GitHub PAT with write access to the tap repo).

After release, users install with:

```
brew tap yourusername/tap
brew install goo
```

---

### Installation script

**What it does:**
A single `curl | sh` installer for users who don't have Homebrew or want a quick install without a package manager.

**`install.sh`** (committed to the repo root):

```bash
#!/usr/bin/env bash
set -euo pipefail

REPO="yourusername/goo"
BINARY="goo"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# Get latest release version from GitHub API
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')

echo "Installing ${BINARY} v${VERSION} for ${OS}/${ARCH}..."

# Download tarball
TARBALL="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${TARBALL}"

TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

curl -fsSL "$URL" -o "${TMP}/${TARBALL}"

# Verify checksum
curl -fsSL "https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt" \
  -o "${TMP}/checksums.txt"

cd "$TMP"
sha256sum --check --ignore-missing checksums.txt

# Extract and install
tar -xzf "${TARBALL}" -C "$TMP"
install -m 755 "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

echo ""
echo "✓ ${BINARY} installed to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "Get started:"
echo "  goo config set-key groq"
echo "  goo chat"
```

Usage:

```
curl -fsSL https://raw.githubusercontent.com/yourusername/goo/main/install.sh | sh
```

Or with a custom install directory:

```
curl -fsSL .../install.sh | INSTALL_DIR=~/.local/bin sh
```

**Add `install.sh` to the goreleaser archive** so it's also bundled in the release tarball:

```yaml
# .goreleaser.yaml — in archives.files
files:
  - README.md
  - LICENSE
  - install.sh
```

---

*updates.md — Goo AI CLI version integration guide.*