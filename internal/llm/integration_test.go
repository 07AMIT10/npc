//go:build integration

package llm

import (
	"context"
	"os"
	"testing"
	"time"
)

// Integration tests - run with: go test -tags=integration -v

func TestGroqRealAPI(t *testing.T) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		t.Skip("GROQ_API_KEY not set")
	}

	adapter := NewOpenAIAdapter(ProviderConfig{
		Name:    "groq",
		BaseURL: "https://api.groq.com/openai/v1",
		APIKey:  apiKey,
		Model:   os.Getenv("GROQ_MODEL"),
	})

	if adapter.model == "" {
		adapter.model = "llama-3.1-8b-instant"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := adapter.Complete(ctx, "Say 'hello' in one word", CompletionOpts{
		MaxTokens:   10,
		Temperature: 0,
	})

	if err != nil {
		t.Fatalf("Groq API failed: %v", err)
	}

	t.Logf("Groq response: %s (latency: %v)", result.Content, result.Latency)
}

func TestGeminiRealAPI(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set")
	}

	adapter := NewGeminiAdapter(ProviderConfig{
		Name:   "gemini",
		APIKey: apiKey,
		Model:  os.Getenv("GEMINI_MODEL"),
	})

	if adapter.model == "" {
		adapter.model = "gemini-2.0-flash"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := adapter.Complete(ctx, "Say 'hello' in one word", CompletionOpts{
		MaxTokens:   10,
		Temperature: 0,
	})

	if err != nil {
		t.Fatalf("Gemini API failed: %v", err)
	}

	t.Logf("Gemini response: %s (latency: %v)", result.Content, result.Latency)
}

func TestHuggingFaceRealAPI(t *testing.T) {
	apiKey := os.Getenv("HF_API_KEY")
	if apiKey == "" {
		t.Skip("HF_API_KEY not set")
	}

	adapter := NewOpenAIAdapter(ProviderConfig{
		Name:    "huggingface",
		BaseURL: "https://router.huggingface.co/v1",
		APIKey:  apiKey,
		Model:   os.Getenv("HF_MODEL"),
	})

	if adapter.model == "" {
		adapter.model = "meta-llama/Llama-3.2-3B-Instruct"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := adapter.Complete(ctx, "Say 'hello' in one word", CompletionOpts{
		MaxTokens:   10,
		Temperature: 0,
	})

	if err != nil {
		t.Fatalf("HuggingFace API failed: %v", err)
	}

	t.Logf("HuggingFace response: %s (latency: %v)", result.Content, result.Latency)
}
