# Goo!!! YOUR CLI AI ASSISTANT

> A terminal-first AI assistant built with Go. Not a chatbot wrapper — a professional-grade **agentic coding environment** powered by AI with Model Context Protocol (MCP) integration, parallel sub-agent orchestration, 3-tier safety gating, persistent memory, and task management baked right in.

[![Go](https://github.com/kingjethro999/goo/actions/workflows/go.yml/badge.svg)](https://github.com/kingjethro999/goo/actions/workflows/go.yml)
[![Release](https://img.shields.io/github/v/release/kingjethro999/goo)](https://github.com/kingjethro999/goo/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [First-Time Setup](#first-time-setup)
- [Model Context Protocol (MCP)](#model-context-protocol-mcp)
- [Parallel Sub-Agent Orchestration](#parallel-sub-agent-orchestration)
- [3-Tier Safety Policy](#3-tier-safety-policy)
- [Agent Skills Engine](#agent-skills-engine)
- [Commands](#commands)
  - [goo chat](#goo-chat)
  - [goo ask](#goo-ask)
  - [goo agent](#goo-agent)
  - [goo mcp](#goo-mcp)
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
| 📡 **Model Context Protocol (MCP)** | Seamlessly connect to external tools & data sources using standard JSON-RPC over `stdio` and `http` transports |
| 🤖 **Parallel Sub-Agent Orchestration** | Decompose complex engineering tasks into non-overlapping file scopes executed concurrently by specialized sub-agents with automatic conflict merging |
| 🛡 **3-Tier Safety Policy** | Professional-grade security gating (`always_confirm`, `allowlist_only`, `autonomous`) over file edits and terminal execution |
| 🧠 **Persistent Memory** | Goo remembers your entire conversation history within and across sessions using SQLite |
| 🔄 **Session Summarisation** | Long conversations are automatically summarised in the background so context is never lost |
| 📊 **Real-Time Agent Feed** | Live action tracking & step-by-step reasoning feed (`Thought for 1s >`, `Analyzed 📄 file #L1-40`, `Run ⚡ go test`) |
| 🎯 **Agent Skills Engine** | Modular slash commands & prompt recipes defined in `.goo/skills`, `.claude/skills`, or `~/.config/goo/skills` |
| 💻 **Agentic Coding** | Read and edit files with precision line-scoped replacement, directory listing, and deep regex grep search |
| 🛠 **Autonomous Tools** | The AI can call tools (file reading/writing, shell commands, grep, search, tasks, GitHub, and MCP tools) on its own |
| 🛡 **Git Safety Gating** | Automatic risk tier classification (Safe, Low, Moderate, High) with Git stash checkpoints for safe undos |
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

# GitHub integration (required for goo gh)
goo config set-key github
```

**Step 3 — Start chatting:**
```bash
goo chat
```

---

## Model Context Protocol (MCP)

Goo natively supports the **Model Context Protocol (MCP)**, allowing you to connect external data sources, custom servers, and specialized toolboxes using JSON-RPC 2.0 over `stdio` or HTTP transports.

### Configuring MCP Servers
Add your MCP servers directly to `~/.config/goo/config.toml`:

```toml
[mcp_servers.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem", "/home/user"]
env = { NODE_ENV = "production" }
enabled = true

[mcp_servers.internal_api]
url = "http://localhost:8080/mcp"
headers = { Authorization = "Bearer ${API_KEY}" }
enabled = true
```

Once configured, Goo automatically discovers and registers all tools exposed by connected MCP servers, making them available to the AI agent during interactive chat and parallel orchestration.

---

## Parallel Sub-Agent Orchestration

Goo features an advanced orchestration engine capable of breaking down complex goals into parallel, non-overlapping sub-tasks.

### How It Works
1. **Goal Decomposition**: The Lead Agent analyzes your objective and decomposes it into independent sub-tasks, assigning precise file scopes (`Scope: ["internal/auth/*.go"]`) to prevent race conditions.
2. **Bounded Execution**: Sub-agents execute concurrently within a bounded semaphore pool (`max_parallel_agents = 4`).
3. **Scope Narrowing**: Each sub-agent is gated by a narrowed safety policy that blocks file modifications outside its assigned scope.
4. **Conflict Merging**: Once sub-agents complete, the orchestrator detects concurrent modifications, flags file-level conflicts, and aggregates results into a clean unified diff.

```bash
# Run orchestration on a goal directly
goo agent run "Refactor logging across auth and api packages to use slog"

# Run from a structured tasks file
goo agent run --tasks tasks.yaml
```

---

## 3-Tier Safety Policy

To ensure you stay in complete control of what ships, Goo implements a strict 3-tier safety policy configurable via `~/.config/goo/config.toml` or `goo config`:

| Policy Mode | Behavior |
|---|---|
| `always_confirm` *(Default)* | Every file modification and bash command prompts for interactive user confirmation in the TUI before execution. |
| `allowlist_only` | Safe commands matching glob patterns in `allowlist` (e.g., `git status`, `npm test`, `write_file`) auto-execute; anything else prompts for confirmation. |
| `autonomous` | Full agentic mode. The AI can execute tools, edit files, and run commands autonomously without interrupting for approval. |

```toml
[agent]
safety_policy = "always_confirm"
allowlist = ["npm install", "npm run *", "git add", "git status", "git diff", "write_file"]
```

---

## Agent Skills Engine

Goo automatically scans for and loads custom **Agent Skills** at the beginning of every session. Skills allow you to define modular prompt recipes, custom slash triggers, domain-specific coding guidelines, and custom workflows.

### Skill Locations
Goo checks three locations (project-specific & global) in order:
1. `.goo/skills/` (project root)
2. `.claude/skills/` (compatibility with Claude CLI skills)
3. `~/.config/goo/skills/` (global user skills available across all projects)

### Creating a Custom Skill
Each skill is a `.md` Markdown file containing YAML frontmatter:

```markdown
---
name: code-review
description: Perform strict security and performance code review
trigger: /review
---

# Code Review Protocol
When performing a code review:
1. Check for unsanitized user inputs or SQL/Command injection risks.
2. Verify error handling for all Go function calls.
3. Suggest performance optimizations without breaking existing tests.
```

---

## Commands

### `goo chat`
Opens the interactive AI chat TUI. Features real-time streaming, a scrollable viewport, multiline input, and autonomous tool use.
```bash
goo chat
```

### `goo ask`
Ask a single question and get a response without entering the full TUI. Ideal for one-shot terminal queries.
```bash
goo ask "what is the capital of Nigeria?"
goo ask "summarise the last git commit in this folder"
```

### `goo agent`
Manage and execute parallel sub-agent orchestration.
```bash
# Decompose and run parallel sub-agents for a goal
goo agent run "Add unit tests for all handlers in internal/api"

# Inspect active and historical sub-agent status
goo agent status

# Stop a running sub-agent or all active agents
goo agent stop --all
```

### `goo mcp`
Manage and inspect configured Model Context Protocol (MCP) servers.
```bash
# List all configured servers, transport types, and exposed tools
goo mcp list

# Check connection health status of MCP servers
goo mcp status
```

### `goo find`
An extensive deep search across your home directory to find any file or folder, no matter how deep or misplaced.
```bash
goo find invoice march
goo find config.toml
```

### `goo task`
A fully offline, SQLite-backed task manager. The AI can read and modify your tasks autonomously during chat.
```bash
goo task add "Finish the project report" --priority high
goo task list
goo task done 3
```

### `goo search`
Perform a real-time web search using the Tavily API, returning an AI-summarized answer directly in the terminal.
```bash
goo search "latest AI news"
```

### `goo gh`
GitHub integration commands. Requires a GitHub personal access token set via `goo config set-key github`.
```bash
goo gh prs
goo gh stats
```

### `goo history`
Browse and resume past conversation sessions.
```bash
goo history list
goo history show [session-id]
goo history resume [session-id]
```

### `goo config`
Manage your Goo configuration and API keys.
```bash
goo config set-key groq
goo config list-keys
```

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
2. **Auto-summarisation** — every 40 messages, the older portion of the conversation is automatically summarised by a fast model in the background. The summary is injected as context at the start of future requests so nothing is lost, even in very long sessions.

---

## Building from Source

Requirements: Go 1.21+, GCC (for SQLite CGO)

```bash
git clone https://github.com/kingjethro999/goo.git
cd goo

# Build binary
make build

# Run tests
make test
```

---

*Goo AI CLI — v2.0.1 · Built with Go, Bubbletea, SQLite, MCP, and Groq*
