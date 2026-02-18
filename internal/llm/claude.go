package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const claudeAPIURL = "https://api.anthropic.com/v1/messages"

// ClaudeProvider implements Provider for Anthropic's Claude API.
type ClaudeProvider struct {
	apiKey     string
	httpClient HTTPClient
}

// HTTPClient is an abstraction over http.Client for testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewClaudeProvider(apiKey string) *ClaudeProvider {
	return &ClaudeProvider{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// WithHTTPClient sets a custom HTTP client (for testing).
func (p *ClaudeProvider) WithHTTPClient(client HTTPClient) *ClaudeProvider {
	p.httpClient = client
	return p
}

func (p *ClaudeProvider) Name() string { return "claude" }

func (p *ClaudeProvider) Available() bool { return p.apiKey != "" }

func (p *ClaudeProvider) Chat(req *Request) (*Response, error) {
	if !p.Available() {
		return nil, fmt.Errorf("claude provider: API key not configured")
	}

	// Build Claude API request
	messages := make([]map[string]string, 0, len(req.Messages))
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			continue // system message is handled separately
		}
		messages = append(messages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	model := req.Model
	if model == "" {
		model = "claude-sonnet-4-5-20250929"
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	body := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": req.Temperature,
	}

	// Use system message if provided
	systemMsg := req.SystemMsg
	if systemMsg == "" {
		for _, msg := range req.Messages {
			if msg.Role == "system" {
				systemMsg = msg.Content
				break
			}
		}
	}
	if systemMsg != "" {
		body["system"] = systemMsg
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("claude provider: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", claudeAPIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("claude provider: failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("claude provider: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("claude provider: failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude provider: API returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var claudeResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return nil, fmt.Errorf("claude provider: failed to parse response: %w", err)
	}

	content := ""
	if len(claudeResp.Content) > 0 {
		content = claudeResp.Content[0].Text
	}

	return &Response{
		Content:      content,
		Model:        claudeResp.Model,
		InputTokens:  claudeResp.Usage.InputTokens,
		OutputTokens: claudeResp.Usage.OutputTokens,
		StopReason:   claudeResp.StopReason,
	}, nil
}
