#!/usr/bin/env bash
# Pulls latest gobot source, rebuilds if changed, and restarts the service.
# Designed to be run by gobot-update.timer every few minutes.
set -euo pipefail

REPO_DIR="${GOBOT_REPO_DIR:?GOBOT_REPO_DIR must be set in /etc/gobot/env}"
BINARY="/usr/local/bin/gobot"

cd "$REPO_DIR"

# Fetch without merging so we can compare.
git fetch origin main --quiet

LOCAL=$(git rev-parse HEAD)
REMOTE=$(git rev-parse origin/main)

if [ "$LOCAL" = "$REMOTE" ]; then
    exit 0
fi

# Don't interrupt an active claude session — the timer will retry in 5min.
if pgrep -u gobot -x claude > /dev/null 2>&1; then
    echo "gobot-update: active claude session detected, deferring restart (will retry in 5min)"
    exit 0
fi

echo "gobot: new commits available ($LOCAL → $REMOTE), rebuilding..."
git pull origin main --quiet

# Build to a temp path first — if it fails, the running binary is untouched.
TMP=$(mktemp)
if ! go build -ldflags="-s -w" -o "$TMP" ./cmd/gobot/; then
    echo "gobot: build failed — keeping current binary"
    rm -f "$TMP"
    exit 1
fi

mv "$TMP" "$BINARY"
chmod 755 "$BINARY"
echo "gobot: updated to $(git rev-parse --short HEAD), restarting service"
systemctl restart gobot
