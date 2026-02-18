package hierarchy

import (
	"fmt"
	"testing"
)

func TestShouldEscalateChannelBlocked(t *testing.T) {
	perms := &Permissions{AllowedChannels: []string{"email"}}

	result := ShouldEscalate(perms, "fully_autonomous", "phone", "call", "hello", 0)
	if result == nil {
		t.Fatal("expected escalation for blocked channel")
	}
	if result.Reason != EscalationOutOfScope {
		t.Errorf("expected out_of_scope reason, got %s", result.Reason)
	}
}

func TestShouldEscalateActionBlocked(t *testing.T) {
	perms := &Permissions{AllowedActions: []string{"read_email"}}

	result := ShouldEscalate(perms, "fully_autonomous", "email", "delete_all", "hello", 0)
	if result == nil {
		t.Fatal("expected escalation for blocked action")
	}
	if result.Reason != EscalationOutOfScope {
		t.Errorf("expected out_of_scope, got %s", result.Reason)
	}
}

func TestShouldEscalateRestrictedTopic(t *testing.T) {
	perms := &Permissions{RestrictedTopics: []string{"salary"}}

	result := ShouldEscalate(perms, "fully_autonomous", "email", "reply", "What is my salary?", 0)
	if result == nil {
		t.Fatal("expected escalation for restricted topic")
	}
	if result.Reason != EscalationRestrictedTopic {
		t.Errorf("expected restricted_topic, got %s", result.Reason)
	}
}

func TestShouldEscalateBudget(t *testing.T) {
	perms := &Permissions{MaxSpendCents: 100}

	result := ShouldEscalate(perms, "fully_autonomous", "email", "send", "hello", 200)
	if result == nil {
		t.Fatal("expected escalation for budget exceeded")
	}
	if result.Reason != EscalationBudgetExceeded {
		t.Errorf("expected budget_exceeded, got %s", result.Reason)
	}
}

func TestShouldEscalateSuggestMode(t *testing.T) {
	perms := &Permissions{}

	result := ShouldEscalate(perms, "suggest", "email", "send", "hello", 0)
	if result == nil {
		t.Fatal("expected escalation for suggest mode")
	}
	if result.Reason != EscalationNeedsApproval {
		t.Errorf("expected needs_approval, got %s", result.Reason)
	}
}

func TestShouldNotEscalate(t *testing.T) {
	perms := &Permissions{
		AllowedChannels: []string{"email"},
		AllowedActions:  []string{"send_email"},
	}

	result := ShouldEscalate(perms, "fully_autonomous", "email", "send_email", "Hello customer!", 0)
	if result != nil {
		t.Errorf("expected no escalation, got reason: %s", result.Reason)
	}
}

func TestEscalationEngineDetermineTarget(t *testing.T) {
	engine := NewEscalationEngine(func(agentID string) (*EscalationTarget, error) {
		if agentID == "agent-1" {
			return &EscalationTarget{Type: "user", ID: "user-1", Name: "Manager"}, nil
		}
		return nil, fmt.Errorf("no parent found")
	})

	target, err := engine.DetermineTarget("agent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Type != "user" {
		t.Errorf("expected type 'user', got '%s'", target.Type)
	}
	if target.ID != "user-1" {
		t.Errorf("expected ID 'user-1', got '%s'", target.ID)
	}
}

func TestEscalationEngineNoParent(t *testing.T) {
	engine := NewEscalationEngine(func(agentID string) (*EscalationTarget, error) {
		return nil, fmt.Errorf("no parent")
	})

	_, err := engine.DetermineTarget("orphan")
	if err == nil {
		t.Error("expected error for agent with no parent")
	}
}
