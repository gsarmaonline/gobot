// smoketest exercises the Claude executor directly — no Telegram needed.
// Usage:
//
//	go run ./cmd/smoketest/
//	go run ./cmd/smoketest/ "your custom prompt here"
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/gsarma/gobot/internal/config"
	claudeexec "github.com/gsarma/gobot/internal/executor/claude"
)

func main() {
	_ = godotenv.Load()

	// Allow running without TELEGRAM_TOKEN for this smoke test.
	if os.Getenv("TELEGRAM_TOKEN") == "" {
		os.Setenv("TELEGRAM_TOKEN", "smoketest")
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	prompt := "What is 2+2? Reply in one sentence."
	if len(os.Args) > 1 {
		prompt = os.Args[1]
	}

	fmt.Printf("Claude path : %s\n", cfg.ClaudePath)
	fmt.Printf("Model       : %s\n", cfg.ClaudeModel)
	fmt.Printf("Prompt      : %s\n\n", prompt)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ExecTimeout)
	defer cancel()

	exec := claudeexec.New(cfg)
	chunks, result, err := exec.Stream(ctx, prompt, cfg.WorkDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stream error: %v\n", err)
		os.Exit(1)
	}

	start := time.Now()

	for chunk := range chunks {
		switch chunk.Type {
		case "text":
			fmt.Print(chunk.Content)
		case "tool_use":
			fmt.Printf("\n[tool: %s]\n", chunk.Content)
		case "error":
			fmt.Fprintf(os.Stderr, "\n[error: %s]\n", chunk.Content)
		}
	}
	fmt.Println()

	res := <-result
	fmt.Printf("\n--- done in %s ---\n", time.Since(start).Round(time.Millisecond))
	if res.SessionID != "" {
		fmt.Printf("session_id: %s\n", res.SessionID)
	}
	if res.Err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", res.Err)
		os.Exit(1)
	}
}
