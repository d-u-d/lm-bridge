# lm-bridge

[English](README.md) | [Русский](README.ru.md)

A macOS menubar app + CLI that connects [Claude Code](https://claude.ai/claude-code) to an LLM — either a local model via [LM Studio](https://lmstudio.ai) or a cloud model via [OpenRouter](https://openrouter.ai).

Offload mechanical tasks (code search, boilerplate generation, transformations) to a secondary model — keeping Claude's context free for reasoning.

![lm-bridge dashboard](docs/screenshot.png)

## Features

- **Menubar app** — live dashboard showing recent calls, token usage, latency, active task progress
- **CLI** — `query`, `agent`, `review`, `explain`, `status` commands
- **Provider settings** — switch between LM Studio (local) and OpenRouter (cloud) in the dashboard
- **Claude Code integration** — injects usage instructions into your `CLAUDE.md` automatically
- **Agent mode** — model reads files via tool calls, no manual copy-paste
- **Code review** — review git diff before committing
- **File explain** — get a structured explanation of any file
- **Streaming** — `--stream` flag with loop detection (catches stuck generations)
- **Active task tracking** — progress bar + cancel button while model is generating
- **Call history** — provider and model shown for every call

## Requirements

- macOS (Apple Silicon recommended)
- One of:
  - [LM Studio](https://lmstudio.ai) running locally with a loaded model
  - [OpenRouter](https://openrouter.ai) API key (free models available)

## Installation

### Download

Grab the latest `.app` from [Releases](https://github.com/d-u-d/lm-bridge/releases).

### Build from source

```bash
# Prerequisites: Go 1.22+, Wails v2, Node.js 18+
go install github.com/wailsapp/wails/v2/cmd/wails@latest

git clone https://github.com/d-u-d/lm-bridge
cd lm-bridge
./build.sh
```

## Setup

### 1. Configure provider

Open `lm-bridge.app` and click **⚙ Settings**:

- **LM Studio** — set URL (default: `http://localhost:1234/v1`)
- **OpenRouter** — paste your API key, click "Load free models", pick a model

### 2. Verify connection

```bash
lm-bridge status
# Provider:  openrouter
# Model:     google/gemma-3-12b-it:free
# Status:    ✓ ready
```

### 3. Enable Claude Code integration (optional)

In the dashboard, click **Enable** next to "Claude Code Integration". This injects a usage block into `~/.claude/CLAUDE.md` so Claude knows when and how to delegate tasks.

**Trigger phrase:** say **"привлеки помощника"** to Claude — it will check status and delegate the task automatically.

## Usage

### CLI

```bash
# Check provider and connection status
lm-bridge status

# Simple query (stdin supported)
lm-bridge query "explain this" < file.txt

# Agent mode — model reads files itself via tool calls
lm-bridge agent --dir /path/to/project "find all TODO comments"

# Review git diff before committing
lm-bridge review
lm-bridge review --staged   # staged changes only

# Explain a file
lm-bridge explain internal/cli/agent.go
cat main.go | lm-bridge explain

# Streaming output with loop detection
lm-bridge query --stream "write a long explanation of..."

# Enable reasoning for complex tasks
lm-bridge agent --think --dir . "analyze this module"
```

### Example workflows

```bash
# Find all places a variable is used
lm-bridge agent --dir . "find all usages of DATABASE_URL env variable"

# Generate boilerplate
lm-bridge agent --dir . "create CRUD endpoints for the User model following existing patterns"

# Transform content
cat api.go | lm-bridge query "add godoc comments to all exported functions, return only the modified file"

# Quick code review before git commit
lm-bridge review --staged
```

## How it works

```
Claude Code  →  lm-bridge CLI  →  LM Studio (local)
                     ↕               or
              SQLite (shared)    OpenRouter (cloud)
                     ↕
              lm-bridge.app (dashboard)
```

- CLI and GUI share a SQLite database for call history, settings, and active task state
- Agent mode uses OpenAI-compatible tool calls for file reading
- Dashboard shows provider, model, and latency for every call

## Building a release

```bash
./build.sh v0.6.0
# Binary: build/bin/lm-bridge.app
```

## License

MIT
