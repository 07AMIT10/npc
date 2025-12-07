// Package llm provides a plug-and-play LLM adapter layer with load balancing.
package llm

import (
	"context"
	"time"
)

// Protocol defines the API format for a provider
type Protocol string

const (
	// ProtocolOpenAI is for OpenAI-compatible APIs (Groq, OpenRouter, SambaNova, etc.)
	ProtocolOpenAI Protocol = "openai"
	// ProtocolGemini is for Google Gemini API
	ProtocolGemini Protocol = "gemini"
)

// Provider is the interface that all LLM adapters must implement.
// This enables plug-and-play switching between providers.
type Provider interface {
	// Name returns the provider identifier (e.g., "groq", "gemini")
	Name() string

	// Complete sends a prompt to the LLM and returns the response
	Complete(ctx context.Context, prompt string, opts CompletionOpts) (*CompletionResult, error)

	// HealthCheck verifies the provider is working
	HealthCheck(ctx context.Context) error

	// Protocol returns the API protocol type
	Protocol() Protocol
}

// CompletionOpts contains parameters for an LLM completion request
type CompletionOpts struct {
	MaxTokens   int
	Temperature float64
}

// DefaultCompletionOpts returns sensible defaults
func DefaultCompletionOpts() CompletionOpts {
	return CompletionOpts{
		MaxTokens:   100,
		Temperature: 0.7,
	}
}

// CompletionResult contains the response from an LLM
type CompletionResult struct {
	Content   string        // The generated text
	Provider  string        // Which provider was used
	Model     string        // Which model was used
	Latency   time.Duration // How long the request took
	TokensIn  int           // Input tokens (if available)
	TokensOut int           // Output tokens (if available)
}

// ProviderConfig holds configuration for a single provider
type ProviderConfig struct {
	Name     string   `yaml:"name"`
	Protocol Protocol `yaml:"protocol"`
	BaseURL  string   `yaml:"base_url"`
	APIKey   string   `yaml:"api_key"`
	Model    string   `yaml:"model"`
	Weight   int      `yaml:"weight"` // For load balancing (higher = more requests)
	Enabled  bool     `yaml:"enabled"`
}
