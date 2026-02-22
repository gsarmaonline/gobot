package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_RequiresTelegramToken(t *testing.T) {
	os.Clearenv()
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when TELEGRAM_TOKEN is missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("TELEGRAM_TOKEN", "test-token")
	defer os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ClaudePath != "claude" {
		t.Errorf("ClaudePath = %q, want %q", cfg.ClaudePath, "claude")
	}
	if cfg.ClaudeModel != "claude-opus-4-6" {
		t.Errorf("ClaudeModel = %q, want %q", cfg.ClaudeModel, "claude-opus-4-6")
	}
	if cfg.ClaudeMaxBudgetUSD != 2.00 {
		t.Errorf("ClaudeMaxBudgetUSD = %v, want 2.00", cfg.ClaudeMaxBudgetUSD)
	}
	if cfg.ExecTimeout != 5*time.Minute {
		t.Errorf("ExecTimeout = %v, want 5m", cfg.ExecTimeout)
	}
	if len(cfg.AllowedChatIDs) != 0 {
		t.Errorf("AllowedChatIDs = %v, want empty", cfg.AllowedChatIDs)
	}
}

func TestLoad_AllowedChatIDs(t *testing.T) {
	os.Clearenv()
	os.Setenv("TELEGRAM_TOKEN", "test-token")
	os.Setenv("ALLOWED_CHAT_IDS", "123, 456, 789")
	defer os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int64{123, 456, 789}
	if len(cfg.AllowedChatIDs) != len(want) {
		t.Fatalf("AllowedChatIDs length = %d, want %d", len(cfg.AllowedChatIDs), len(want))
	}
	for i, id := range want {
		if cfg.AllowedChatIDs[i] != id {
			t.Errorf("AllowedChatIDs[%d] = %d, want %d", i, cfg.AllowedChatIDs[i], id)
		}
	}
}

func TestLoad_CustomExecTimeout(t *testing.T) {
	os.Clearenv()
	os.Setenv("TELEGRAM_TOKEN", "test-token")
	os.Setenv("EXEC_TIMEOUT", "10m")
	defer os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ExecTimeout != 10*time.Minute {
		t.Errorf("ExecTimeout = %v, want 10m", cfg.ExecTimeout)
	}
}

func TestLoad_InvalidBudget(t *testing.T) {
	os.Clearenv()
	os.Setenv("TELEGRAM_TOKEN", "test-token")
	os.Setenv("CLAUDE_MAX_BUDGET_USD", "not-a-number")
	defer os.Clearenv()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid CLAUDE_MAX_BUDGET_USD")
	}
}
