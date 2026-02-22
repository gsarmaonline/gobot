# Gobot

A minimal Go-based agent orchestrator that listens to Telegram messages and delegates them to **Claude Code CLI** (`claude`) running locally. Claude Code handles the agentic loop — tool use, planning, execution — while gobot handles the plumbing: receiving messages, invoking `claude`, and streaming results back.

## Architecture

```
Telegram msg → Orchestrator → claude -p "<msg>" --output-format stream-json
                                     ↓
                         stream JSON events line-by-line
                                     ↓
             chunk type == "text"      →  buffer → send to Telegram (every 2s)
             chunk type == "tool_use"  →  send "_Using tool: Bash..._" to Telegram
             type == "result"          →  extract session_id, flush buffer, done
```

Multi-turn conversations are supported automatically via `--resume <session_id>` — each chat (or Telegram thread) gets its own isolated Claude Code session.

## Setup

### Prerequisites

- Go 1.24+
- [`claude` CLI](https://github.com/anthropics/claude-code) installed and authenticated (`ANTHROPIC_API_KEY` set)
- A Telegram bot token (create via [@BotFather](https://t.me/BotFather))

### Install & run

```bash
git clone https://github.com/gsarma/gobot
cd gobot

cp .env.example .env
# edit .env — at minimum set TELEGRAM_TOKEN

go run ./cmd/gobot/
```

### Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `TELEGRAM_TOKEN` | ✅ | — | Telegram bot token |
| `ALLOWED_CHAT_IDS` | | (all) | Comma-separated int64 chat IDs to allow |
| `CLAUDE_PATH` | | `claude` | Path to the `claude` binary |
| `CLAUDE_MODEL` | | `claude-opus-4-6` | Model to use |
| `CLAUDE_ALLOWED_TOOLS` | | `Bash,Read,Edit,Write,Glob,Grep` | Tools Claude may use |
| `CLAUDE_MAX_BUDGET_USD` | | `2.00` | Per-request budget cap |
| `EXEC_TIMEOUT` | | `5m` | Timeout per request |
| `WORK_DIR` | | `$PWD` | Working directory for `claude` |

## Usage

1. Start gobot
2. Send a message to your bot: `list files in /tmp`
3. gobot shows a typing indicator, then streams the response back as Claude works
4. Send a follow-up — the session is automatically resumed

## Testing

Unit tests:

```bash
go test ./...
```

Smoke test the Claude executor directly (no Telegram needed):

```bash
# default prompt
go run ./cmd/smoketest/

# custom prompt
go run ./cmd/smoketest/ "list files in /tmp and summarise"
```

Prints each streamed chunk as it arrives, shows tool invocations, and reports elapsed time and session ID on completion.

## Project layout

```
cmd/gobot/main.go                      # entry point
cmd/smoketest/main.go                  # CLI smoke test for the Claude executor
internal/
  config/config.go                     # env-var config
  provider/
    provider.go                        # Provider interface + message types
    telegram/telegram.go               # Telegram long-poll implementation
  executor/
    executor.go                        # Executor interface
    claude/claude.go                   # Claude Code CLI implementation
  orchestrator/orchestrator.go         # ties provider + executor together
```
