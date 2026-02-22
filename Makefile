BINARY := gobot
BUILD_FLAGS := -ldflags="-s -w"

.PHONY: build run smoketest test deploy

build:
	go build $(BUILD_FLAGS) -o $(BINARY) ./cmd/gobot/

run:
	go run ./cmd/gobot/

smoketest:
	go run ./cmd/smoketest/

test:
	go test ./...

# Deploy to an Ubuntu server: make deploy HOST=user@host
deploy: build
	scp $(BINARY) $(HOST):/tmp/gobot
	ssh -t $(HOST) "sudo mv /tmp/gobot /usr/local/bin/gobot && sudo systemctl restart gobot && sudo systemctl status gobot --no-pager"
