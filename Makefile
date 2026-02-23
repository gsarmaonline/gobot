BINARY := gobot
BUILD_FLAGS := -ldflags="-s -w"
PROJECTS_FILE ?= projects.json

.PHONY: build build-mcp build-setup run dev smoketest test init watch deploy deploy-config setup-google

build:
	go build $(BUILD_FLAGS) -o $(BINARY) ./cmd/gobot/

build-mcp:
	go build $(BUILD_FLAGS) -o gobot-mcp ./cmd/gobot-mcp/

build-setup:
	go build $(BUILD_FLAGS) -o gobot-setup ./cmd/gobot-setup/

setup-google: build-setup
	./gobot-setup --projects-file $(PROJECTS_FILE)

run:
	go run ./cmd/gobot/

# Hot-reload with Air: rebuilds and restarts on any .go file change
dev:
	air

smoketest:
	go run ./cmd/smoketest/

test:
	go test ./...

# Self-healing local dev loop: git pull every 30s, restart on exit or new commits
watch:
	@bash service/watch.sh

# First-time setup: copy example files (skips if already present)
init:
	@[ -f projects.json ] && echo "projects.json already exists, skipping" || (cp projects.json.example projects.json && echo "Created projects.json")
	@[ -f .env ] && echo ".env already exists, skipping" || (cp .env.example .env && echo "Created .env")

# Deploy binary to an Ubuntu server: make deploy HOST=user@host
deploy: build
	scp $(BINARY) $(HOST):/tmp/gobot
	ssh -t $(HOST) "sudo mv /tmp/gobot /usr/local/bin/gobot && sudo systemctl restart gobot && sudo systemctl status gobot --no-pager"

# Push projects.json to server without restarting: make deploy-config HOST=user@host
deploy-config:
	scp projects.json $(HOST):/tmp/projects.json
	ssh -t $(HOST) "sudo mv /tmp/projects.json /etc/gobot/projects.json"
