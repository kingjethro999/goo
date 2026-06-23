# Goo AI CLI

Goo is a terminal-first AI assistant built with Go. It is not a chatbot wrapper — it is a **tool** that happens to be powered by AI. The design philosophy focuses on context persistence, follow-up awareness, and deep integration with tasks, GitHub, and web search.

## Features
- **Context persistence**: Goo remembers your entire session history and context.
- **Follow-up awareness**: If an AI response is ambiguous, Goo prompts you.
- **Tool integration**: Tasks, GitHub, and web search are first-class features.
- **Offline-first UX**: All non-AI features work without an internet connection.
- **User-controlled keys**: Bring your own Groq and Tavily keys, secured by AES-256 encryption.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/kingjethro999/goo/main/install.sh | sh
```

Or download binary releases from the GitHub Releases page.

## Getting Started

1. Set your API keys:
   ```bash
   goo config set-key groq
   goo config set-key tavily
   goo config set-key github
   ```

2. Start an interactive session:
   ```bash
   goo chat
   ```

## Commands & Usage

Goo offers a suite of powerful commands to streamline your workflow:

### Chat & AI
- `goo chat` — Starts a persistent, interactive AI chat session. Goo remembers previous conversations and utilizes background tasks and tools.
- `goo ask [question]` — Asks a quick, single-shot question without entering the interactive TUI. Ideal for quick terminal queries.
- `goo history list` — View your past conversation sessions.

### System Tools
- `goo find [query]` — **Goo Find**: An extensive, deep search across your system to find anything misplaced. Uses a heuristic fuzzy-matching algorithm to instantly scan your home directory for files. 
  Example: `goo find my resume`

### Task Management
Goo features a fully offline, SQLite-backed task manager that the AI can read and modify.
- `goo task add "Task description" --priority high` — Create a new task.
- `goo task list` — List all open tasks.
- `goo task done [ID]` — Mark a task as completed.

### Web Search
- `goo search "Latest AI news"` — Perform a fast web search using Tavily and return a clean, AI-summarized answer directly in your terminal.

### GitHub Integration
- `goo gh prs` — List your open Pull Requests.
- `goo gh stats` — View your GitHub contribution statistics.
