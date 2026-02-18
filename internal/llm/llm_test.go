package llm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// mockHTTPClient returns a canned response for testing LLM providers
type mockHTTPClient struct {
	statusCode int
	body       string
	err        error
	lastReq    *http.Request
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(m.body)),
	}, nil
}

// --- Token Usage ---

func TestTokenUsageAdd(t *testing.T) {
	usage := &TokenUsage{}
	usage.Add(&Response{InputTokens: 10, OutputTokens: 5})
	usage.Add(&Response{InputTokens: 20, OutputTokens: 15})

	if usage.InputTokens != 30 {
		t.Errorf("expected 30 input tokens, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 20 {
		t.Errorf("expected 20 output tokens, got %d", usage.OutputTokens)
	}
	if usage.Requests != 2 {
		t.Errorf("expected 2 requests, got %d", usage.Requests)
	}
}

// --- Router ---

func TestRouterRegisterAndChat(t *testing.T) {
	router := NewRouter()

	mock := &mockHTTPClient{
		statusCode: 200,
		body: `{"content":[{"text":"Hello!"}],"model":"claude-sonnet-4-5-20250929",
			"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`,
	}

	provider := NewClaudeProvider("test-key").WithHTTPClient(mock)
	router.Register(provider)

	resp, err := router.Chat("claude", &Request{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello!" {
		t.Errorf("expected 'Hello!', got '%s'", resp.Content)
	}
}

func TestRouterUnknownProvider(t *testing.T) {
	router := NewRouter()
	_, err := router.Chat("nonexistent", &Request{})
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestRouterUnavailableProvider(t *testing.T) {
	router := NewRouter()
	provider := NewClaudeProvider("") // no API key
	router.Register(provider)

	_, err := router.Chat("claude", &Request{})
	if err == nil {
		t.Error("expected error for unavailable provider")
	}
}

func TestRouterAvailableProviders(t *testing.T) {
	router := NewRouter()
	router.Register(NewClaudeProvider("key1"))
	router.Register(NewOpenAIProvider(""))       // unavailable
	router.Register(NewGeminiProvider("key3"))

	avail := router.AvailableProviders()
	if len(avail) != 2 {
		t.Errorf("expected 2 available providers, got %d", len(avail))
	}
}

func TestRouterGetProvider(t *testing.T) {
	router := NewRouter()
	router.Register(NewClaudeProvider("key"))

	p, ok := router.GetProvider("claude")
	if !ok || p == nil {
		t.Error("expected to find claude provider")
	}

	_, ok = router.GetProvider("openai")
	if ok {
		t.Error("expected not to find openai provider")
	}
}

// --- Claude Provider ---

func TestClaudeProviderAvailable(t *testing.T) {
	p := NewClaudeProvider("key")
	if !p.Available() {
		t.Error("expected available with key")
	}
	p2 := NewClaudeProvider("")
	if p2.Available() {
		t.Error("expected unavailable without key")
	}
}

func TestClaudeProviderName(t *testing.T) {
	p := NewClaudeProvider("key")
	if p.Name() != "claude" {
		t.Errorf("expected 'claude', got '%s'", p.Name())
	}
}

func TestClaudeProviderChat(t *testing.T) {
	mock := &mockHTTPClient{
		statusCode: 200,
		body: `{
			"content": [{"text": "I'm Claude, here to help!"}],
			"model": "claude-sonnet-4-5-20250929",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 15, "output_tokens": 8}
		}`,
	}

	p := NewClaudeProvider("test-key").WithHTTPClient(mock)
	resp, err := p.Chat(&Request{
		Messages:    []Message{{Role: "user", Content: "Hello"}},
		SystemMsg:   "You are helpful.",
		Temperature: 0.7,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "I'm Claude, here to help!" {
		t.Errorf("unexpected content: %s", resp.Content)
	}
	if resp.InputTokens != 15 {
		t.Errorf("expected 15 input tokens, got %d", resp.InputTokens)
	}
	if resp.OutputTokens != 8 {
		t.Errorf("expected 8 output tokens, got %d", resp.OutputTokens)
	}

	// Check request headers
	if mock.lastReq.Header.Get("x-api-key") != "test-key" {
		t.Error("expected x-api-key header")
	}
	if mock.lastReq.Header.Get("anthropic-version") != "2023-06-01" {
		t.Error("expected anthropic-version header")
	}
}

func TestClaudeProviderChatError(t *testing.T) {
	mock := &mockHTTPClient{
		statusCode: 429,
		body:       `{"error": "rate_limited"}`,
	}

	p := NewClaudeProvider("test-key").WithHTTPClient(mock)
	_, err := p.Chat(&Request{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err == nil {
		t.Error("expected error for 429 status")
	}
}

func TestClaudeProviderSystemInMessages(t *testing.T) {
	mock := &mockHTTPClient{
		statusCode: 200,
		body: `{"content":[{"text":"ok"}],"model":"m","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`,
	}

	p := NewClaudeProvider("test-key").WithHTTPClient(mock)
	_, err := p.Chat(&Request{
		Messages: []Message{
			{Role: "system", Content: "Be helpful"},
			{Role: "user", Content: "Hi"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify system message was extracted and put in body.system, not in messages
	body, _ := io.ReadAll(mock.lastReq.Body)
	var parsed map[string]interface{}
	json.Unmarshal(body, &parsed)
	if parsed["system"] != "Be helpful" {
		t.Error("expected system message in body.system")
	}
	messages := parsed["messages"].([]interface{})
	for _, m := range messages {
		msg := m.(map[string]interface{})
		if msg["role"] == "system" {
			t.Error("system message should not be in messages array")
		}
	}
}

// --- OpenAI Provider ---

func TestOpenAIProviderChat(t *testing.T) {
	mock := &mockHTTPClient{
		statusCode: 200,
		body: `{
			"choices": [{"message": {"content": "I'm GPT!"}, "finish_reason": "stop"}],
			"model": "gpt-4o",
			"usage": {"prompt_tokens": 12, "completion_tokens": 6}
		}`,
	}

	p := NewOpenAIProvider("test-key").WithHTTPClient(mock)
	resp, err := p.Chat(&Request{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "I'm GPT!" {
		t.Errorf("unexpected content: %s", resp.Content)
	}
	if resp.InputTokens != 12 {
		t.Errorf("expected 12 input tokens, got %d", resp.InputTokens)
	}

	// Check auth header
	if mock.lastReq.Header.Get("Authorization") != "Bearer test-key" {
		t.Error("expected Bearer auth header")
	}
}

func TestOpenAIProviderName(t *testing.T) {
	p := NewOpenAIProvider("key")
	if p.Name() != "openai" {
		t.Errorf("expected 'openai', got '%s'", p.Name())
	}
}

// --- Gemini Provider ---

func TestGeminiProviderChat(t *testing.T) {
	mock := &mockHTTPClient{
		statusCode: 200,
		body: `{
			"candidates": [{"content": {"parts": [{"text": "I'm Gemini!"}]}, "finishReason": "STOP"}],
			"usageMetadata": {"promptTokenCount": 10, "candidatesTokenCount": 4}
		}`,
	}

	p := NewGeminiProvider("test-key").WithHTTPClient(mock)
	resp, err := p.Chat(&Request{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "I'm Gemini!" {
		t.Errorf("unexpected content: %s", resp.Content)
	}
	if resp.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.InputTokens)
	}
}

func TestGeminiProviderName(t *testing.T) {
	p := NewGeminiProvider("key")
	if p.Name() != "gemini" {
		t.Errorf("expected 'gemini', got '%s'", p.Name())
	}
}

func TestGeminiProviderNoKey(t *testing.T) {
	p := NewGeminiProvider("")
	_, err := p.Chat(&Request{Messages: []Message{{Role: "user", Content: "Hi"}}})
	if err == nil {
		t.Error("expected error without API key")
	}
}
