package memory

import (
	"github.com/gsarmaonline/gobot/internal/llm"
)

// Store is the abstraction layer for agent memory.
// Implementations can be in-memory, PostgreSQL-backed, or vector DB-backed.
type Store interface {
	// AddMessage appends a message to the agent's conversation history.
	AddMessage(agentID string, msg llm.Message) error

	// GetHistory returns the recent conversation history for an agent.
	// limit controls how many messages to return (0 = all).
	GetHistory(agentID string, limit int) ([]llm.Message, error)

	// ClearHistory removes all conversation history for an agent.
	ClearHistory(agentID string) error

	// SetFact stores a key-value fact in the agent's long-term memory.
	SetFact(agentID, key, value string) error

	// GetFact retrieves a fact from the agent's long-term memory.
	GetFact(agentID, key string) (string, error)

	// GetAllFacts returns all facts for an agent.
	GetAllFacts(agentID string) (map[string]string, error)
}
