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

// GeminiAdapter handles Google Gemini API
type GeminiAdapter struct {
	name       string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewGeminiAdapter creates a new Gemini adapter
func NewGeminiAdapter(cfg ProviderConfig) *GeminiAdapter {
	model := cfg.Model
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GeminiAdapter{
		name:   cfg.Name,
		apiKey: cfg.APIKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider identifier
func (a *GeminiAdapter) Name() string {
	return a.name
}

// Protocol returns ProtocolGemini
func (a *GeminiAdapter) Protocol() Protocol {
	return ProtocolGemini
}

// Complete sends a completion request to Gemini API
func (a *GeminiAdapter) Complete(ctx context.Context, prompt string, opts CompletionOpts) (*CompletionResult, error) {
	startTime := time.Now()

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		a.model, a.apiKey,
	)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:     opts.Temperature,
			MaxOutputTokens: opts.MaxTokens,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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

	var result geminiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("[%s] failed to parse response: %w", a.name, err)
	}

	if result.Error.Message != "" {
		return nil, fmt.Errorf("[%s] API error: %s", a.name, result.Error.Message)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("[%s] no response returned", a.name)
	}

	return &CompletionResult{
		Content:  result.Candidates[0].Content.Parts[0].Text,
		Provider: a.name,
		Model:    a.model,
		Latency:  time.Since(startTime),
	}, nil
}

// HealthCheck verifies the provider is working
func (a *GeminiAdapter) HealthCheck(ctx context.Context) error {
	_, err := a.Complete(ctx, "Say 'ok'", CompletionOpts{MaxTokens: 5, Temperature: 0})
	return err
}

// Gemini API request/response structures
type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}
