BINARY := gobot
BUILD_FLAGS := -ldflags="-s -w"

.PHONY: build run smoketest test init watch deploy deploy-config

build:
	go build $(BUILD_FLAGS) -o $(BINARY) ./cmd/gobot/

run:
	go run ./cmd/gobot/

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
