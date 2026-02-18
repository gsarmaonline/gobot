package memory

import (
	"fmt"
	"sync"

	"github.com/gsarmaonline/gobot/internal/llm"
)

// InMemoryStore is a simple in-memory implementation of Store.
// Useful for development and testing.
type InMemoryStore struct {
	mu       sync.RWMutex
	messages map[string][]llm.Message
	facts    map[string]map[string]string
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		messages: make(map[string][]llm.Message),
		facts:    make(map[string]map[string]string),
	}
}

func (s *InMemoryStore) AddMessage(agentID string, msg llm.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[agentID] = append(s.messages[agentID], msg)
	return nil
}

func (s *InMemoryStore) GetHistory(agentID string, limit int) ([]llm.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs := s.messages[agentID]
	if msgs == nil {
		return []llm.Message{}, nil
	}

	if limit > 0 && len(msgs) > limit {
		return msgs[len(msgs)-limit:], nil
	}
	// Return a copy to avoid data races
	result := make([]llm.Message, len(msgs))
	copy(result, msgs)
	return result, nil
}

func (s *InMemoryStore) ClearHistory(agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.messages, agentID)
	return nil
}

func (s *InMemoryStore) SetFact(agentID, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.facts[agentID] == nil {
		s.facts[agentID] = make(map[string]string)
	}
	s.facts[agentID][key] = value
	return nil
}

func (s *InMemoryStore) GetFact(agentID, key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	facts := s.facts[agentID]
	if facts == nil {
		return "", fmt.Errorf("no facts found for agent %s", agentID)
	}
	val, ok := facts[key]
	if !ok {
		return "", fmt.Errorf("fact %q not found for agent %s", key, agentID)
	}
	return val, nil
}

func (s *InMemoryStore) GetAllFacts(agentID string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	facts := s.facts[agentID]
	if facts == nil {
		return map[string]string{}, nil
	}
	// Return a copy
	result := make(map[string]string, len(facts))
	for k, v := range facts {
		result[k] = v
	}
	return result, nil
}
