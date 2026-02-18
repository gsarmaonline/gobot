package runtime

import (
	"testing"

	"github.com/gsarmaonline/gobot/internal/llm"
	"github.com/gsarmaonline/gobot/internal/memory"
	"github.com/gsarmaonline/gobot/internal/models"
)

// mockLLMProvider for testing
type testProvider struct {
	response *llm.Response
	err      error
}

func (p *testProvider) Name() string      { return "test" }
func (p *testProvider) Available() bool    { return true }
func (p *testProvider) Chat(req *llm.Request) (*llm.Response, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.response, nil
}

func makeTestWorker(response string) *AgentWorker {
	router := llm.NewRouter()
	router.Register(&testProvider{
		response: &llm.Response{
			Content:      response,
			Model:        "test-model",
			InputTokens:  10,
			OutputTokens: 5,
		},
	})

	agent := &models.Agent{
		ID:               "agent-test-1",
		Name:             "TestBot",
		Title:            "Test Agent",
		LLMProvider:      "test",
		LLMModel:         "test-model",
		Temperature:      0.7,
		MaxAutonomyLevel: "suggest",
		ScopeDescription: "Handle test requests",
	}

	mem := memory.NewInMemoryStore()
	ctx := NewAgentContext(agent, router, mem)
	return NewAgentWorker(ctx)
}

func TestAgentWorkerProcessMessage(t *testing.T) {
	worker := makeTestWorker("Hello! I'm here to help.")

	msg := IncomingMessage{
		Channel:   "api",
		From:      "user@test.com",
		Content:   "Hi there",
		MessageID: "msg-1",
	}

	response, err := worker.ProcessMessage(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response.Content != "Hello! I'm here to help." {
		t.Errorf("expected 'Hello! I'm here to help.', got '%s'", response.Content)
	}

	if response.Channel != "api" {
		t.Errorf("expected channel 'api', got '%s'", response.Channel)
	}

	if response.To != "user@test.com" {
		t.Errorf("expected to 'user@test.com', got '%s'", response.To)
	}

	if response.InReplyTo != "msg-1" {
		t.Errorf("expected inReplyTo 'msg-1', got '%s'", response.InReplyTo)
	}
}

func TestAgentWorkerStoresHistory(t *testing.T) {
	worker := makeTestWorker("Response 1")

	worker.ProcessMessage(IncomingMessage{
		Channel: "api", From: "user", Content: "Message 1", MessageID: "1",
	})

	// Check that history was stored
	history, err := worker.ctx.Memory.GetHistory("agent-test-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 messages: user + assistant
	if len(history) != 2 {
		t.Errorf("expected 2 messages in history, got %d", len(history))
	}

	if history[0].Role != "user" {
		t.Errorf("expected first message role 'user', got '%s'", history[0].Role)
	}

	if history[1].Role != "assistant" {
		t.Errorf("expected second message role 'assistant', got '%s'", history[1].Role)
	}
}

func TestAgentWorkerTracksTokenUsage(t *testing.T) {
	worker := makeTestWorker("Response")

	worker.ProcessMessage(IncomingMessage{
		Channel: "api", From: "user", Content: "Hi", MessageID: "1",
	})
	worker.ProcessMessage(IncomingMessage{
		Channel: "api", From: "user", Content: "Hi again", MessageID: "2",
	})

	usage := worker.ctx.TokenUsage
	if usage.Requests != 2 {
		t.Errorf("expected 2 requests, got %d", usage.Requests)
	}
	if usage.InputTokens != 20 {
		t.Errorf("expected 20 input tokens, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 10 {
		t.Errorf("expected 10 output tokens, got %d", usage.OutputTokens)
	}
}

func TestAgentWorkerRunAndStop(t *testing.T) {
	worker := makeTestWorker("Auto response")

	// Start worker in goroutine
	done := make(chan struct{})
	go func() {
		worker.Run()
		close(done)
	}()

	// Send a message
	worker.Inbox() <- IncomingMessage{
		Channel: "api", From: "user", Content: "Hi", MessageID: "1",
	}

	// Wait for response
	response := <-worker.Outbox()
	if response.Content != "Auto response" {
		t.Errorf("expected 'Auto response', got '%s'", response.Content)
	}

	// Stop worker
	worker.Stop()
	<-done // Wait for Run() to return
}

func TestBuildSystemPrompt(t *testing.T) {
	tests := []struct {
		name     string
		agent    *models.Agent
		contains string
	}{
		{
			name:     "default prompt",
			agent:    &models.Agent{Name: "Sarah", Title: "Support Rep", MaxAutonomyLevel: "suggest"},
			contains: "Sarah",
		},
		{
			name:     "custom prompt",
			agent:    &models.Agent{SystemPrompt: "You are a sales bot", MaxAutonomyLevel: "suggest"},
			contains: "You are a sales bot",
		},
		{
			name:     "scope included",
			agent:    &models.Agent{Name: "Bot", ScopeDescription: "Handle customer emails", MaxAutonomyLevel: "suggest"},
			contains: "Handle customer emails",
		},
		{
			name:     "fully autonomous",
			agent:    &models.Agent{Name: "Bot", MaxAutonomyLevel: "fully_autonomous"},
			contains: "autonomously",
		},
		{
			name:     "act with approval",
			agent:    &models.Agent{Name: "Bot", MaxAutonomyLevel: "act_with_approval"},
			contains: "approval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &AgentContext{Agent: tt.agent}
			prompt := ctx.BuildSystemPrompt()
			if !containsStr(prompt, tt.contains) {
				t.Errorf("expected prompt to contain '%s', got '%s'", tt.contains, prompt)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
