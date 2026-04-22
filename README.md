# lm-bridge

[English](README.md) | [Русский](README.ru.md)

A macOS menubar app + CLI that connects [Claude Code](https://claude.ai/claude-code) to a local LLM running in [LM Studio](https://lmstudio.ai).

Offload mechanical tasks (code search, boilerplate generation, transformations) to a local model — keeping Claude's context free for reasoning.

![lm-bridge dashboard](docs/screenshot.png)

## Features

- **Menubar app** — live dashboard showing recent calls, token usage, latency, active task progress
- **CLI** — `query`, `agent`, `review`, `explain` commands
- **Claude Code integration** — injects usage instructions into your `CLAUDE.md` automatically
- **Agent mode** — local model reads files via tool calls, no manual copy-paste
- **Code review** — review git diff before committing
- **File explain** — get a structured explanation of any file
- **Streaming** — `--stream` flag with loop detection (catches stuck generations)
- **Active task tracking** — progress bar + cancel button while model is generating
- **Busy detection** — fast pre-flight check prevents disrupting running generations

## Requirements

- macOS (Apple Silicon recommended)
- [LM Studio](https://lmstudio.ai) running locally
- A loaded model (tested with Qwen3.6-35B-A3B)

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

## Claude Code Integration

lm-bridge can automatically configure Claude Code to use the local model.

**Enable via the dashboard:** open `lm-bridge.app`, click **Enable** next to "Claude Code Integration". This injects a usage block into `~/.claude/CLAUDE.md` — Claude will know when and how to delegate tasks to the local model.

**Or add manually** to your `~/.claude/CLAUDE.md`:

```markdown
## Local LLM Helper (lm-bridge)

A local model is available via `lm-bridge`. Always ask the user if LM Studio is running before using it.

### When to delegate

Delegate tasks where the result is deterministic, easy to verify, or reversible:
- Search & collect: "find all files importing X", "list all TODO comments"
- Boilerplate: "generate CRUD endpoints for model Y"
- Transforms: "translate comments to English", "add JSDoc to all exports"
- CI tasks: "run tests and return failed ones with error messages"

### When NOT to delegate

- Debugging non-trivial logic
- Architectural decisions
- Anything security-related
- Tasks where errors are hard to detect

### How to call

```bash
# Agent mode — model reads files via tool calls:
lm-bridge agent --dir /path/to/project "task"

# Simple query (stdin supported):
lm-bridge query "request"
cat file.txt | lm-bridge query "summarize this"

# Review git diff before committing:
lm-bridge review
lm-bridge review --staged

# Explain a file:
lm-bridge explain path/to/file.go

# Streaming with loop detection:
lm-bridge query --stream "request"

# Enable reasoning for complex tasks:
lm-bridge agent --think --dir . "task"
```

### Concurrency

LM Studio handles one request at a time. If a generation is already running — do NOT call lm-bridge, it will interrupt it.
- Error "LM Studio is busy" — wait for the current task to finish
- Progress is shown in the lm-bridge dashboard
```

## Usage

### Menubar app

Launch `lm-bridge.app` — it lives in the menubar. Click to open the dashboard.

### CLI

```bash
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
Claude Code  →  lm-bridge CLI  →  LM Studio (local model)
                     ↕
              SQLite (shared state)
                     ↕
              lm-bridge.app (dashboard)
```

- CLI and GUI share a SQLite database for call history and active task state
- Agent mode uses OpenAI-compatible tool calls for file reading
- Dashboard polls active task every 2s, shows real-time progress from LM Studio server logs

## Building a release

```bash
./build.sh v1.0.0
# Binary: build/bin/lm-bridge.app
```

## License

MIT
