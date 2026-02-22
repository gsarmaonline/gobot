#!/usr/bin/env bash
# Installs gobot as a systemd service on Ubuntu.
# Run as root or with sudo: sudo bash service/install.sh
set -euo pipefail

BINARY_SRC="${1:-./gobot}"   # pass path to pre-built binary, or build first
SERVICE_FILE="$(dirname "$0")/gobot.service"
ENV_EXAMPLE="$(dirname "$0")/env.example"
GOBOT_HOME="/var/lib/gobot"

# ── 1. Build binary if not provided ─────────────────────────────────────────
if [[ ! -f "$BINARY_SRC" ]]; then
    echo "Binary not found at $BINARY_SRC — building..."
    go build -o gobot ./cmd/gobot/
    BINARY_SRC="./gobot"
fi

# ── 2. Install binary ────────────────────────────────────────────────────────
install -m 755 "$BINARY_SRC" /usr/local/bin/gobot
echo "Installed /usr/local/bin/gobot"

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

# ── 5. Log claude in as the gobot user (skip if already authenticated) ───────
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

# ── 6. Install and enable the systemd unit ───────────────────────────────────
cp "$SERVICE_FILE" /etc/systemd/system/gobot.service
systemctl daemon-reload
systemctl enable gobot
echo "Enabled gobot.service (starts on boot)"

# ── 7. Start (or restart if already running) ─────────────────────────────────
if systemctl is-active --quiet gobot; then
    systemctl restart gobot
    echo "Restarted gobot.service"
else
    systemctl start gobot
    echo "Started gobot.service"
fi

echo ""
systemctl status gobot --no-pager
