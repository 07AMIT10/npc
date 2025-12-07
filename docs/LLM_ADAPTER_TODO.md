# LLM Adapter Integration - TODO

## Overview

We created a new plug-and-play LLM adapter layer in `internal/llm/` but it's **not yet connected** to the game. The game still uses the old `api.Manager` provider logic.

---

## Current State

✅ **Completed:**
- `internal/llm/provider.go` - Interface + types
- `internal/llm/openai_adapter.go` - Groq, OpenRouter, SambaNova, HuggingFace
- `internal/llm/gemini_adapter.go` - Google Gemini
- `internal/llm/balancer.go` - Weighted round-robin (nginx-style)
- `internal/llm/router.go` - Main entry with rate limiting
- `internal/llm/balancer_test.go` - Unit tests (30:20:10 verified)
- `config.yaml` updated with `protocol` and `weight` fields

❌ **Not Connected:**
- `api.Manager` still uses its own provider code
- Weighted load balancing not active in game

---

## TODO: Refactor api.Manager

### Step 1: Add llm.Router to Manager struct

```go
// internal/api/manager.go

import "github.com/amit/npc/internal/llm"

type Manager struct {
    slmRouter   *llm.Router  // NEW: Replace slmProviders
    brainRouter *llm.Router  // NEW: Replace brainProviders
    // ... keep existing fields for backward compat
}
```

### Step 2: Update NewManager()

```go
func NewManager(cfg *config.Config) *Manager {
    // Convert config.ProviderConfig to llm.ProviderConfig
    slmConfigs := convertToLLMConfig(cfg.SLMProviders)
    brainConfigs := convertToLLMConfig(cfg.BrainProviders)
    
    m := &Manager{
        slmRouter:   llm.NewRouter(slmConfigs),
        brainRouter: llm.NewRouter(brainConfigs),
        // ...
    }
    return m
}
```

### Step 3: Update GetEnhancedDecision()

```go
func (m *Manager) GetEnhancedDecision(observation map[string]interface{}) (map[string]interface{}, error) {
    prompt := promptBuilder.BuildMovementPrompt(observation)
    
    // Use new router instead of callProviderWithRetry
    result, err := m.slmRouter.Complete(context.Background(), prompt, llm.DefaultCompletionOpts())
    if err != nil {
        return DefaultDecision(observation), err
    }
    
    return parseActionResponse(result.Content, observation)
}
```

### Step 4: Update GetStrategy() and GetCommentary()

Same pattern - use `m.brainRouter.Complete()` instead of direct provider calls.

### Step 5: Update config.Config struct

```go
// internal/config/config.go

type ProviderConfig struct {
    Name     string `yaml:"name"`
    Protocol string `yaml:"protocol"`  // ADD
    Enabled  bool   `yaml:"enabled"`
    APIKey   string `yaml:"api_key"`
    BaseURL  string `yaml:"base_url"`
    Model    string `yaml:"model"`
    Weight   int    `yaml:"weight"`    // ADD
}
```

### Step 6: Remove old provider code

After migration, remove:
- Old `callProvider()`, `callOpenAICompatible()`, `callGemini()`, `callHuggingFace()` methods
- Old `RateLimiter` (now in llm package)
- Old provider iteration logic

---

## Testing After Migration

```bash
# Unit tests
go test ./internal/llm/... -v

# Integration tests (with API keys)
source .env
go test ./internal/llm/... -v -tags=integration

# Run server and check /health, /test endpoints
go run ./cmd/server/main.go
curl http://localhost:8080/health
curl http://localhost:8080/test
```

---

## Weight Configuration

Set weights in `config.yaml` or via environment:

```bash
# Env overrides
export LLM_GROQ_WEIGHT=5      # 5x more requests
export LLM_HF_WEIGHT=1        # baseline
export LLM_GEMINI_WEIGHT=2    # 2x more
```

Higher weight = more requests routed to that provider.
