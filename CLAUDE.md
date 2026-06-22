# Goo AI CLI — CLAUDE.md
> **Project Codename:** Goo
> **Full Name:** Goo AI CLI
> **Purpose:** A powerful, context-aware AI assistant for the terminal — combining task management, GitHub tooling, web search, and general-purpose AI chat in one cohesive CLI experience.

---

## Table of Contents

1. [Project Vision](#1-project-vision)
2. [Architecture Overview](#2-architecture-overview)
3. [Directory Structure](#3-directory-structure)
4. [Core Systems](#4-core-systems)
   - 4.1 [Configuration & API Key Management](#41-configuration--api-key-management)
   - 4.2 [Memory & Conversation History](#42-memory--conversation-history)
   - 4.3 [AI Engine (Groq)](#43-ai-engine-groq)
   - 4.4 [Web Search (Tavily)](#44-web-search-tavily)
   - 4.5 [Task Manager](#45-task-manager)
   - 4.6 [GitHub Tool](#46-github-tool)
5. [Commands Reference](#5-commands-reference)
6. [AI Context & Follow-Up System](#6-ai-context--follow-up-system)
7. [Encryption Design](#7-encryption-design)
8. [Rendering & UX](#8-rendering--ux)
9. [Error Handling Philosophy](#9-error-handling-philosophy)
10. [Extensibility: Custom APIs](#10-extensibility-custom-apis)
11. [Testing Strategy](#11-testing-strategy)
12. [Starter Code](#12-starter-code)
13. [Makefile & Tooling](#13-makefile--tooling)
14. [Roadmap](#14-roadmap)

---

## 1. Project Vision

Goo is a terminal-first AI assistant built with Go. It is not a chatbot wrapper — it is a **tool** that happens to be powered by AI. The design philosophy:

- **Context persistence**: Goo remembers what you said three messages ago. It knows if you just ran `goo task list` and then asked "which ones are overdue?". It answers **in context**, not in isolation.
- **Follow-up awareness**: If an AI response is ambiguous, Goo prompts you. It never silently dead-ends.
- **Tool integration**: Tasks, GitHub, and web search are first-class features — the AI can invoke them, not just talk about them.
- **User-controlled keys**: Users bring their own Groq and Tavily keys. Keys are AES-256 encrypted at rest. Custom API providers can be registered.
- **Offline-first UX**: All non-AI features (tasks, GitHub cache) work without an internet connection.

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                      goo (CLI binary)                   │
│  ┌───────────┐  ┌──────────┐  ┌────────┐  ┌─────────┐ │
│  │  cmd/     │  │ memory/  │  │ tools/ │  │ config/ │ │
│  │ (cobra)   │  │ (context)│  │ (ai,   │  │ (keys,  │ │
│  │           │  │          │  │  tasks,│  │  prefs) │ │
│  │           │  │          │  │  gh,   │  │         │ │
│  │           │  │          │  │  web)  │  │         │ │
│  └─────┬─────┘  └────┬─────┘  └───┬────┘  └────┬────┘ │
│        │             │             │             │      │
│        └─────────────┴─────────────┴─────────────┘     │
│                          │                              │
│                   ┌──────▼──────┐                       │
│                   │  core/      │                       │
│                   │  (session,  │                       │
│                   │   router,   │                       │
│                   │   renderer) │                       │
│                   └─────────────┘                       │
└─────────────────────────────────────────────────────────┘
         │                    │                  │
    ┌────▼───┐          ┌─────▼────┐      ┌──────▼─────┐
    │  Groq  │          │  Tavily  │      │  GitHub    │
    │  API   │          │  Search  │      │  REST API  │
    └────────┘          └──────────┘      └────────────┘
```

**Data flow for a typical AI chat turn:**

```
User input
  → Router: classify intent (chat / task / github / search)
  → Memory: inject conversation history + tool context
  → AI Engine: send to Groq with system prompt + history
  → Response parser: detect tool calls in response
  → Tool executor: run tasks/github/search if needed
  → Memory: store turn (user + assistant + tool results)
  → Renderer: pretty-print to terminal
  → Follow-up detector: prompt user if needed
```

---

## 3. Directory Structure

```
goo/
├── CLAUDE.md                    ← this file
├── README.md
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yaml
│
├── main.go                      ← entry point
│
├── cmd/                         ← cobra command definitions
│   ├── root.go                  ← root command, global flags
│   ├── chat.go                  ← `goo chat` — interactive AI session
│   ├── ask.go                   ← `goo ask "..."` — single-shot query
│   ├── task.go                  ← `goo task [add|list|done|delete|...]`
│   ├── gh.go                    ← `goo gh [prs|issues|stats|...]`
│   ├── search.go                ← `goo search "..."` — web search via Tavily
│   ├── config.go                ← `goo config [set|get|list|reset]`
│   └── history.go               ← `goo history [show|clear|export]`
│
├── core/
│   ├── session.go               ← active session state (current mode, tool context)
│   ├── router.go                ← intent classifier — decides which tool to invoke
│   └── renderer.go              ← terminal output: markdown, tables, spinners, colours
│
├── memory/
│   ├── store.go                 ← read/write conversation history (SQLite)
│   ├── context.go               ← build context window for AI (trim, summarise, inject)
│   ├── summariser.go            ← LLM-backed conversation summariser for long sessions
│   └── schema.sql               ← SQLite schema for messages, sessions, tool calls
│
├── tools/
│   ├── ai/
│   │   ├── groq.go              ← Groq API client (streaming)
│   │   ├── prompt.go            ← system prompt builder
│   │   └── followup.go          ← follow-up detector and prompter
│   ├── search/
│   │   └── tavily.go            ← Tavily search client
│   ├── tasks/
│   │   ├── manager.go           ← CRUD + filtering + priorities
│   │   ├── store.go             ← SQLite persistence
│   │   └── reminder.go          ← deadline reminder logic
│   ├── github/
│   │   ├── client.go            ← GitHub REST client (OAuth device flow)
│   │   ├── stats.go             ← commit streaks, contribution data
│   │   └── prs.go               ← PR reviews, issue tracking
│   └── custom/
│       ├── registry.go          ← user-defined custom API providers
│       └── executor.go          ← HTTP executor for custom providers
│
├── config/
│   ├── config.go                ← load/save config (TOML)
│   ├── keys.go                  ← AES-256-GCM key encryption/decryption
│   └── defaults.go              ← default settings
│
└── testdata/
    ├── fixtures/
    └── mocks/
```

---

## 4. Core Systems

### 4.1 Configuration & API Key Management

**Config file location:** `~/.config/goo/config.toml`
**Encrypted keystore:** `~/.config/goo/keys.enc`

The config stores user preferences and metadata. API keys are **never** stored in the config file — they live only in the encrypted keystore.

#### Config schema (TOML)

```toml
[general]
default_model    = "llama-3.3-70b-versatile"   # Groq model
theme            = "dark"                        # dark | light | auto
history_limit    = 50                            # messages kept in context window
auto_followup    = true                          # prompt follow-ups automatically
stream           = true                          # stream AI responses token by token

[ai]
max_tokens       = 4096
temperature      = 0.7
system_prompt    = ""                            # optional custom system prompt override

[github]
username         = ""
default_repo     = ""                            # owner/repo format

[tasks]
storage_path     = "~/.config/goo/tasks.db"
default_priority = "medium"

[search]
max_results      = 5
search_depth     = "basic"                       # basic | advanced (Tavily param)

[custom_apis]
# Each entry is a named provider block
# [[custom_apis.providers]]
# name    = "openai"
# base_url = "https://api.openai.com/v1"
# model   = "gpt-4o"
```

#### Key encryption model

```
Master key derivation:
  passphrase (user) + salt (random, stored in ~/.config/goo/salt)
    → Argon2id → 32-byte master key

Storing an API key:
  plaintext_key
    → AES-256-GCM encrypt with master key + random nonce
    → base64(nonce + ciphertext) stored in keys.enc

Decrypting for a request:
  master key (derived from passphrase cached in-session or re-prompted)
    → AES-256-GCM decrypt
    → plaintext key used in-memory only, never written
```

On first run, Goo prompts the user for a passphrase and caches it in a session-scoped in-memory store (not on disk). The passphrase is re-prompted if the session is older than 15 minutes of inactivity (configurable).

**Supported key slots:**

| Slot          | Description                         |
|---------------|-------------------------------------|
| `groq`        | Groq API key (required for AI)      |
| `tavily`      | Tavily search API key               |
| `github`      | GitHub personal access token        |
| `custom.<name>` | Any user-defined provider key     |

---

### 4.2 Memory & Conversation History

This is the most critical system. Poor memory is why most AI CLIs feel dumb. Goo's memory system has three layers:

#### Layer 1 — Active context window

The last N messages (default 50, configurable) sent to Groq on every turn. Stored in-process during a session.

```go
type Message struct {
    Role      string    // "system" | "user" | "assistant" | "tool"
    Content   string
    ToolCall  *ToolCall // non-nil if this message invoked a tool
    Timestamp time.Time
    SessionID string
}
```

#### Layer 2 — SQLite persistent history

Every message is written to `~/.config/goo/history.db`. Sessions are keyed by UUID. Users can resume any past session with `goo history resume <session-id>`.

**Schema:**

```sql
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    started_at  DATETIME NOT NULL,
    title       TEXT,                  -- auto-generated from first message
    mode        TEXT NOT NULL,         -- chat | task | github | search
    summary     TEXT                   -- auto-generated summary when session is long
);

CREATE TABLE messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT NOT NULL REFERENCES sessions(id),
    role        TEXT NOT NULL,
    content     TEXT NOT NULL,
    tool_name   TEXT,
    tool_input  TEXT,
    tool_output TEXT,
    created_at  DATETIME NOT NULL
);

CREATE INDEX idx_messages_session ON messages(session_id, created_at);
```

#### Layer 3 — Smart context injection

When building the prompt for Groq, `memory/context.go` does more than just append history. It:

1. **Counts tokens** (approximated at 4 chars/token) and trims oldest messages if the window would exceed the model's context limit.
2. **Injects tool state**: if the user recently ran `goo task list`, the current task list is summarised and injected as a system note.
3. **Injects session summary**: for long sessions, the summariser condenses older messages into a paragraph that's prepended.
4. **Detects topic shifts**: if the user's message has low semantic overlap with recent history, Goo acknowledges the shift gracefully rather than forcing false continuity.

#### Follow-up detection

`tools/ai/followup.go` inspects every AI response for signals that a follow-up is warranted:

- Response ends with a question → surface it prominently
- Response contains uncertainty language ("I'm not sure", "it depends", "could you clarify") → prompt user with suggested clarifying questions
- Response references a task/repo/file that doesn't exist yet → offer to create it
- Response is very short (< 3 sentences) relative to a complex question → ask "Want me to go deeper on any part?"

```go
type FollowUpSignal struct {
    Type       string   // "question" | "uncertainty" | "reference" | "shallow"
    Suggestion string   // suggested follow-up prompt to show the user
    AutoPrompt bool     // if true, show immediately; if false, show after a pause
}
```

---

### 4.3 AI Engine (Groq)

Groq is the primary AI provider. It is fast (low latency LPU inference) and supports streaming, which Goo uses by default.

#### Supported models (configurable)

| Model                        | Best for                    |
|------------------------------|-----------------------------|
| `llama-3.3-70b-versatile`    | Default — best overall      |
| `llama-3.1-8b-instant`       | Fast responses, simple tasks|
| `mixtral-8x7b-32768`         | Long context sessions       |
| `gemma2-9b-it`               | Lightweight fallback        |

#### System prompt (built dynamically)

```
You are Goo, a terminal AI assistant. You are concise, direct, and tool-aware.

Current context:
- Date/time: {datetime}
- Active session: {session_id}
- Open tasks: {task_summary}         ← injected if tasks exist
- GitHub context: {gh_context}        ← injected if gh tool used recently
- Last search: {search_summary}       ← injected if search used recently

You have access to the following tools: search_web, list_tasks, add_task, complete_task,
get_github_prs, get_github_stats, get_github_issues.

When you are uncertain or need clarification, ask ONE focused question. Do not ask multiple
questions at once. Do not give up — always attempt a best-effort answer and offer to refine.

If you invoke a tool, wait for the result before responding to the user.
```

#### Streaming implementation

Goo uses Groq's SSE streaming endpoint. Characters print to the terminal as they arrive, giving the appearance of real-time generation.

```go
// tools/ai/groq.go (excerpt)
func (c *GroqClient) StreamChat(ctx context.Context, messages []Message, out io.Writer) error {
    // POST to https://api.groq.com/openai/v1/chat/completions
    // with stream: true
    // Parse SSE events, extract delta.content, write to out
    // On finish_reason: "stop", flush and return
}
```

---

### 4.4 Web Search (Tavily)

Tavily provides real-time web search with AI-friendly result summaries.

#### Invocation modes

1. **Explicit**: `goo search "what is the Raft consensus algorithm?"`
2. **AI-triggered**: The AI decides to call `search_web` when the user asks about something likely requiring current information.
3. **Inline in chat**: During a `goo chat` session, typing `search: <query>` bypasses the AI and runs a direct search.

#### Result presentation

```
┌─ Web Search: "raft consensus algorithm" ─────────────────┐
│ 1. Raft: Understandable Consensus — raft.github.io        │
│    Raft is designed to be more understandable than Paxos. │
│    It decomposes consensus into leader election, log...   │
│                                                           │
│ 2. The Raft Paper (PDF) — cs.stanford.edu                 │
│    Diego Ongaro and John Ousterhout, 2014. In Search of   │
│    an Understandable Consensus Algorithm...               │
└───────────────────────────────────────────────────────────┘
  AI summary injected into context ✓
```

After a search, results are summarised and injected into the conversation context so the AI can reference them in the next turn.

---

### 4.5 Task Manager

Tasks are stored in SQLite at `~/.config/goo/tasks.db`.

#### Task schema

```sql
CREATE TABLE tasks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT NOT NULL,
    description TEXT,
    status      TEXT NOT NULL DEFAULT 'todo',   -- todo | in_progress | done | cancelled
    priority    TEXT NOT NULL DEFAULT 'medium', -- low | medium | high | urgent
    tags        TEXT,                           -- JSON array of strings
    due_date    DATETIME,
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL,
    project     TEXT                            -- optional project grouping
);

CREATE TABLE task_notes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id    INTEGER NOT NULL REFERENCES tasks(id),
    content    TEXT NOT NULL,
    created_at DATETIME NOT NULL
);
```

#### Task commands

```
goo task add "Buy groceries" --priority high --due 2025-07-01 --tag personal
goo task list
goo task list --status todo --priority high
goo task list --project "side project"
goo task done 3
goo task delete 5
goo task note 3 "Remember to check the fridge first"
goo task edit 3 --title "Buy groceries and cooking oil"
goo task stats                   # summary: total, done, overdue, by priority
```

#### AI integration

During a chat session, the AI can:
- Read your task list to give context-aware responses
- Add tasks on your behalf ("remind me to deploy by Friday" → task created)
- Mark tasks done via natural language ("I finished the groceries task")
- Suggest task breakdowns for complex goals

---

### 4.6 GitHub Tool

Uses the GitHub REST API v3 via a personal access token (stored encrypted).

#### Authentication

On first use, `goo config set github-token` stores the token encrypted. Alternatively, Goo supports the GitHub OAuth device flow for users who prefer not to generate tokens manually.

#### Commands

```
goo gh prs                        # open PRs assigned to you
goo gh prs --repo owner/repo      # PRs for a specific repo
goo gh issues                     # open issues assigned to you
goo gh issues --label bug
goo gh stats                      # your contribution stats (commits, reviews, streak)
goo gh stats --user someoneelse
goo gh repos                      # your repositories
goo gh diff 123                   # show diff of PR #123
goo gh review 123 "Looks good"    # post a review comment
```

#### Stats output (example)

```
┌─ GitHub Stats — @username ────────────────────────────────┐
│  Commit streak    ██████████████░░░░░░  14 / 30 days      │
│  PRs opened       23   ▲ +5 from last month               │
│  PRs reviewed     41   ▲ +12 from last month              │
│  Issues closed    18                                       │
│  Top language     Go (68%)                                 │
└────────────────────────────────────────────────────────────┘
```

#### AI integration

The AI can answer questions like:
- "What PRs are waiting on me?"
- "Summarise the issues labelled 'bug' in my main repo"
- "What did I work on last week?"

The GitHub tool injects relevant data into the context window automatically.

---

## 5. Commands Reference

### Top-level commands

```
goo                              # shows help
goo version                      # prints version and build info
goo update                       # self-update binary

goo chat                         # interactive multi-turn AI chat session
goo ask "<question>"             # single-shot AI query, exits after response
goo search "<query>"             # direct web search via Tavily

goo task <subcommand>            # task manager
goo gh <subcommand>              # github tool

goo config set <key> <value>     # set a config value
goo config get <key>             # get a config value
goo config list                  # list all config
goo config set-key <provider>    # securely set an API key (prompts, never echoed)
goo config reset                 # reset config to defaults

goo history show                 # list past sessions
goo history show <session-id>    # show a specific session
goo history resume <session-id>  # resume a past session (loads context)
goo history clear                # clear all history
goo history export <session-id>  # export to markdown
```

### Global flags

```
--model string       Override AI model for this command
--no-stream          Disable streaming, wait for full response
--raw                Raw output, no colours or formatting
--debug              Verbose debug output (API requests, context window)
--quiet              Suppress all non-essential output
```

---

## 6. AI Context & Follow-Up System

This is what separates Goo from a simple API wrapper.

### The context window pipeline

Every time Goo sends a request to Groq, it builds the messages array like this:

```
[system prompt]
[session summary if session > 30 messages]
[last N messages from history]
[injected tool context (tasks, github, search)]
[current user message]
```

The system prompt is **rebuilt on every turn** with fresh timestamps and tool state.

### Follow-up behaviour

After every AI response, `followup.go` runs a lightweight analysis:

```go
type FollowUpAnalyser struct{}

func (f *FollowUpAnalyser) Analyse(response string, history []Message) []FollowUpSignal {
    signals := []FollowUpSignal{}

    // 1. Did the AI ask a question?
    if endsWithQuestion(response) {
        signals = append(signals, FollowUpSignal{
            Type: "question",
            AutoPrompt: true,
        })
    }

    // 2. Did the AI express uncertainty?
    uncertaintyPhrases := []string{"not sure", "it depends", "clarify", "could you", "unclear"}
    if containsAny(response, uncertaintyPhrases) {
        signals = append(signals, FollowUpSignal{
            Type:       "uncertainty",
            Suggestion: generateClarifyingPrompt(response, history),
            AutoPrompt: false,
        })
    }

    // 3. Was the response very short for the question complexity?
    questionComplexity := estimateComplexity(lastUserMessage(history))
    if questionComplexity > 0.7 && wordCount(response) < 80 {
        signals = append(signals, FollowUpSignal{
            Type:       "shallow",
            Suggestion: "Want me to go deeper on any part of this?",
            AutoPrompt: false,
        })
    }

    return signals
}
```

When signals are found:
- `AutoPrompt: true` → the follow-up question is displayed immediately below the response, styled in a distinct colour, and the prompt awaits input.
- `AutoPrompt: false` → a subtle prompt hint appears: `  ↩  Press Enter to continue or type a follow-up...`

### Conversation resumption

```
$ goo history show

  ID         STARTED           TITLE                          MSGS
  a3f2c1     2025-06-20 14:22  "distributed systems design"   24
  b9e01d     2025-06-19 09:11  "task planning for sprint"     12
  c77a44     2025-06-18 20:30  "debug Go race condition"      38

$ goo history resume a3f2c1

  Resuming session: "distributed systems design" (24 messages)
  Last message (Jun 20, 15:44): "So the write-ahead log goes before..."
  
  Goo: Welcome back. You were last asking about WAL durability guarantees 
       in the context of Raft. Want to pick up where we left off?

>
```

---

## 7. Encryption Design

### Key files

| File                       | Contents                             | Permissions |
|----------------------------|--------------------------------------|-------------|
| `~/.config/goo/config.toml`| User preferences (no secrets)        | `0644`      |
| `~/.config/goo/keys.enc`   | Encrypted API keys                   | `0600`      |
| `~/.config/goo/salt`       | Random salt for key derivation       | `0600`      |
| `~/.config/goo/history.db` | Conversation history (SQLite)        | `0600`      |
| `~/.config/goo/tasks.db`   | Tasks (SQLite)                       | `0600`      |

### Encryption implementation

```go
// config/keys.go

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "golang.org/x/crypto/argon2"
)

// Key derivation parameters (Argon2id)
const (
    ArgonTime    = 3
    ArgonMemory  = 64 * 1024   // 64 MB
    ArgonThreads = 4
    ArgonKeyLen  = 32
)

func DeriveKey(passphrase, salt []byte) []byte {
    return argon2.IDKey(passphrase, salt, ArgonTime, ArgonMemory, ArgonThreads, ArgonKeyLen)
}

func Encrypt(plaintext, key []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    nonce := make([]byte, gcm.NonceSize())
    if _, err = rand.Read(nonce); err != nil {
        return nil, err
    }
    return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func Decrypt(ciphertext, key []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, errors.New("ciphertext too short")
    }
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    return gcm.Open(nil, nonce, ciphertext, nil)
}
```

### Session key cache

The derived master key is stored in a `sync.Map` keyed by process PID, and evicted after a configurable inactivity timeout (default 15 minutes). It is **never written to disk**.

---

## 8. Rendering & UX

Goo uses `github.com/charmbracelet/lipgloss` for styled terminal output and `github.com/charmbracelet/bubbletea` for the interactive chat session UI.

### Output conventions

```
Colours (dark theme):
  AI responses    → white (#FFFFFF)
  User prompts    → cyan (#00D7FF)
  Tool output     → green (#00FF87)
  Warnings        → yellow (#FFD700)
  Errors          → red (#FF5F5F)
  Dim/metadata    → grey (#626262)
  Follow-up hints → magenta (#FF79C6)

Spinners:
  While waiting for AI response → "Thinking..." with dots spinner
  While fetching GitHub data    → "Fetching..." with line spinner
  While searching               → "Searching..." with globe spinner

Tables:
  Task list, GitHub stats → lipgloss table with borders
  
Markdown:
  AI responses are rendered as terminal markdown (headers, bold, code blocks)
  using github.com/charmbracelet/glamour
```

### Chat session UI

```
╭─ Goo AI CLI ─────────────────────────────── session: a3f2c1 ─╮
│                                                               │
│  You: how does raft leader election work?                    │
│                                                              │
│  Goo: Raft leader election works in terms (each lasting      │
│       a random timeout between 150–300ms). When a follower   │
│       hasn't heard from a leader...                          │
│                                                              │
│  ↳ Want me to go deeper on split-vote scenarios?            │
│                                                              │
╰──────────────────────────────────────────────────────────────╯
  > |
  [Ctrl+C to exit]  [/search <q>]  [/tasks]  [/gh]  [/history]
```

### In-session slash commands

Within a `goo chat` session, slash commands provide quick access to tools:

```
/tasks                    → show task list inline
/tasks add "Deploy v2"    → add a task without leaving chat
/gh prs                   → show open PRs inline
/search <query>           → web search inline
/clear                    → clear the visible chat (history preserved)
/model llama-3.1-8b       → switch model mid-session
/export                   → export current session to markdown
/exit                     → end session
```

---

## 9. Error Handling Philosophy

### Principles

1. **Never crash silently.** All errors are surfaced to the user with a clear message and suggested action.
2. **Recover gracefully.** Network errors retry up to 3 times with exponential backoff before failing.
3. **Never lose history.** If an error occurs mid-response, the partial message is saved to history with a `[partial]` tag.
4. **Context errors are fatal.** If the master key can't be derived (wrong passphrase), Goo exits cleanly without attempting any API calls.

### Error categories

```go
type GooError struct {
    Code    ErrorCode
    Message string
    Hint    string     // shown to the user: "Try running: goo config set-key groq"
    Err     error      // underlying error (shown only in --debug mode)
}

type ErrorCode string

const (
    ErrKeyNotFound      ErrorCode = "KEY_NOT_FOUND"
    ErrKeyDecrypt       ErrorCode = "KEY_DECRYPT_FAILED"
    ErrAPITimeout       ErrorCode = "API_TIMEOUT"
    ErrAPIRateLimit     ErrorCode = "API_RATE_LIMIT"
    ErrNoHistory        ErrorCode = "NO_HISTORY"
    ErrTaskNotFound     ErrorCode = "TASK_NOT_FOUND"
    ErrGitHubUnauth     ErrorCode = "GITHUB_UNAUTHORIZED"
    ErrContextTooLarge  ErrorCode = "CONTEXT_TOO_LARGE"
)
```

### API error messages shown to users

```
✗ Groq API error: rate limit exceeded
  → You've hit Groq's rate limit. Wait 60 seconds and try again.
  → To reduce usage, try: goo config set model llama-3.1-8b-instant

✗ API key not found for provider: groq
  → Run: goo config set-key groq
  → Your key will be stored encrypted.

✗ GitHub token expired or invalid
  → Run: goo config set-key github
  → Or authenticate via: goo gh login
```

---

## 10. Extensibility: Custom APIs

Users can register their own AI providers (e.g., OpenAI, Ollama, local LLM).

### Registering a provider

```
$ goo config add-provider openai \
    --base-url https://api.openai.com/v1 \
    --model gpt-4o \
    --auth-header "Authorization: Bearer {key}"

→ Provider 'openai' registered. Set your key with:
  goo config set-key custom.openai
```

### Provider config (stored in config.toml)

```toml
[[custom_apis.providers]]
name         = "openai"
base_url     = "https://api.openai.com/v1"
model        = "gpt-4o"
auth_header  = "Authorization: Bearer {key}"
max_tokens   = 4096
```

### Using a custom provider

```
goo ask "Hello" --provider openai
goo chat --provider openai
goo config set default_provider openai   # persist as default
```

The `tools/custom/executor.go` handles the HTTP request, mapping Goo's internal `Message` format to the OpenAI-compatible chat completions format (which most providers use). Streaming is supported if the provider supports SSE.

### Ollama (local LLM) example

```toml
[[custom_apis.providers]]
name         = "ollama"
base_url     = "http://localhost:11434/v1"
model        = "mistral"
auth_header  = ""     # no auth needed for local
```

```
goo chat --provider ollama   # fully offline AI
```

---

## 11. Testing Strategy

### Unit tests

Every package has `_test.go` files. Key areas:

- `config/keys_test.go` — encrypt/decrypt round trips, wrong passphrase rejection, salt uniqueness
- `memory/context_test.go` — context window trimming, summary injection, token counting
- `tools/ai/followup_test.go` — signal detection accuracy across different response types
- `tools/tasks/manager_test.go` — CRUD operations, filtering, edge cases
- `core/router_test.go` — intent classification accuracy

### Integration tests

- `tools/ai/groq_test.go` — uses a mock HTTP server to simulate Groq streaming responses
- `tools/search/tavily_test.go` — mock server for search results
- `tools/github/client_test.go` — mock GitHub API responses

### Running tests

```
make test              # all tests
make test-unit         # unit tests only
make test-integration  # integration tests (requires mock servers)
make test-cover        # with coverage report
```

---

## 12. Starter Code

### main.go

```go
package main

import (
    "fmt"
    "os"

    "github.com/yourusername/goo/cmd"
)

var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)

func main() {
    cmd.SetVersion(version, commit, date)
    if err := cmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

### cmd/root.go

```go
package cmd

import (
    "github.com/spf13/cobra"
    "github.com/yourusername/goo/config"
)

var (
    version string
    cfgFile string
    debug   bool
    quiet   bool
    raw     bool
)

var rootCmd = &cobra.Command{
    Use:   "goo",
    Short: "Goo — AI CLI assistant for your terminal",
    Long: `Goo is a context-aware AI CLI assistant.
It combines AI chat, task management, GitHub tooling, and web search
in a single terminal application with persistent memory.`,
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        return config.Load(cfgFile)
    },
}

func Execute() error {
    return rootCmd.Execute()
}

func SetVersion(v, c, d string) {
    version = v
    // attach to version command
}

func init() {
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/goo/config.toml)")
    rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug output")
    rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-essential output")
    rootCmd.PersistentFlags().BoolVar(&raw, "raw", false, "raw output, no colours")

    rootCmd.AddCommand(chatCmd)
    rootCmd.AddCommand(askCmd)
    rootCmd.AddCommand(searchCmd)
    rootCmd.AddCommand(taskCmd)
    rootCmd.AddCommand(ghCmd)
    rootCmd.AddCommand(configCmd)
    rootCmd.AddCommand(historyCmd)
    rootCmd.AddCommand(versionCmd)
}
```

### cmd/chat.go

```go
package cmd

import (
    "github.com/spf13/cobra"
    "github.com/yourusername/goo/core"
    "github.com/yourusername/goo/memory"
)

var chatCmd = &cobra.Command{
    Use:   "chat",
    Short: "Start an interactive AI chat session",
    Long:  `Opens a persistent multi-turn chat session with context memory.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        store, err := memory.NewStore()
        if err != nil {
            return err
        }
        session, err := store.NewSession("chat")
        if err != nil {
            return err
        }
        return core.RunChatSession(session, store)
    },
}
```

### core/session.go

```go
package core

import (
    "bufio"
    "fmt"
    "os"
    "strings"

    "github.com/yourusername/goo/memory"
    "github.com/yourusername/goo/tools/ai"
    "github.com/yourusername/goo/core/renderer"
)

// RunChatSession starts and manages an interactive chat session.
func RunChatSession(session *memory.Session, store *memory.Store) error {
    r := renderer.New()
    groqClient, err := ai.NewGroqClient()
    if err != nil {
        return err
    }
    followup := ai.NewFollowUpAnalyser()
    ctx := buildContext(session, store)

    r.PrintSessionHeader(session)
    r.PrintHint("Type /exit to quit. Use /help for commands.")

    scanner := bufio.NewScanner(os.Stdin)
    for {
        r.PrintPrompt()
        if !scanner.Scan() {
            break
        }

        input := strings.TrimSpace(scanner.Text())
        if input == "" {
            continue
        }

        // Handle slash commands
        if strings.HasPrefix(input, "/") {
            if input == "/exit" || input == "/quit" {
                r.PrintInfo("Session saved. Goodbye!")
                return nil
            }
            if err := handleSlashCommand(input, session, store, r); err != nil {
                r.PrintError(err)
            }
            continue
        }

        // Save user message
        userMsg := memory.Message{
            Role:      "user",
            Content:   input,
            SessionID: session.ID,
        }
        if err := store.SaveMessage(userMsg); err != nil {
            return err
        }

        // Build context window
        messages := ctx.Build(input)

        // Stream response
        r.PrintAILabel()
        var fullResponse strings.Builder
        if err := groqClient.StreamChat(
            cmd.Context(),
            messages,
            r.StreamWriter(&fullResponse),
        ); err != nil {
            r.PrintError(fmt.Errorf("AI error: %w", err))
            continue
        }
        fmt.Println()

        // Save assistant message
        assistantMsg := memory.Message{
            Role:      "assistant",
            Content:   fullResponse.String(),
            SessionID: session.ID,
        }
        if err := store.SaveMessage(assistantMsg); err != nil {
            return err
        }

        // Analyse for follow-ups
        signals := followup.Analyse(fullResponse.String(), ctx.RecentMessages())
        for _, sig := range signals {
            if sig.AutoPrompt {
                r.PrintFollowUp(sig.Suggestion)
            }
        }
    }
    return nil
}
```

### memory/context.go

```go
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
```

### tools/ai/groq.go

```go
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
```

### tools/ai/followup.go

```go
package ai

import (
    "strings"
    "unicode"

    "github.com/yourusername/goo/memory"
)

type FollowUpSignal struct {
    Type       string
    Suggestion string
    AutoPrompt bool
}

type FollowUpAnalyser struct{}

func NewFollowUpAnalyser() *FollowUpAnalyser {
    return &FollowUpAnalyser{}
}

func (f *FollowUpAnalyser) Analyse(response string, history []memory.Message) []FollowUpSignal {
    var signals []FollowUpSignal

    trimmed := strings.TrimRightFunc(response, unicode.IsSpace)

    // Signal 1: Response ends with a question
    if strings.HasSuffix(trimmed, "?") {
        signals = append(signals, FollowUpSignal{
            Type:       "question",
            AutoPrompt: true,
        })
    }

    // Signal 2: Uncertainty language
    uncertaintyPhrases := []string{
        "i'm not sure", "it depends", "could you clarify",
        "not certain", "unclear", "you might want to check",
        "let me know if", "does that help",
    }
    lower := strings.ToLower(response)
    for _, phrase := range uncertaintyPhrases {
        if strings.Contains(lower, phrase) {
            signals = append(signals, FollowUpSignal{
                Type:       "uncertainty",
                Suggestion: "Want me to try a different angle, or can you give more context?",
                AutoPrompt: false,
            })
            break
        }
    }

    // Signal 3: Short response to a long / complex question
    if len(history) > 0 {
        lastUser := lastUserMessage(history)
        if wordCount(lastUser) > 20 && wordCount(response) < 80 {
            signals = append(signals, FollowUpSignal{
                Type:       "shallow",
                Suggestion: "Want me to go deeper on any part of this?",
                AutoPrompt: false,
            })
        }
    }

    return signals
}

func lastUserMessage(history []memory.Message) string {
    for i := len(history) - 1; i >= 0; i-- {
        if history[i].Role == "user" {
            return history[i].Content
        }
    }
    return ""
}

func wordCount(s string) int {
    return len(strings.Fields(s))
}
```

### config/keys.go

```go
package config

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "errors"
    "io"
    "os"
    "path/filepath"
    "sync"

    "golang.org/x/crypto/argon2"
    "golang.org/x/term"
    "fmt"
)

var (
    sessionKey     []byte
    sessionKeyOnce sync.Once
    sessionKeyMu   sync.Mutex
)

// GetAPIKey decrypts and returns an API key by slot name.
func GetAPIKey(slot string) (string, error) {
    key, err := getOrDeriveSessionKey()
    if err != nil {
        return "", err
    }
    return loadAndDecryptKey(slot, key)
}

// SetAPIKey encrypts and stores an API key.
func SetAPIKey(slot, value string) error {
    key, err := getOrDeriveSessionKey()
    if err != nil {
        return err
    }
    return encryptAndStoreKey(slot, value, key)
}

func getOrDeriveSessionKey() ([]byte, error) {
    sessionKeyMu.Lock()
    defer sessionKeyMu.Unlock()
    if sessionKey != nil {
        return sessionKey, nil
    }

    salt, err := loadOrCreateSalt()
    if err != nil {
        return nil, err
    }

    fmt.Print("Enter Goo passphrase: ")
    passphrase, err := term.ReadPassword(int(os.Stdin.Fd()))
    fmt.Println()
    if err != nil {
        return nil, err
    }

    derived := argon2.IDKey(passphrase, salt, 3, 64*1024, 4, 32)
    sessionKey = derived
    return derived, nil
}

func loadOrCreateSalt() ([]byte, error) {
    saltPath := filepath.Join(gooConfigDir(), "salt")
    data, err := os.ReadFile(saltPath)
    if err == nil {
        return data, nil
    }
    salt := make([]byte, 32)
    if _, err := rand.Read(salt); err != nil {
        return nil, err
    }
    if err := os.MkdirAll(gooConfigDir(), 0700); err != nil {
        return nil, err
    }
    return salt, os.WriteFile(saltPath, salt, 0600)
}

func encryptAndStoreKey(slot, value string, masterKey []byte) error {
    block, _ := aes.NewCipher(masterKey)
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    io.ReadFull(rand.Reader, nonce)
    ciphertext := gcm.Seal(nonce, nonce, []byte(value), nil)

    store := loadKeyStore()
    store[slot] = base64.StdEncoding.EncodeToString(ciphertext)
    return saveKeyStore(store)
}

func loadAndDecryptKey(slot string, masterKey []byte) (string, error) {
    store := loadKeyStore()
    encoded, ok := store[slot]
    if !ok {
        return "", errors.New("key not found for slot: " + slot)
    }
    ciphertext, err := base64.StdEncoding.DecodeString(encoded)
    if err != nil {
        return "", err
    }
    block, _ := aes.NewCipher(masterKey)
    gcm, _ := cipher.NewGCM(block)
    nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", errors.New("decryption failed: wrong passphrase?")
    }
    return string(plaintext), nil
}

func loadKeyStore() map[string]string {
    store := map[string]string{}
    data, err := os.ReadFile(keyStorePath())
    if err != nil {
        return store
    }
    json.Unmarshal(data, &store)
    return store
}

func saveKeyStore(store map[string]string) error {
    data, _ := json.Marshal(store)
    return os.WriteFile(keyStorePath(), data, 0600)
}

func keyStorePath() string { return filepath.Join(gooConfigDir(), "keys.enc") }
func gooConfigDir() string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".config", "goo")
}
```

### go.mod

```
module github.com/yourusername/goo

go 1.22

require (
    github.com/spf13/cobra          v1.8.0
    github.com/spf13/viper          v1.18.0
    github.com/charmbracelet/bubbletea  v0.27.0
    github.com/charmbracelet/lipgloss   v0.12.0
    github.com/charmbracelet/glamour    v0.7.0
    github.com/mattn/go-sqlite3     v1.14.22
    golang.org/x/crypto             v0.27.0
    golang.org/x/term               v0.24.0
)
```

---

## 13. Makefile & Tooling

```makefile
BINARY    := goo
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE      := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS   := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: build run test test-unit test-cover lint clean install

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./

run:
	go run $(LDFLAGS) . $(ARGS)

install:
	go install $(LDFLAGS) ./

test:
	go test ./...

test-unit:
	go test ./... -short

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ coverage.out coverage.html

# Build for multiple platforms
release:
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64   ./
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64  ./
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe ./

# First time setup
setup:
	go mod tidy
	mkdir -p bin dist
	@echo "Run 'make build' to compile."

# Database migrations
db-migrate:
	sqlite3 ~/.config/goo/history.db < memory/schema.sql
```

---

## 14. Roadmap

### v0.1 — Core (build first)
- [x] Project scaffold, cobra commands
- [x] Config system + encrypted key store
- [x] Groq streaming client
- [x] Basic `goo ask` and `goo chat`
- [x] SQLite memory store
- [x] Context window builder

### v0.2 — Memory & follow-ups
- [ ] Session persistence and resume (`goo history`)
- [ ] Follow-up detector
- [ ] Session summariser (long-session compression)
- [ ] Topic shift detection

### v0.3 — Tools
- [ ] Task manager with SQLite backend
- [ ] GitHub tool (stats, PRs, issues)
- [ ] Tavily web search
- [ ] AI tool invocation (function calling via Groq)

### v0.4 — UX
- [ ] Bubbletea interactive chat UI
- [ ] Lipgloss / Glamour rendering
- [ ] Slash commands in chat
- [ ] In-session task and GitHub commands

### v0.5 — Extensibility
- [ ] Custom API provider registry
- [ ] Ollama / OpenAI-compatible providers
- [ ] Plugin system (future)

### v1.0 — Release
- [ ] Full test suite (>80% coverage)
- [ ] Cross-platform releases (goreleaser)
- [ ] Man page generation
- [ ] Homebrew formula
- [ ] Installation script

---

*CLAUDE.md — Goo AI CLI. Last updated: 2025.*