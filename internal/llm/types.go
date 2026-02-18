package llm

// Message represents a single message in a conversation
type Message struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// Request is the input to an LLM provider
type Request struct {
	Messages    []Message `json:"messages"`
	Model       string    `json:"model"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"maxTokens,omitempty"`
	SystemMsg   string    `json:"systemMsg,omitempty"`
}

// Response is the output from an LLM provider
type Response struct {
	Content      string `json:"content"`
	Model        string `json:"model"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	StopReason   string `json:"stopReason,omitempty"`
}

// Provider is the abstraction layer for all LLM backends.
// Implementations must be safe for concurrent use.
type Provider interface {
	// Name returns the provider identifier (e.g., "claude", "openai", "gemini")
	Name() string

	// Chat sends a conversation to the LLM and returns its response.
	Chat(req *Request) (*Response, error)

	// Available returns true if this provider is configured (has API keys, etc.)
	Available() bool
}

// TokenUsage tracks cumulative token usage for cost tracking
type TokenUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	Requests     int `json:"requests"`
}

func (u *TokenUsage) Add(resp *Response) {
	u.InputTokens += resp.InputTokens
	u.OutputTokens += resp.OutputTokens
	u.Requests++
}
