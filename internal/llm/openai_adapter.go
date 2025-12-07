package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIAdapter handles all OpenAI-compatible APIs
// This includes: Groq, OpenRouter, SambaNova, Together, HuggingFace, and OpenAI itself
type OpenAIAdapter struct {
	name       string
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAIAdapter creates a new OpenAI-compatible adapter
func NewOpenAIAdapter(cfg ProviderConfig) *OpenAIAdapter {
	return &OpenAIAdapter{
		name:    cfg.Name,
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider identifier
func (a *OpenAIAdapter) Name() string {
	return a.name
}

// Protocol returns ProtocolOpenAI
func (a *OpenAIAdapter) Protocol() Protocol {
	return ProtocolOpenAI
}

// Complete sends a completion request to the OpenAI-compatible API
func (a *OpenAIAdapter) Complete(ctx context.Context, prompt string, opts CompletionOpts) (*CompletionResult, error) {
	startTime := time.Now()

	reqBody := map[string]interface{}{
		"model": a.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": opts.Temperature,
		"max_tokens":  opts.MaxTokens,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := a.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("[%s] HTTP %d: %s", a.name, resp.StatusCode, truncateString(string(respBody), 200))
	}

	var result openAIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("[%s] failed to parse response: %w", a.name, err)
	}

	if result.Error.Message != "" {
		return nil, fmt.Errorf("[%s] API error: %s", a.name, result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("[%s] no response choices returned", a.name)
	}

	return &CompletionResult{
		Content:   result.Choices[0].Message.Content,
		Provider:  a.name,
		Model:     a.model,
		Latency:   time.Since(startTime),
		TokensIn:  result.Usage.PromptTokens,
		TokensOut: result.Usage.CompletionTokens,
	}, nil
}

// HealthCheck verifies the provider is working
func (a *OpenAIAdapter) HealthCheck(ctx context.Context) error {
	_, err := a.Complete(ctx, "Say 'ok'", CompletionOpts{MaxTokens: 5, Temperature: 0})
	return err
}

// openAIResponse represents the OpenAI API response format
type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
