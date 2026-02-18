package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("expected default port '8080', got '%s'", cfg.Port)
	}

	if cfg.Env != "development" {
		t.Errorf("expected default env 'development', got '%s'", cfg.Env)
	}

	if cfg.DefaultLLMProvider != "claude" {
		t.Errorf("expected default LLM provider 'claude', got '%s'", cfg.DefaultLLMProvider)
	}

	if cfg.DefaultTemperature != 0.7 {
		t.Errorf("expected default temperature 0.7, got %f", cfg.DefaultTemperature)
	}

	if cfg.DatabaseURL == "" {
		t.Error("expected non-empty default database URL")
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("ENV", "production")
	os.Setenv("DEFAULT_TEMPERATURE", "0.5")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("ENV")
		os.Unsetenv("DEFAULT_TEMPERATURE")
	}()

	cfg := Load()

	if cfg.Port != "9090" {
		t.Errorf("expected port '9090', got '%s'", cfg.Port)
	}

	if cfg.Env != "production" {
		t.Errorf("expected env 'production', got '%s'", cfg.Env)
	}

	if cfg.DefaultTemperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", cfg.DefaultTemperature)
	}
}

func TestValidate(t *testing.T) {
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error with default config, got: %v", err)
	}

	cfg.DatabaseURL = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected error when DatabaseURL is empty")
	}
}

func TestGetEnvFloat(t *testing.T) {
	// Invalid float should return fallback
	os.Setenv("TEST_FLOAT", "not-a-number")
	defer os.Unsetenv("TEST_FLOAT")

	val := getEnvFloat("TEST_FLOAT", 1.5)
	if val != 1.5 {
		t.Errorf("expected fallback 1.5, got %f", val)
	}
}
