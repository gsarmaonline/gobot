# Gobot

A minimal Go-based agent orchestrator that listens for messages from one or more **providers** (Telegram, Linear) and delegates them to **Claude Code CLI** (`claude`) running locally. Claude Code handles the agentic loop — tool use, planning, execution — while gobot handles the plumbing: receiving events, invoking `claude`, and routing results back.

All provider configuration lives in a single `projects.json` file that is hot-reloaded every 5 seconds — no restart needed to add projects or update bindings.

## Architecture

```
projects.json (source of truth, hot-reloaded every 5s)
       │
  Registry.Load() → Registry.Watch() (goroutine)
       │
  main.go reads registry → starts all configured providers simultaneously
       │
  ┌─────────────────────────────────┐
  │  Telegram provider              │  ← admin /commands handled internally
  │  Linear provider (HTTP server)  │  ← team key → project lookup
  └──────────────┬──────────────────┘
                 │ fan-in (merged channel)
                 ▼
         Orchestrator (multi-provider)
                 │
         Claude executor (workDir from project binding)
```

### Telegram provider (streaming)

```
Telegram msg → project lookup (chatBindings) → Orchestrator
  → claude -p "<msg>" --output-format stream-json
  → stream text chunks back to Telegram every 2s
  → session_id stored for --resume on follow-up messages
```

### Linear provider (batch)

```
Linear "In Progress" webhook → verify HMAC-SHA256 → fetch issue
  → resolve project via teamBindings → Orchestrator
  → Claude: branch, implement, commit, push, gh pr create
  → extract PR URL → post as comment on Linear issue

Linear "Done" webhook → emit "merge" action
  → look up stored PR URL → gh pr merge --squash → post confirmation
```

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

# Create your projects.json from the example
cp projects.json.example projects.json
# Edit projects.json — configure the providers you want and add your projects

cp .env.example .env
# Edit .env — set ANTHROPIC_API_KEY and any Claude executor overrides

go run ./cmd/gobot/
```

### Configuration: `projects.json`

All provider and project configuration lives here. Sections are optional — only providers whose section exists will start.

```json
{
  "projects": {
    "backend": { "workDir": "/home/user/repos/backend" }
  },
  "telegram": {
    "token": "123456:ABCDEFG...",
    "adminChatIDs": [123456789],
    "chatBindings": { "-1001234567890": "backend" }
  },
  "linear": {
    "apiKey": "lin_api_...",
    "webhookSecret": "your_webhook_signing_secret",
    "webhookPort": 8080,
    "triggerState": "In Progress",
    "doneState": "Done",
    "teamBindings": { "ENG": "backend" }
  }
}
```

| Field | Description |
|---|---|
| `projects.<name>.workDir` | Absolute path to the git repo Claude will work in |
| `telegram.token` | Telegram bot token from @BotFather |
| `telegram.adminChatIDs` | Chat IDs that can issue `/commands` to manage the bot |
| `telegram.chatBindings` | Map of Telegram chat ID → project name |
| `linear.apiKey` | Linear API key (`lin_api_…`) |
| `linear.webhookSecret` | Webhook signing secret from Linear team settings |
| `linear.webhookPort` | Port for the webhook HTTP server (default: `8080`) |
| `linear.triggerState` | Issue state that triggers implementation (default: `"In Progress"`) |
| `linear.doneState` | Issue state that triggers PR merge (default: `"Done"`) |
| `linear.teamBindings` | Map of Linear team key → project name |

### Environment variables

Only Claude executor settings are configured via env:

| Variable | Default | Description |
|---|---|---|
| `PROJECTS_FILE` | `projects.json` | Path to projects.json |
| `CLAUDE_PATH` | `claude` | Path to the `claude` binary |
| `CLAUDE_MODEL` | `claude-opus-4-6` | Model to use |
| `CLAUDE_ALLOWED_TOOLS` | `Bash,Read,Edit,Write,Glob,Grep` | Tools Claude may use |
| `CLAUDE_MAX_BUDGET_USD` | `2.00` | Per-request budget cap |
| `EXEC_TIMEOUT` | `5m` | Timeout per request |

## Usage

### Telegram

1. Add a `telegram` section to `projects.json` with your bot token
2. Set yourself as an admin chat ID in `adminChatIDs`
3. Start gobot: `go run ./cmd/gobot/`
4. Use admin commands to set up project bindings:
   - `/addproject backend /path/to/repo` — register a project
   - `/setproject backend` — bind the current chat to that project
   - `/listprojects` — show all projects and bindings
   - `/addlinear ENG backend` — bind a Linear team to a project
5. Send a message to your bot — gobot streams Claude's response back in real time
6. Follow-up messages automatically resume the Claude session

### Linear

1. Add a `linear` section to `projects.json` with your API key and webhook secret
2. Add `teamBindings` to map your team key(s) to projects
3. Start gobot: `go run ./cmd/gobot/`
4. Expose the webhook port: `ngrok http 8080`
5. Set the webhook URL in Linear → Team Settings → API → Webhooks (subscribe to Issue events)
6. Move an issue to **In Progress** → Claude branches, implements, creates a PR, posts the PR URL as a comment
7. Move the issue to **Done** → gobot squash-merges the PR and posts confirmation

## Deployment (Ubuntu)

```bash
# First-time setup on the server
sudo bash service/install.sh

# Edit secrets and config
sudo nano /etc/gobot/env           # Claude settings + PROJECTS_FILE path
sudo nano /etc/gobot/projects.json # Provider config

# Deploy updates from your dev machine
make deploy HOST=user@yourserver
```

See `service/` for the systemd unit file, env template, and install script.

## Testing

Unit tests:

```bash
go test ./...
```

Smoke test the Claude executor directly (no provider or projects.json needed):

```bash
# default prompt
go run ./cmd/smoketest/

# custom prompt
go run ./cmd/smoketest/ "list files in /tmp and summarise"
```

## Project layout

```
cmd/gobot/main.go                      # entry point — loads registry, starts all providers
cmd/smoketest/main.go                  # CLI smoke test for the Claude executor
projects.json.example                  # annotated example projects.json
internal/
  config/config.go                     # Claude executor env-var config only
  registry/registry.go                 # projects.json loader, watcher, atomic Save
  provider/
    provider.go                        # Provider interface + message types
    telegram/telegram.go               # Telegram long-poll + admin commands
    linear/
      client.go                        # Linear GraphQL API client
      linear.go                        # Linear webhook HTTP server
  executor/
    executor.go                        # Executor interface
    claude/claude.go                   # Claude Code CLI implementation
  orchestrator/orchestrator.go         # multi-provider fan-in; streaming vs batch; merge
```
