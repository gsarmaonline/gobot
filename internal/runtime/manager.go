package runtime

import (
	"fmt"
	"log"
	"sync"

	"github.com/gsarmaonline/gobot/internal/llm"
	"github.com/gsarmaonline/gobot/internal/memory"
	"github.com/gsarmaonline/gobot/internal/models"
)

// Manager manages the lifecycle of all agent workers.
type Manager struct {
	mu      sync.RWMutex
	workers map[string]*AgentWorker // agentID -> worker
	router  *llm.Router
	memory  memory.Store
	agents  *models.AgentStore
}

func NewManager(router *llm.Router, mem memory.Store, agents *models.AgentStore) *Manager {
	return &Manager{
		workers: make(map[string]*AgentWorker),
		router:  router,
		memory:  mem,
		agents:  agents,
	}
}

// StartAgent creates and starts a worker for the given agent.
func (m *Manager) StartAgent(agent *models.Agent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.workers[agent.ID]; exists {
		return fmt.Errorf("agent %s is already running", agent.ID)
	}

	ctx := NewAgentContext(agent, m.router, m.memory)
	worker := NewAgentWorker(ctx)
	m.workers[agent.ID] = worker

	// Start worker in a goroutine
	go worker.Run()

	// Update status in DB
	if m.agents != nil {
		if err := m.agents.UpdateStatus(agent.ID, "active"); err != nil {
			log.Printf("[manager] Failed to update agent %s status: %v", agent.ID, err)
		}
	}

	log.Printf("[manager] Started agent: %s (%s)", agent.Name, agent.ID)
	return nil
}

// StopAgent stops a running agent worker.
func (m *Manager) StopAgent(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	worker, exists := m.workers[agentID]
	if !exists {
		return fmt.Errorf("agent %s is not running", agentID)
	}

	worker.Stop()
	delete(m.workers, agentID)

	if m.agents != nil {
		if err := m.agents.UpdateStatus(agentID, "inactive"); err != nil {
			log.Printf("[manager] Failed to update agent %s status: %v", agentID, err)
		}
	}

	log.Printf("[manager] Stopped agent: %s", agentID)
	return nil
}

// PauseAgent pauses a running agent (stops processing but keeps in map).
func (m *Manager) PauseAgent(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	worker, exists := m.workers[agentID]
	if !exists {
		return fmt.Errorf("agent %s is not running", agentID)
	}

	worker.Stop()
	delete(m.workers, agentID)

	if m.agents != nil {
		if err := m.agents.UpdateStatus(agentID, "paused"); err != nil {
			log.Printf("[manager] Failed to update agent %s status: %v", agentID, err)
		}
	}

	return nil
}

// SendMessage routes a message to the appropriate agent's inbox.
func (m *Manager) SendMessage(agentID string, msg IncomingMessage) error {
	m.mu.RLock()
	worker, exists := m.workers[agentID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("agent %s is not running", agentID)
	}

	select {
	case worker.Inbox() <- msg:
		return nil
	default:
		return fmt.Errorf("agent %s inbox is full", agentID)
	}
}

// GetWorker returns the worker for an agent (if running).
func (m *Manager) GetWorker(agentID string) (*AgentWorker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.workers[agentID]
	return w, ok
}

// RunningAgents returns the IDs of all running agents.
func (m *Manager) RunningAgents() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.workers))
	for id := range m.workers {
		ids = append(ids, id)
	}
	return ids
}

// StopAll stops all running agents. Used for graceful shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, worker := range m.workers {
		worker.Stop()
		log.Printf("[manager] Stopped agent: %s", id)
	}
	m.workers = make(map[string]*AgentWorker)
}
