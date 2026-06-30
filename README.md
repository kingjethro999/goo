# Goo!!! YOUR CLI AI ASSISTANT

> A terminal-first AI assistant built with Go. Not a chatbot wrapper — a **tool** powered by AI with persistent memory, task management, deep file search, GitHub integration, and live web search baked right in.

[![Go](https://github.com/kingjethro999/goo/actions/workflows/go.yml/badge.svg)](https://github.com/kingjethro999/goo/actions/workflows/go.yml)
[![Release](https://img.shields.io/github/v/release/kingjethro999/goo)](https://github.com/kingjethro999/goo/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [First-Time Setup](#first-time-setup)
- [Commands](#commands)
  - [goo chat](#goo-chat)
  - [goo ask](#goo-ask)
  - [goo find](#goo-find)
  - [goo task](#goo-task)
  - [goo search](#goo-search)
  - [goo gh](#goo-gh)
  - [goo history](#goo-history)
  - [goo config](#goo-config)
  - [goo version](#goo-version)
- [Security & Passphrase](#security--passphrase)
- [API Keys](#api-keys)
- [How Memory Works](#how-memory-works)
- [Building from Source](#building-from-source)

---

## Features

| Feature | Description |
|---|---|
| 🧠 **Persistent Memory** | Goo remembers your entire conversation history within and across sessions using SQLite |
| 🔄 **Session Summarisation** | Long conversations are automatically summarised in the background so context is never lost |
| 🛠 **Autonomous Tools** | The AI can call tools (search, tasks, GitHub) on its own — no slash commands needed |
| 📋 **Task Manager** | Fully offline SQLite-backed task manager the AI reads and writes to |
| 🔍 **Deep File Search** | Scan your entire home directory to find any file, instantly |
| 🌐 **Web Search** | Real-time web search via Tavily with AI-summarized results |
| 🐙 **GitHub Integration** | View open PRs and contribution stats from your terminal |
| 🔒 **Encrypted Keystore** | All API keys are encrypted with AES-256-GCM using a passphrase you set. Nothing is stored in plain text |
| ⚡ **Streaming TUI** | Real-time streaming responses in a beautiful Bubbletea terminal UI |
| 🚀 **Single Binary** | One binary, no runtime dependencies, cross-platform |

---

## Installation

**Via install script (Linux/macOS):**
```bash
curl -fsSL https://raw.githubusercontent.com/kingjethro999/goo/main/install.sh | bash
```
> **Note:** After installation, you may need to restart your terminal or refresh your path (e.g., `source ~/.bashrc` or `source ~/.zshrc`) for the `goo` command to be recognized.


**Manual download:**
Download the pre-built binary for your platform from the [Releases page](https://github.com/kingjethro999/goo/releases).

**From source:**
```bash
git clone https://github.com/kingjethro999/goo.git
cd goo
make build
sudo install -m 755 bin/goo /usr/local/bin/goo
```

---

## First-Time Setup

On your first run, Goo will ask you to create a **passphrase**. This passphrase is used to encrypt your API keys on disk. **It is never stored anywhere — remember it.**

**Step 1 — Set your Groq API key** (required for AI features):
```bash
goo config set-key groq
# Enter API key for groq: [paste your key]
# Enter Goo passphrase: [choose a passphrase]
```
Get a free Groq key at: https://console.groq.com

**Step 2 — Set optional keys for more features:**
```bash
# Web search (required for goo search and AI web tool)
goo config set-key tavily
# Get a free Tavily key at: https://app.tavily.com

# GitHub integration (required for goo gh)
goo config set-key github
# Create a token at: https://github.com/settings/tokens
```

**Step 3 — Start chatting:**
```bash
goo chat
```

---

## Commands

### `goo chat`

Opens the interactive AI chat TUI. Features real-time streaming, a scrollable viewport, multiline input, and autonomous tool use.

```bash
goo chat
```

**Inside the chat:**
| Key | Action |
|---|---|
| `Enter` | Send message |
| `Alt+Enter` | Insert newline (multiline input) |
| `Ctrl+C` | Quit |

The AI can call tools automatically during a chat session. When it does, you will see a status indicator like `⚙ Calling tool: search_web...` and the result is injected back into the conversation seamlessly.

---

### `goo ask`

Ask a single question and get a response without entering the full TUI. Ideal for one-shot terminal queries.

```bash
goo ask "what is the capital of Nigeria?"
goo ask "summarise the last git commit in this folder"
goo ask what time is it in Tokyo right now
```

---

### `goo find`

**Goo Find** — an extensive deep search across your home directory to find any file or folder, no matter how deep or misplaced.

```bash
goo find [query]
```

**Examples:**
```bash
goo find my resume
goo find invoice march
goo find config.toml
goo find notes from meeting
```

**How it works:**
- Scans your entire `$HOME` directory recursively
- Skips heavy system/dev folders (`.git`, `node_modules`, `vendor`, `.cache`) for speed
- Uses a heuristic scoring system: files with names matching all keywords score highest, with bonus points for exact matches
- Returns the top 10 matches with path highlighting
- Shows total items scanned and time taken

---

### `goo task`

A fully offline, SQLite-backed task manager. The AI can read and modify your tasks autonomously during chat.

```bash
# Add a task
goo task add "Finish the project report"
goo task add "Buy groceries" --priority high

# List all open tasks
goo task list

# Mark a task as done (use the ID shown in list)
goo task done 3
```

**Priority levels:** `low`, `medium` (default), `high`

The AI is aware of your open tasks during every chat session and can:
- Add tasks on your behalf ("remind me to call back John")
- List your current tasks when asked
- Mark tasks complete when you say you're done with something

---

### `goo search`

Perform a real-time web search using the Tavily API, returning an AI-summarized answer directly in the terminal.

```bash
goo search "latest AI news"
goo search "how to reverse a string in Go"
goo search "Nigeria tech ecosystem 2025"
```

**Requires:** `tavily` API key set via `goo config set-key tavily`

---

### `goo gh`

GitHub integration commands. Requires a GitHub personal access token set via `goo config set-key github`.

```bash
# List your open pull requests
goo gh prs

# View your contribution stats
goo gh stats
```

Create a GitHub token at: https://github.com/settings/tokens (only `repo` and `read:user` scopes needed)

---

### `goo history`

Browse and resume past conversation sessions.

```bash
# List all past sessions
goo history list

# Show messages from a specific session
goo history show [session-id]

# Resume a past session
goo history resume [session-id]
```

---

### `goo config`

Manage your Goo configuration and API keys.

```bash
# Store an API key (encrypted)
goo config set-key groq
goo config set-key tavily
goo config set-key github

# List all stored key slots (names only, not values)
goo config list-keys
```

---

### `goo version`

Prints the current version, commit hash, and build date.

```bash
goo version
```

---

## Security & Passphrase

Goo uses a **passphrase-based encrypted keystore** to protect your API keys.

**How it works:**
1. On first setup, a random 32-byte `salt` is generated and saved at `~/.config/goo/salt`
2. When you set a key, Goo prompts for your passphrase and derives a 256-bit AES key using `argon2id` (memory-hard, resistant to brute-force)
3. Each key is encrypted with `AES-256-GCM` (authenticated encryption) and stored at `~/.config/goo/keys.enc`
4. **Your passphrase is never saved anywhere** — it only lives in memory for the duration of the current command

**If you forget your passphrase:**
There is no recovery mechanism. This is by design. To reset:
```bash
rm ~/.config/goo/salt ~/.config/goo/keys.enc
# Then re-run: goo config set-key groq
```

**Fallback keys:**
Goo ships with built-in developer keys as a fallback for users who haven't set up their own keys yet. Once you set your own key for a given slot, it always takes priority over the fallback.

---

## API Keys

| Key Slot | Used For | Where to Get |
|---|---|---|
| `groq` | All AI features (chat, ask, summarisation) | https://console.groq.com |
| `tavily` | Web search (`goo search`, AI search tool) | https://app.tavily.com |
| `github` | GitHub PR and stats commands | https://github.com/settings/tokens |

---

## How Memory Works

Goo uses a two-layer memory system backed by SQLite (`~/.config/goo/tasks.db`):

1. **Full session history** — every message in a session is stored and included in each request (up to token budget)
2. **Auto-summarisation** — every 40 messages, the older portion of the conversation is automatically summarised by a fast model (`llama-3.1-8b-instant`) in the background. The summary is injected as context at the start of future requests so nothing is lost, even in very long sessions

**Context files:**
| Path | Contents |
|---|---|
| `~/.config/goo/tasks.db` | All sessions, messages, tasks |
| `~/.config/goo/keys.enc` | Encrypted API keys |
| `~/.config/goo/salt` | Argon2 salt for key derivation |
| `~/.config/goo/config.toml` | App configuration |

---

## Building from Source

Requirements: Go 1.21+, GCC (for SQLite CGO)

```bash
git clone https://github.com/kingjethro999/goo.git
cd goo

# Build binary
make build
# Binary is at bin/goo

# Run tests
make test

# Generate man page
make man

# Install man page to system
make install-man
```

---

*Goo AI CLI — v1.0 · Built with Go, Bubbletea, SQLite, and Groq*
