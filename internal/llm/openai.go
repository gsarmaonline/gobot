package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openaiAPIURL = "https://api.openai.com/v1/chat/completions"

// OpenAIProvider implements Provider for OpenAI's API.
type OpenAIProvider struct {
	apiKey     string
	httpClient HTTPClient
}

func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

func (p *OpenAIProvider) WithHTTPClient(client HTTPClient) *OpenAIProvider {
	p.httpClient = client
	return p
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Available() bool { return p.apiKey != "" }

func (p *OpenAIProvider) Chat(req *Request) (*Response, error) {
	if !p.Available() {
		return nil, fmt.Errorf("openai provider: API key not configured")
	}

	messages := make([]map[string]string, 0, len(req.Messages)+1)

	// Add system message
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
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": systemMsg,
		})
	}

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			continue
		}
		messages = append(messages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	model := req.Model
	if model == "" {
		model = "gpt-4o"
	}

	body := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai provider: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", openaiAPIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("openai provider: failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai provider: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai provider: failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai provider: API returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var openaiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Model string `json:"model"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("openai provider: failed to parse response: %w", err)
	}

	content := ""
	stopReason := ""
	if len(openaiResp.Choices) > 0 {
		content = openaiResp.Choices[0].Message.Content
		stopReason = openaiResp.Choices[0].FinishReason
	}

	return &Response{
		Content:      content,
		Model:        openaiResp.Model,
		InputTokens:  openaiResp.Usage.PromptTokens,
		OutputTokens: openaiResp.Usage.CompletionTokens,
		StopReason:   stopReason,
	}, nil
}
