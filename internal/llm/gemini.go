package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const geminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models"

// GeminiProvider implements Provider for Google's Gemini API.
type GeminiProvider struct {
	apiKey     string
	httpClient HTTPClient
}

func NewGeminiProvider(apiKey string) *GeminiProvider {
	return &GeminiProvider{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

func (p *GeminiProvider) WithHTTPClient(client HTTPClient) *GeminiProvider {
	p.httpClient = client
	return p
}

func (p *GeminiProvider) Name() string { return "gemini" }

func (p *GeminiProvider) Available() bool { return p.apiKey != "" }

func (p *GeminiProvider) Chat(req *Request) (*Response, error) {
	if !p.Available() {
		return nil, fmt.Errorf("gemini provider: API key not configured")
	}

	model := req.Model
	if model == "" {
		model = "gemini-2.0-flash"
	}

	// Build Gemini contents format
	var contents []map[string]interface{}

	// Add system instruction via systemInstruction field
	systemMsg := req.SystemMsg
	if systemMsg == "" {
		for _, msg := range req.Messages {
			if msg.Role == "system" {
				systemMsg = msg.Content
				break
			}
		}
	}

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			continue
		}
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role": role,
			"parts": []map[string]string{
				{"text": msg.Content},
			},
		})
	}

	body := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature": req.Temperature,
		},
	}
	if req.MaxTokens > 0 {
		body["generationConfig"].(map[string]interface{})["maxOutputTokens"] = req.MaxTokens
	}
	if systemMsg != "" {
		body["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]string{
				{"text": systemMsg},
			},
		}
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("gemini provider: failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", geminiAPIURL, model, p.apiKey)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("gemini provider: failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini provider: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini provider: failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini provider: API returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("gemini provider: failed to parse response: %w", err)
	}

	content := ""
	stopReason := ""
	if len(geminiResp.Candidates) > 0 {
		if len(geminiResp.Candidates[0].Content.Parts) > 0 {
			content = geminiResp.Candidates[0].Content.Parts[0].Text
		}
		stopReason = geminiResp.Candidates[0].FinishReason
	}

	return &Response{
		Content:      content,
		Model:        model,
		InputTokens:  geminiResp.UsageMetadata.PromptTokenCount,
		OutputTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
		StopReason:   stopReason,
	}, nil
}
