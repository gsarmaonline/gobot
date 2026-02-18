package runtime

import (
	"testing"
	"time"

	"github.com/gsarmaonline/gobot/internal/llm"
	"github.com/gsarmaonline/gobot/internal/memory"
	"github.com/gsarmaonline/gobot/internal/models"
)

func makeTestManager() *Manager {
	router := llm.NewRouter()
	router.Register(&testProvider{
		response: &llm.Response{
			Content:      "Manager test response",
			Model:        "test-model",
			InputTokens:  10,
			OutputTokens: 5,
		},
	})

	mem := memory.NewInMemoryStore()
	// Pass nil for AgentStore since we don't have a DB in tests
	return NewManager(router, mem, nil)
}

func TestManagerStartAndStop(t *testing.T) {
	mgr := makeTestManager()
	// Override agents store to avoid nil panic on status update
	mgr.agents = nil

	agent := &models.Agent{
		ID:               "agent-1",
		Name:             "TestBot",
		LLMProvider:      "test",
		LLMModel:         "test-model",
		Temperature:      0.7,
		MaxAutonomyLevel: "suggest",
	}

	err := mgr.StartAgent(agent)
	if err != nil {
		t.Fatalf("unexpected error starting agent: %v", err)
	}

	// Should be running
	running := mgr.RunningAgents()
	if len(running) != 1 {
		t.Errorf("expected 1 running agent, got %d", len(running))
	}

	// Start same agent again should fail
	err = mgr.StartAgent(agent)
	if err == nil {
		t.Error("expected error starting already running agent")
	}

	// Stop
	err = mgr.StopAgent("agent-1")
	if err != nil {
		t.Fatalf("unexpected error stopping agent: %v", err)
	}

	running = mgr.RunningAgents()
	if len(running) != 0 {
		t.Errorf("expected 0 running agents, got %d", len(running))
	}
}

func TestManagerSendMessage(t *testing.T) {
	mgr := makeTestManager()
	mgr.agents = nil

	agent := &models.Agent{
		ID:               "agent-msg",
		Name:             "MsgBot",
		LLMProvider:      "test",
		LLMModel:         "test-model",
		Temperature:      0.7,
		MaxAutonomyLevel: "suggest",
	}

	mgr.StartAgent(agent)
	defer mgr.StopAll()

	// Send message
	err := mgr.SendMessage("agent-msg", IncomingMessage{
		Channel: "api", From: "user", Content: "Hello", MessageID: "1",
	})
	if err != nil {
		t.Fatalf("unexpected error sending message: %v", err)
	}

	// Get worker and read response
	worker, ok := mgr.GetWorker("agent-msg")
	if !ok {
		t.Fatal("expected to find worker")
	}

	select {
	case resp := <-worker.Outbox():
		if resp.Content != "Manager test response" {
			t.Errorf("expected 'Manager test response', got '%s'", resp.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for response")
	}
}

func TestManagerSendMessageToStopped(t *testing.T) {
	mgr := makeTestManager()

	err := mgr.SendMessage("nonexistent", IncomingMessage{
		Channel: "api", From: "user", Content: "Hi",
	})
	if err == nil {
		t.Error("expected error sending to non-running agent")
	}
}

func TestManagerStopAll(t *testing.T) {
	mgr := makeTestManager()
	mgr.agents = nil

	for i := 0; i < 3; i++ {
		mgr.StartAgent(&models.Agent{
			ID:               "agent-" + string(rune('a'+i)),
			Name:             "Bot",
			LLMProvider:      "test",
			LLMModel:         "test-model",
			Temperature:      0.7,
			MaxAutonomyLevel: "suggest",
		})
	}

	if len(mgr.RunningAgents()) != 3 {
		t.Errorf("expected 3 running agents, got %d", len(mgr.RunningAgents()))
	}

	mgr.StopAll()

	if len(mgr.RunningAgents()) != 0 {
		t.Errorf("expected 0 running agents after StopAll, got %d", len(mgr.RunningAgents()))
	}
}

func TestManagerStopNonexistent(t *testing.T) {
	mgr := makeTestManager()

	err := mgr.StopAgent("nonexistent")
	if err == nil {
		t.Error("expected error stopping non-running agent")
	}
}
