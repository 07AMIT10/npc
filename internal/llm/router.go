package llm

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Router is the main entry point for LLM operations.
// It manages multiple providers with load balancing and rate limiting.
type Router struct {
	balancer    *Balancer
	rateLimiter *RateLimiter
	npcMapping  map[string]Provider // Per-NPC provider overrides
	mu          sync.RWMutex

	// Statistics
	successCount map[string]int
	errorCount   map[string]int
	lastError    map[string]string
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a rate limiter
func NewRateLimiter(maxTokens, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available
func (r *RateLimiter) Wait(tokens float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens = min(r.maxTokens, r.tokens+elapsed*r.refillRate)
	r.lastRefill = now

	if r.tokens < tokens {
		waitTime := time.Duration((tokens - r.tokens) / r.refillRate * float64(time.Second))
		time.Sleep(waitTime)
		r.tokens = 0
	} else {
		r.tokens -= tokens
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// NewRouter creates a router from provider configurations
func NewRouter(configs []ProviderConfig) *Router {
	providers := make([]Provider, 0, len(configs))
	weights := make(map[string]int)

	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}

		// Check for API key
		apiKey := cfg.APIKey
		if apiKey == "" {
			apiKey = getEnvAPIKey(cfg.Name)
		}
		if apiKey == "" {
			log.Printf("⚠️  Skipping %s: no API key", cfg.Name)
			continue
		}

		// Check for weight override from env
		weight := cfg.Weight
		if envWeight := os.Getenv(fmt.Sprintf("LLM_%s_WEIGHT", strings.ToUpper(cfg.Name))); envWeight != "" {
			if w, err := strconv.Atoi(envWeight); err == nil && w > 0 {
				weight = w
			}
		}
		if weight <= 0 {
			weight = 1
		}
		weights[cfg.Name] = weight

		// Check for model override from env
		model := cfg.Model
		if envModel := os.Getenv(fmt.Sprintf("%s_MODEL", strings.ToUpper(cfg.Name))); envModel != "" {
			model = envModel
		}

		// Create provider based on protocol
		var provider Provider
		switch cfg.Protocol {
		case ProtocolGemini:
			provider = NewGeminiAdapter(ProviderConfig{
				Name:   cfg.Name,
				APIKey: apiKey,
				Model:  model,
			})
		case ProtocolOpenAI:
			fallthrough
		default:
			provider = NewOpenAIAdapter(ProviderConfig{
				Name:    cfg.Name,
				BaseURL: cfg.BaseURL,
				APIKey:  apiKey,
				Model:   model,
			})
		}

		providers = append(providers, provider)
		log.Printf("✅ Loaded provider: %s (weight=%d, model=%s)", cfg.Name, weight, model)
	}

	return &Router{
		balancer:     NewBalancer(providers, weights),
		rateLimiter:  NewRateLimiter(5, 1.0),
		npcMapping:   make(map[string]Provider),
		successCount: make(map[string]int),
		errorCount:   make(map[string]int),
		lastError:    make(map[string]string),
	}
}

// Complete sends a prompt to an LLM provider selected by load balancer
func (r *Router) Complete(ctx context.Context, prompt string, opts CompletionOpts) (*CompletionResult, error) {
	r.rateLimiter.Wait(1)

	provider := r.balancer.Next()
	if provider == nil {
		return nil, fmt.Errorf("no providers available")
	}

	result, err := provider.Complete(ctx, prompt, opts)
	if err != nil {
		r.recordError(provider.Name(), err)
		return nil, err
	}

	r.recordSuccess(provider.Name())
	return result, nil
}

// CompleteWithProvider sends to a specific provider (for NPC mapping)
func (r *Router) CompleteWithProvider(ctx context.Context, providerName, prompt string, opts CompletionOpts) (*CompletionResult, error) {
	r.rateLimiter.Wait(1)

	provider := r.balancer.GetByName(providerName)
	if provider == nil {
		// Fallback to load balancer
		provider = r.balancer.Next()
	}
	if provider == nil {
		return nil, fmt.Errorf("no providers available")
	}

	result, err := provider.Complete(ctx, prompt, opts)
	if err != nil {
		r.recordError(provider.Name(), err)
		return nil, err
	}

	r.recordSuccess(provider.Name())
	return result, nil
}

// GetProviderForNPC returns the assigned provider for an NPC
func (r *Router) GetProviderForNPC(npcName string) Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if p, ok := r.npcMapping[npcName]; ok {
		return p
	}
	return r.balancer.Next()
}

// SetNPCProvider assigns a specific provider to an NPC
func (r *Router) SetNPCProvider(npcName, providerName string) {
	provider := r.balancer.GetByName(providerName)
	if provider == nil {
		return
	}

	r.mu.Lock()
	r.npcMapping[npcName] = provider
	r.mu.Unlock()
}

// GetActiveProviders returns list of active provider names
func (r *Router) GetActiveProviders() []string {
	providers := r.balancer.GetAll()
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name()
	}
	return names
}

// GetStats returns provider statistics
func (r *Router) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"success":   r.successCount,
		"errors":    r.errorCount,
		"lastError": r.lastError,
	}
}

// TestProviders tests all configured providers
func (r *Router) TestProviders(ctx context.Context) []ProviderTestResult {
	providers := r.balancer.GetAll()
	results := make([]ProviderTestResult, 0, len(providers))

	for _, p := range providers {
		startTime := time.Now()
		err := p.HealthCheck(ctx)
		latency := time.Since(startTime)

		result := ProviderTestResult{
			Provider: p.Name(),
			Latency:  latency,
		}

		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		} else {
			result.Status = "ok"
		}

		results = append(results, result)
	}

	return results
}

// ProviderTestResult contains the result of testing a provider
type ProviderTestResult struct {
	Provider string        `json:"provider"`
	Status   string        `json:"status"`
	Latency  time.Duration `json:"latency"`
	Error    string        `json:"error,omitempty"`
}

func (r *Router) recordSuccess(provider string) {
	r.mu.Lock()
	r.successCount[provider]++
	r.mu.Unlock()
}

func (r *Router) recordError(provider string, err error) {
	r.mu.Lock()
	r.errorCount[provider]++
	r.lastError[provider] = err.Error()
	r.mu.Unlock()
}

// getEnvAPIKey gets API key from environment
func getEnvAPIKey(provider string) string {
	envMap := map[string]string{
		"groq":        "GROQ_API_KEY",
		"sambanova":   "SAMBANOVA_API_KEY",
		"openrouter":  "OPENROUTER_API_KEY",
		"huggingface": "HF_API_KEY",
		"nebius":      "NEBIUS_API_KEY",
		"gemini":      "GEMINI_API_KEY",
		"openai":      "OPENAI_API_KEY",
	}
	if envName, ok := envMap[strings.ToLower(provider)]; ok {
		return os.Getenv(envName)
	}
	return ""
}
