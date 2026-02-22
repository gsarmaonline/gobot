package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken      string
	AllowedChatIDs     []int64
	ClaudePath         string
	ClaudeModel        string
	ClaudeAllowedTools string
	ClaudeMaxBudgetUSD float64
	ExecTimeout        time.Duration
	WorkDir            string
}

func Load() (*Config, error) {
	// Load .env if present; ignore error if file doesn't exist
	_ = godotenv.Load()

	cfg := &Config{
		TelegramToken:      os.Getenv("TELEGRAM_TOKEN"),
		ClaudePath:         getEnvOrDefault("CLAUDE_PATH", "claude"),
		ClaudeModel:        getEnvOrDefault("CLAUDE_MODEL", "claude-opus-4-6"),
		ClaudeAllowedTools: getEnvOrDefault("CLAUDE_ALLOWED_TOOLS", "Bash,Read,Edit,Write,Glob,Grep"),
		ClaudeMaxBudgetUSD: 2.00,
		ExecTimeout:        5 * time.Minute,
	}

	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN is required")
	}

	if raw := os.Getenv("CLAUDE_MAX_BUDGET_USD"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid CLAUDE_MAX_BUDGET_USD: %w", err)
		}
		cfg.ClaudeMaxBudgetUSD = v
	}

	if raw := os.Getenv("EXEC_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid EXEC_TIMEOUT: %w", err)
		}
		cfg.ExecTimeout = d
	}

	if raw := os.Getenv("ALLOWED_CHAT_IDS"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid chat ID %q: %w", part, err)
			}
			cfg.AllowedChatIDs = append(cfg.AllowedChatIDs, id)
		}
	}

	workDir := os.Getenv("WORK_DIR")
	if workDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("could not determine working directory: %w", err)
		}
		workDir = wd
	}
	cfg.WorkDir = workDir

	return cfg, nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
