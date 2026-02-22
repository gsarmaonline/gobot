#!/usr/bin/env bash
# Installs gobot as a systemd service on Ubuntu.
# Run as root or with sudo: sudo bash service/install.sh
set -euo pipefail

SCRIPT_DIR="$(dirname "$0")"
BINARY_SRC="${1:-./gobot}"   # pass path to pre-built binary, or build first
SERVICE_FILE="$SCRIPT_DIR/gobot.service"
UPDATE_SERVICE_FILE="$SCRIPT_DIR/gobot-update.service"
UPDATE_TIMER_FILE="$SCRIPT_DIR/gobot-update.timer"
UPDATE_SCRIPT="$SCRIPT_DIR/update.sh"
ENV_EXAMPLE="$SCRIPT_DIR/env.example"
GOBOT_HOME="/var/lib/gobot"

# ── 1. Build binary if not provided ─────────────────────────────────────────
if [[ ! -f "$BINARY_SRC" ]]; then
    echo "Binary not found at $BINARY_SRC — building..."
    go build -o gobot ./cmd/gobot/
    BINARY_SRC="./gobot"
fi

# ── 2. Install binaries ───────────────────────────────────────────────────────
install -m 755 "$BINARY_SRC" /usr/local/bin/gobot
echo "Installed /usr/local/bin/gobot"

install -m 755 "$UPDATE_SCRIPT" /usr/local/bin/gobot-update
echo "Installed /usr/local/bin/gobot-update"

# ── 3. Create dedicated user ─────────────────────────────────────────────────
if ! id gobot &>/dev/null; then
    useradd --system --home-dir "$GOBOT_HOME" --no-create-home --shell /usr/sbin/nologin gobot
    echo "Created system user: gobot"
fi

# ── 4. Create workspace and config dir ───────────────────────────────────────
mkdir -p "$GOBOT_HOME/workspace"
chown -R gobot:gobot "$GOBOT_HOME"

mkdir -p /etc/gobot

if [[ ! -f /etc/gobot/env ]]; then
    cp "$ENV_EXAMPLE" /etc/gobot/env
    chmod 600 /etc/gobot/env
    echo ""
    echo "⚠  Created /etc/gobot/env from example — edit it before starting:"
    echo "   sudo nano /etc/gobot/env"
    echo ""
fi

# ── 5. Clone source repo for auto-update (if GOBOT_REPO_DIR not present) ────
REPO_DIR=$(grep -E '^GOBOT_REPO_DIR=' /etc/gobot/env | cut -d= -f2- || echo "$GOBOT_HOME/src")
if [[ ! -d "$REPO_DIR/.git" ]]; then
    echo ""
    echo "Source repo not found at $REPO_DIR."
    read -rp "Enter the git remote URL to clone (e.g. git@github.com:you/gobot.git): " REMOTE_URL
    git clone "$REMOTE_URL" "$REPO_DIR"
    chown -R gobot:gobot "$REPO_DIR"
    echo "Cloned repo to $REPO_DIR"
    echo ""
fi

# ── 6. Log claude in as the gobot user (skip if already authenticated) ───────
CLAUDE_DIR="${GOBOT_HOME}/.claude"
if sudo -u gobot test -d "$CLAUDE_DIR" && \
   [ -n "$(sudo -u gobot ls -A "$CLAUDE_DIR" 2>/dev/null)" ]; then
    echo "claude already authenticated for gobot user — skipping login"
else
    echo ""
    echo "Logging in to claude as the gobot user (follow the prompts)..."
    sudo -u gobot -H claude login
    echo ""
fi

# ── 7. Install and enable systemd units ──────────────────────────────────────
cp "$SERVICE_FILE"        /etc/systemd/system/gobot.service
cp "$UPDATE_SERVICE_FILE" /etc/systemd/system/gobot-update.service
cp "$UPDATE_TIMER_FILE"   /etc/systemd/system/gobot-update.timer
systemctl daemon-reload

systemctl enable gobot
systemctl enable --now gobot-update.timer
echo "Enabled gobot.service and gobot-update.timer (runs every 5 min)"

# ── 8. Start (or restart) gobot ──────────────────────────────────────────────
if systemctl is-active --quiet gobot; then
    systemctl restart gobot
    echo "Restarted gobot.service"
else
    systemctl start gobot
    echo "Started gobot.service"
fi

echo ""
systemctl status gobot --no-pager
echo ""
echo "Auto-update timer status:"
systemctl status gobot-update.timer --no-pager
