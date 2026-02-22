package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds Claude executor settings and the path to projects.json.
type Config struct {
	ProjectsFile       string
	SessionsFile       string
	ClaudePath         string
	ClaudeModel        string
	ClaudeAllowedTools string
	ClaudeMaxBudgetUSD float64
	ExecTimeout        time.Duration
}

func Load() (*Config, error) {
	// Load .env if present; ignore error if file doesn't exist.
	_ = godotenv.Load()

	cfg := &Config{
		ProjectsFile:       getEnvOrDefault("PROJECTS_FILE", "projects.json"),
		SessionsFile:       getEnvOrDefault("SESSIONS_FILE", "sessions.json"),
		ClaudePath:         getEnvOrDefault("CLAUDE_PATH", "claude"),
		ClaudeModel:        getEnvOrDefault("CLAUDE_MODEL", "claude-opus-4-6"),
		ClaudeAllowedTools: getEnvOrDefault("CLAUDE_ALLOWED_TOOLS", "Bash,Read,Edit,Write,Glob,Grep"),
		ClaudeMaxBudgetUSD: 2.00,
		ExecTimeout:        5 * time.Minute,
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

	return cfg, nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
