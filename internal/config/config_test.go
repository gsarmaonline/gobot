package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()
	defer os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ProjectsFile != "projects.json" {
		t.Errorf("ProjectsFile = %q, want %q", cfg.ProjectsFile, "projects.json")
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
}

func TestLoad_CustomProjectsFile(t *testing.T) {
	os.Clearenv()
	os.Setenv("PROJECTS_FILE", "/etc/gobot/projects.json")
	defer os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ProjectsFile != "/etc/gobot/projects.json" {
		t.Errorf("ProjectsFile = %q, want %q", cfg.ProjectsFile, "/etc/gobot/projects.json")
	}
}

func TestLoad_CustomExecTimeout(t *testing.T) {
	os.Clearenv()
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
	os.Setenv("CLAUDE_MAX_BUDGET_USD", "not-a-number")
	defer os.Clearenv()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid CLAUDE_MAX_BUDGET_USD")
	}
}
