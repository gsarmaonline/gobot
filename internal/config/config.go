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

	// CI watching
	CICheckInterval time.Duration
	CIStuckTimeout  time.Duration
	CIMaxRetries    int

	// MCP server binary for gobot tools (Gmail, Twilio)
	GobotMCPPath string
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
		CICheckInterval:    60 * time.Second,
		CIStuckTimeout:     30 * time.Minute,
		CIMaxRetries:       3,
		GobotMCPPath:       getEnvOrDefault("GOBOT_MCP_PATH", "gobot-mcp"),
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

	if raw := os.Getenv("CI_CHECK_INTERVAL"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid CI_CHECK_INTERVAL: %w", err)
		}
		cfg.CICheckInterval = d
	}

	if raw := os.Getenv("CI_STUCK_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid CI_STUCK_TIMEOUT: %w", err)
		}
		cfg.CIStuckTimeout = d
	}

	if raw := os.Getenv("CI_MAX_RETRIES"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid CI_MAX_RETRIES: %w", err)
		}
		cfg.CIMaxRetries = v
	}

	return cfg, nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
