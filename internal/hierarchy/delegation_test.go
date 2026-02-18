package hierarchy

import (
	"fmt"
	"testing"
)

func TestDelegationEngineFindSubordinate(t *testing.T) {
	engine := NewDelegationEngine(func(parentType, parentID string) ([]string, error) {
		if parentType == "user" && parentID == "user-1" {
			return []string{"agent-1", "agent-2"}, nil
		}
		return nil, fmt.Errorf("not found")
	})

	child, err := engine.FindSubordinate("user", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child != "agent-1" {
		t.Errorf("expected 'agent-1', got '%s'", child)
	}
}

func TestDelegationEngineNoSubordinates(t *testing.T) {
	engine := NewDelegationEngine(func(parentType, parentID string) ([]string, error) {
		return []string{}, nil
	})

	_, err := engine.FindSubordinate("user", "user-1")
	if err == nil {
		t.Error("expected error when no subordinates found")
	}
}

func TestDelegationEngineNilLookup(t *testing.T) {
	engine := NewDelegationEngine(nil)

	_, err := engine.FindSubordinate("user", "user-1")
	if err == nil {
		t.Error("expected error when no lookup function configured")
	}
}

func TestDelegationEngineLookupError(t *testing.T) {
	engine := NewDelegationEngine(func(parentType, parentID string) ([]string, error) {
		return nil, fmt.Errorf("database error")
	})

	_, err := engine.FindSubordinate("agent", "agent-1")
	if err == nil {
		t.Error("expected error on lookup failure")
	}
}

func TestCanDelegateTrue(t *testing.T) {
	engine := NewDelegationEngine(func(parentType, parentID string) ([]string, error) {
		return []string{"agent-sub-1"}, nil
	})

	can, err := engine.CanDelegate("user", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !can {
		t.Error("expected CanDelegate to return true")
	}
}

func TestCanDelegateFalseEmpty(t *testing.T) {
	engine := NewDelegationEngine(func(parentType, parentID string) ([]string, error) {
		return []string{}, nil
	})

	can, err := engine.CanDelegate("user", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if can {
		t.Error("expected CanDelegate to return false when no children")
	}
}

func TestCanDelegateNilLookup(t *testing.T) {
	engine := NewDelegationEngine(nil)

	can, err := engine.CanDelegate("user", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if can {
		t.Error("expected CanDelegate to return false when no lookup configured")
	}
}
