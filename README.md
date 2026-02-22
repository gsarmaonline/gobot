# Gobot

A minimal Go-based agent orchestrator that listens for messages from a **provider** (Telegram or Linear) and delegates them to **Claude Code CLI** (`claude`) running locally. Claude Code handles the agentic loop — tool use, planning, execution — while gobot handles the plumbing: receiving events, invoking `claude`, and routing results back.

## Architecture

### Telegram provider (streaming)

```
Telegram msg → Orchestrator → claude -p "<msg>" --output-format stream-json
                                     ↓
                         stream JSON events line-by-line
                                     ↓
             chunk type == "text"      →  buffer → send to Telegram (every 2s)
             chunk type == "tool_use"  →  send "_Using tool: Bash..._" to Telegram
             type == "result"          →  extract session_id, flush buffer, done
```

### Linear provider (batch)

```
Linear "In Progress" webhook → HTTP server (linear provider)
  → verify HMAC-SHA256 signature
  → fetch full issue from Linear GraphQL API
  → build prompt (title + description + branch/commit/PR instructions)
  → Orchestrator → Claude executor (in team's repo dir)
  → Claude: branch, implement, commit, push, gh pr create
  → extract PR URL from Claude output
  → post PR URL as comment on Linear issue

Linear "Done" webhook → HTTP server → emit "merge" action
  → Orchestrator looks up session → gets stored PR URL
  → gh pr merge --squash <url>
  → post merge confirmation comment on Linear issue
```

Multi-turn conversations are supported via `--resume <session_id>` — each chat (or issue) gets its own isolated Claude Code session.

## Setup

### Prerequisites

- Go 1.24+
- [`claude` CLI](https://github.com/anthropics/claude-code) installed and authenticated (`ANTHROPIC_API_KEY` set)
- **Telegram:** A bot token (create via [@BotFather](https://t.me/BotFather))
- **Linear:** A Linear API key, webhook signing secret, and `gh` CLI authenticated

### Install & run

```bash
git clone https://github.com/gsarma/gobot
cd gobot

cp .env.example .env
# edit .env — set PROVIDER and the required vars for that provider

go run ./cmd/gobot/
```

### Environment variables

#### Shared

| Variable | Required | Default | Description |
|---|---|---|---|
| `PROVIDER` | | `telegram` | Backend: `telegram` or `linear` |
| `CLAUDE_PATH` | | `claude` | Path to the `claude` binary |
| `CLAUDE_MODEL` | | `claude-opus-4-6` | Model to use |
| `CLAUDE_ALLOWED_TOOLS` | | `Bash,Read,Edit,Write,Glob,Grep` | Tools Claude may use |
| `CLAUDE_MAX_BUDGET_USD` | | `2.00` | Per-request budget cap |
| `EXEC_TIMEOUT` | | `5m` | Timeout per request |
| `WORK_DIR` | | `$PWD` | Fallback working directory for `claude` |

#### Telegram (`PROVIDER=telegram`)

| Variable | Required | Default | Description |
|---|---|---|---|
| `TELEGRAM_TOKEN` | ✅ | — | Telegram bot token |
| `ALLOWED_CHAT_IDS` | | (all) | Comma-separated int64 chat IDs to allow |

#### Linear (`PROVIDER=linear`)

| Variable | Required | Default | Description |
|---|---|---|---|
| `LINEAR_API_KEY` | ✅ | — | Linear API key (`lin_api_…`) |
| `LINEAR_WEBHOOK_SECRET` | ✅ | — | Webhook signing secret from Linear team settings |
| `LINEAR_WEBHOOK_PORT` | | `8080` | Port for the webhook HTTP server |
| `LINEAR_TRIGGER_STATE` | | `In Progress` | Issue state that triggers implementation |
| `LINEAR_DONE_STATE` | | `Done` | Issue state that triggers PR merge |
| `LINEAR_TEAM_REPOS` | | — | `KEY:/path,KEY2:/path2` — map team keys to repo paths |
| `LINEAR_DEFAULT_REPO` | | `WORK_DIR` | Fallback repo path when team key not found |

## Usage

### Telegram

1. Start gobot (`PROVIDER=telegram`)
2. Send a message to your bot: `list files in /tmp`
3. gobot shows a typing indicator, then streams the response back as Claude works
4. Send a follow-up — the session is automatically resumed

### Linear

1. Start gobot (`PROVIDER=linear`)
2. Expose the webhook port: `ngrok http 8080`
3. Set the webhook URL in Linear → Team Settings → API → Webhooks (subscribe to Issue events)
4. Move an issue to **In Progress** → Claude branches, implements, creates a PR, and posts the PR URL as a comment
5. Move the issue to **Done** → gobot squash-merges the PR and posts confirmation

## Deployment (Ubuntu)

```bash
# First-time setup on the server
sudo bash service/install.sh

# Edit secrets
sudo nano /etc/gobot/env

# Deploy updates from your dev machine
make deploy HOST=user@yourserver
```

See `service/` for the systemd unit file, env template, and install script.

## Testing

Unit tests:

```bash
go test ./...
```

Smoke test the Claude executor directly (no provider needed):

```bash
# default prompt
go run ./cmd/smoketest/

# custom prompt
go run ./cmd/smoketest/ "list files in /tmp and summarise"
```

## Project layout

```
cmd/gobot/main.go                      # entry point (selects provider via PROVIDER)
cmd/smoketest/main.go                  # CLI smoke test for the Claude executor
internal/
  config/config.go                     # env-var config (Telegram + Linear)
  provider/
    provider.go                        # Provider interface + message types
    telegram/telegram.go               # Telegram long-poll implementation
    linear/
      client.go                        # Linear GraphQL API client
      linear.go                        # Linear webhook HTTP server
  executor/
    executor.go                        # Executor interface
    claude/claude.go                   # Claude Code CLI implementation
  orchestrator/orchestrator.go         # ties provider + executor; streaming vs batch, merge
```
