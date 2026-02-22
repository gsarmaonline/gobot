#!/usr/bin/env bash
# Local dev self-healing loop:
#   - Starts gobot via `go run`
#   - Polls for new commits every 30s; pulls and restarts if found
#   - Restarts automatically if gobot exits (crash or error)
# Usage: make watch   (or bash service/watch.sh directly)
set -euo pipefail

INTERVAL="${WATCH_INTERVAL:-30}"
GOBOT_PID=""

cleanup() {
    echo ""
    echo "[watch] stopping..."
    if [[ -n "$GOBOT_PID" ]] && kill -0 "$GOBOT_PID" 2>/dev/null; then
        kill "$GOBOT_PID"
    fi
    exit 0
}
trap cleanup SIGINT SIGTERM

start_gobot() {
    go run ./cmd/gobot/ &
    GOBOT_PID=$!
    echo "[watch] started gobot (pid $GOBOT_PID)"
}

pull_if_changed() {
    git fetch origin main --quiet 2>/dev/null || return 0
    local local_sha remote_sha
    local_sha=$(git rev-parse HEAD)
    remote_sha=$(git rev-parse origin/main)
    if [[ "$local_sha" != "$remote_sha" ]]; then
        # Don't interrupt an active claude session — check again next interval.
        if pgrep -P "$GOBOT_PID" -x claude > /dev/null 2>&1; then
            echo "[watch] new commits available but claude session active, deferring..."
            return 0
        fi
        echo "[watch] new commits available — pulling..."
        git pull origin main --quiet
        return 1  # signal: restart needed
    fi
    return 0
}

echo "[watch] starting (git poll every ${INTERVAL}s). Ctrl-C to stop."
start_gobot

while true; do
    sleep "$INTERVAL"

    # Check for new commits; restart if pulled.
    if ! pull_if_changed; then
        echo "[watch] restarting after pull..."
        kill "$GOBOT_PID" 2>/dev/null || true
        wait "$GOBOT_PID" 2>/dev/null || true
        start_gobot
        continue
    fi

    # Restart if gobot has crashed.
    if ! kill -0 "$GOBOT_PID" 2>/dev/null; then
        echo "[watch] gobot exited, restarting in 5s..."
        sleep 5
        start_gobot
    fi
done
