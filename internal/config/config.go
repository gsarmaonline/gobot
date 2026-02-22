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
	// Provider selection: "telegram" (default) or "linear"
	Provider string

	// Telegram settings
	TelegramToken  string
	AllowedChatIDs []int64

	// Linear settings
	LinearAPIKey        string
	LinearWebhookSecret string
	LinearWebhookPort   int
	LinearTriggerState  string
	LinearDoneState     string
	LinearTeamRepos     map[string]string // teamKey → absolute repo path
	LinearDefaultRepo   string

	// Claude executor settings
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
		Provider:           getEnvOrDefault("PROVIDER", "telegram"),
		TelegramToken:      os.Getenv("TELEGRAM_TOKEN"),
		LinearAPIKey:       os.Getenv("LINEAR_API_KEY"),
		LinearWebhookSecret: os.Getenv("LINEAR_WEBHOOK_SECRET"),
		LinearWebhookPort:  8080,
		LinearTriggerState: getEnvOrDefault("LINEAR_TRIGGER_STATE", "In Progress"),
		LinearDoneState:    getEnvOrDefault("LINEAR_DONE_STATE", "Done"),
		LinearDefaultRepo:  os.Getenv("LINEAR_DEFAULT_REPO"),
		ClaudePath:         getEnvOrDefault("CLAUDE_PATH", "claude"),
		ClaudeModel:        getEnvOrDefault("CLAUDE_MODEL", "claude-opus-4-6"),
		ClaudeAllowedTools: getEnvOrDefault("CLAUDE_ALLOWED_TOOLS", "Bash,Read,Edit,Write,Glob,Grep"),
		ClaudeMaxBudgetUSD: 2.00,
		ExecTimeout:        5 * time.Minute,
	}

	switch cfg.Provider {
	case "telegram":
		if cfg.TelegramToken == "" {
			return nil, fmt.Errorf("TELEGRAM_TOKEN is required when PROVIDER=telegram")
		}
	case "linear":
		if cfg.LinearAPIKey == "" {
			return nil, fmt.Errorf("LINEAR_API_KEY is required when PROVIDER=linear")
		}
		if cfg.LinearWebhookSecret == "" {
			return nil, fmt.Errorf("LINEAR_WEBHOOK_SECRET is required when PROVIDER=linear")
		}
	default:
		return nil, fmt.Errorf("unknown PROVIDER %q: must be telegram or linear", cfg.Provider)
	}

	if raw := os.Getenv("LINEAR_WEBHOOK_PORT"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid LINEAR_WEBHOOK_PORT: %w", err)
		}
		cfg.LinearWebhookPort = v
	}

	if raw := os.Getenv("LINEAR_TEAM_REPOS"); raw != "" {
		cfg.LinearTeamRepos = make(map[string]string)
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			idx := strings.Index(part, ":")
			if idx < 0 {
				return nil, fmt.Errorf("invalid LINEAR_TEAM_REPOS entry %q: expected KEY:/path", part)
			}
			key := strings.TrimSpace(part[:idx])
			path := strings.TrimSpace(part[idx+1:])
			cfg.LinearTeamRepos[key] = path
		}
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
