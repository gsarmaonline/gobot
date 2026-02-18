package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	// Server
	Port string
	Env  string // "development" | "production"

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// LLM Providers (API keys)
	ClaudeAPIKey string
	OpenAIAPIKey string
	GeminiAPIKey string

	// Identity Providers
	GoogleWorkspaceCredentials string
	TwilioAccountSID           string
	TwilioAuthToken            string

	// Agent defaults
	DefaultLLMProvider string
	DefaultLLMModel    string
	DefaultTemperature float64
}

func Load() *Config {
	return &Config{
		Port: getEnv("PORT", "8080"),
		Env:  getEnv("ENV", "development"),

		DatabaseURL: getEnv("DATABASE_URL", "postgres://gobot:gobot_dev@localhost:5432/gobot?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),

		ClaudeAPIKey: getEnv("CLAUDE_API_KEY", ""),
		OpenAIAPIKey: getEnv("OPENAI_API_KEY", ""),
		GeminiAPIKey: getEnv("GEMINI_API_KEY", ""),

		GoogleWorkspaceCredentials: getEnv("GOOGLE_WORKSPACE_CREDENTIALS", ""),
		TwilioAccountSID:           getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:            getEnv("TWILIO_AUTH_TOKEN", ""),

		DefaultLLMProvider: getEnv("DEFAULT_LLM_PROVIDER", "claude"),
		DefaultLLMModel:    getEnv("DEFAULT_LLM_MODEL", "claude-sonnet-4-5-20250929"),
		DefaultTemperature: getEnvFloat("DEFAULT_TEMPERATURE", 0.7),
	}
}

func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if val := os.Getenv(key); val != "" {
		f, err := strconv.ParseFloat(val, 64)
		if err == nil {
			return f
		}
	}
	return fallback
}
