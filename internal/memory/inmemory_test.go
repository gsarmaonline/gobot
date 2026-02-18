package memory

import (
	"testing"

	"github.com/gsarmaonline/gobot/internal/llm"
)

func TestInMemoryStoreMessages(t *testing.T) {
	store := NewInMemoryStore()

	// Add messages
	store.AddMessage("agent-1", llm.Message{Role: "user", Content: "Hello"})
	store.AddMessage("agent-1", llm.Message{Role: "assistant", Content: "Hi there!"})
	store.AddMessage("agent-1", llm.Message{Role: "user", Content: "How are you?"})

	// Get all history
	history, err := store.GetHistory("agent-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 3 {
		t.Errorf("expected 3 messages, got %d", len(history))
	}

	// Get limited history
	history, err = store.GetHistory("agent-1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("expected 2 messages with limit, got %d", len(history))
	}
	if history[0].Content != "Hi there!" {
		t.Errorf("expected 'Hi there!' as first limited message, got '%s'", history[0].Content)
	}
}

func TestInMemoryStoreEmptyHistory(t *testing.T) {
	store := NewInMemoryStore()

	history, err := store.GetHistory("nonexistent", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d messages", len(history))
	}
}

func TestInMemoryStoreClearHistory(t *testing.T) {
	store := NewInMemoryStore()
	store.AddMessage("agent-1", llm.Message{Role: "user", Content: "Hello"})

	err := store.ClearHistory("agent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	history, _ := store.GetHistory("agent-1", 0)
	if len(history) != 0 {
		t.Errorf("expected empty history after clear, got %d messages", len(history))
	}
}

func TestInMemoryStoreFacts(t *testing.T) {
	store := NewInMemoryStore()

	// Set facts
	store.SetFact("agent-1", "name", "Alice")
	store.SetFact("agent-1", "role", "Support Rep")

	// Get specific fact
	val, err := store.GetFact("agent-1", "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "Alice" {
		t.Errorf("expected 'Alice', got '%s'", val)
	}

	// Get all facts
	facts, err := store.GetAllFacts("agent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(facts) != 2 {
		t.Errorf("expected 2 facts, got %d", len(facts))
	}
}

func TestInMemoryStoreFactNotFound(t *testing.T) {
	store := NewInMemoryStore()

	_, err := store.GetFact("agent-1", "nonexistent")
	if err == nil {
		t.Error("expected error for missing fact")
	}

	_, err = store.GetFact("nonexistent-agent", "key")
	if err == nil {
		t.Error("expected error for missing agent")
	}
}

func TestInMemoryStoreIsolation(t *testing.T) {
	store := NewInMemoryStore()

	store.AddMessage("agent-1", llm.Message{Role: "user", Content: "For agent 1"})
	store.AddMessage("agent-2", llm.Message{Role: "user", Content: "For agent 2"})
	store.SetFact("agent-1", "key", "value1")
	store.SetFact("agent-2", "key", "value2")

	// Messages are isolated
	h1, _ := store.GetHistory("agent-1", 0)
	h2, _ := store.GetHistory("agent-2", 0)
	if len(h1) != 1 || len(h2) != 1 {
		t.Error("messages should be isolated per agent")
	}

	// Facts are isolated
	f1, _ := store.GetFact("agent-1", "key")
	f2, _ := store.GetFact("agent-2", "key")
	if f1 == f2 {
		t.Error("facts should be isolated per agent")
	}
}

func TestInMemoryStoreEmptyFacts(t *testing.T) {
	store := NewInMemoryStore()

	facts, err := store.GetAllFacts("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("expected empty facts, got %d", len(facts))
	}
}
