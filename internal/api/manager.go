package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/amit/npc/internal/config"
)

// Manager handles multiple LLM API providers with rate limiting
type Manager struct {
	slmProviders   []Provider
	brainProviders []Provider
	activeSLM      *Provider
	activeBrain    *Provider
	httpClient     *http.Client

	// Per-NPC provider mapping
	npcProviders  map[string]*Provider // npc_name -> provider
	providerIndex int                  // for round-robin fallback

	// Rate limiting
	rateLimiter     *RateLimiter
	lastCallTime    time.Time
	minCallInterval time.Duration
	mu              sync.Mutex

	// Audit logging
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

func NewRateLimiter(maxTokens, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (r *RateLimiter) Wait(tokens float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens = min(r.maxTokens, r.tokens+elapsed*r.refillRate)
	r.lastRefill = now

	if r.tokens < tokens {
		waitTime := time.Duration((tokens - r.tokens) / r.refillRate * float64(time.Second))
		log.Printf("⏳ Rate limiting: waiting %.1fs", waitTime.Seconds())
		time.Sleep(waitTime)
		r.tokens = 0
	} else {
		r.tokens -= tokens
	}
}

// Provider represents an LLM API provider
type Provider struct {
	Name    string
	BaseURL string
	APIKey  string
	Model   string
	Enabled bool
}

// NewManager creates a new API manager with rate limiting
func NewManager(cfg *config.Config) *Manager {
	m := &Manager{
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		rateLimiter:     NewRateLimiter(5, 1.0),
		minCallInterval: 500 * time.Millisecond,
		npcProviders:    make(map[string]*Provider),
		successCount:    make(map[string]int),
		errorCount:      make(map[string]int),
		lastError:       make(map[string]string),
	}

	// Load SLM providers
	for _, p := range cfg.SLMProviders {
		if !p.Enabled {
			continue
		}
		apiKey := p.APIKey
		if apiKey == "" {
			apiKey = getEnvKey(p.Name)
		}
		if apiKey == "" {
			continue
		}
		model := getEnvModel(p.Name, p.Model)

		provider := Provider{
			Name:    p.Name,
			BaseURL: p.BaseURL,
			APIKey:  apiKey,
			Model:   model,
			Enabled: true,
		}
		m.slmProviders = append(m.slmProviders, provider)
		if m.activeSLM == nil {
			m.activeSLM = &provider
		}
	}

	// Load Brain providers
	for _, p := range cfg.BrainProviders {
		if !p.Enabled {
			continue
		}
		apiKey := p.APIKey
		if apiKey == "" {
			apiKey = getEnvKey(p.Name)
		}
		if apiKey == "" {
			continue
		}
		model := getEnvModel(p.Name, p.Model)

		provider := Provider{
			Name:    p.Name,
			BaseURL: p.BaseURL,
			APIKey:  apiKey,
			Model:   model,
			Enabled: true,
		}
		m.brainProviders = append(m.brainProviders, provider)
		if m.activeBrain == nil {
			m.activeBrain = &provider
		}
	}

	// Load per-NPC provider and model assignments
	npcNames := []string{"Explorer", "Scout", "Wanderer", "Seeker"}
	for _, name := range npcNames {
		providerEnv := fmt.Sprintf("NPC_%s_PROVIDER", strings.ToUpper(name))
		modelEnv := fmt.Sprintf("NPC_%s_MODEL", strings.ToUpper(name))

		providerName := os.Getenv(providerEnv)
		modelOverride := os.Getenv(modelEnv)

		if providerName != "" {
			for i := range m.slmProviders {
				if strings.EqualFold(m.slmProviders[i].Name, providerName) {
					npcProvider := m.slmProviders[i]
					if modelOverride != "" {
						npcProvider.Model = modelOverride
					}
					m.npcProviders[name] = &npcProvider
					log.Printf("📍 NPC %s → %s (%s)", name, npcProvider.Name, npcProvider.Model)
					break
				}
			}
		}
	}

	return m
}

// GetProviderForNPC returns the provider for a specific NPC
func (m *Manager) GetProviderForNPC(npcName string) *Provider {
	if provider, ok := m.npcProviders[npcName]; ok && provider != nil {
		return provider
	}

	if len(m.slmProviders) == 0 {
		return nil
	}

	m.mu.Lock()
	provider := &m.slmProviders[m.providerIndex%len(m.slmProviders)]
	m.providerIndex++
	m.mu.Unlock()

	return provider
}

func getEnvKey(provider string) string {
	envMap := map[string]string{
		"groq":        "GROQ_API_KEY",
		"sambanova":   "SAMBANOVA_API_KEY",
		"openrouter":  "OPENROUTER_API_KEY",
		"huggingface": "HF_API_KEY",
		"nebius":      "NEBIUS_API_KEY",
		"gemini":      "GEMINI_API_KEY",
	}
	if envName, ok := envMap[provider]; ok {
		return os.Getenv(envName)
	}
	return ""
}

func getEnvModel(provider, defaultModel string) string {
	envMap := map[string]string{
		"groq":        "GROQ_MODEL",
		"sambanova":   "SAMBANOVA_MODEL",
		"openrouter":  "OPENROUTER_MODEL",
		"huggingface": "HF_MODEL",
		"nebius":      "NEBIUS_MODEL",
		"gemini":      "GEMINI_MODEL",
		"openai":      "OPENAI_MODEL",
	}
	if envName, ok := envMap[provider]; ok {
		if model := os.Getenv(envName); model != "" {
			return model
		}
	}
	return defaultModel
}

// GetActiveSLM returns the active SLM provider name
func (m *Manager) GetActiveSLM() string {
	if m.activeSLM != nil {
		return fmt.Sprintf("%s (%s)", m.activeSLM.Name, m.activeSLM.Model)
	}
	return "none (demo mode)"
}

// GetActiveBrain returns the active brain provider name
func (m *Manager) GetActiveBrain() string {
	if m.activeBrain != nil {
		return fmt.Sprintf("%s (%s)", m.activeBrain.Name, m.activeBrain.Model)
	}
	return "none (demo mode)"
}

// GetStats returns provider statistics
func (m *Manager) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"success":   m.successCount,
		"errors":    m.errorCount,
		"lastError": m.lastError,
	}
}

// ProviderTestResult contains the result of testing a provider
type ProviderTestResult struct {
	Status   string `json:"status"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Latency  string `json:"latency"`
	Response string `json:"response,omitempty"`
	Error    string `json:"error,omitempty"`
}

// TestProviders tests all configured providers and returns results
func (m *Manager) TestProviders() []ProviderTestResult {
	results := []ProviderTestResult{}
	testPrompt := `Reply with exactly: {"action":"idle","reason":"test"}`

	// Test SLM providers
	for i := range m.slmProviders {
		p := &m.slmProviders[i]
		startTime := time.Now()

		resp, err := m.callProvider(p, testPrompt)
		latency := time.Since(startTime).Milliseconds()

		result := ProviderTestResult{
			Provider: p.Name,
			Model:    p.Model,
			Latency:  fmt.Sprintf("%dms", latency),
		}

		if err != nil {
			result.Status = "❌ FAILED"
			result.Error = err.Error()
			log.Printf("❌ TEST %s (%s): %s", p.Name, p.Model, truncateError(err))
		} else {
			result.Status = "✅ OK"
			result.Response = truncateForLog(resp, 80)
			log.Printf("✅ TEST %s (%s): %dms", p.Name, p.Model, latency)
		}
		results = append(results, result)
	}

	// Test Brain providers
	for i := range m.brainProviders {
		p := &m.brainProviders[i]
		startTime := time.Now()

		var resp string
		var err error

		if p.Name == "gemini" {
			resp, err = m.callGemini(p, "Say hello in 3 words")
		} else {
			resp, err = m.callOpenAICompatible(p, "Say hello in 3 words")
		}

		latency := time.Since(startTime).Milliseconds()

		result := ProviderTestResult{
			Provider: p.Name + "_brain",
			Model:    p.Model,
			Latency:  fmt.Sprintf("%dms", latency),
		}

		if err != nil {
			result.Status = "❌ FAILED"
			result.Error = err.Error()
			log.Printf("❌ TEST %s brain (%s): %s", p.Name, p.Model, truncateError(err))
		} else {
			result.Status = "✅ OK"
			result.Response = truncateForLog(resp, 80)
			log.Printf("✅ TEST %s brain (%s): %dms", p.Name, p.Model, latency)
		}
		results = append(results, result)
	}

	return results
}

func truncateForLog(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// throttle ensures minimum time between API calls
func (m *Manager) throttle() {
	m.mu.Lock()
	defer m.mu.Unlock()

	elapsed := time.Since(m.lastCallTime)
	if elapsed < m.minCallInterval {
		time.Sleep(m.minCallInterval - elapsed)
	}
	m.lastCallTime = time.Now()
}

// recordSuccess logs a successful API call
func (m *Manager) recordSuccess(provider string) {
	m.mu.Lock()
	m.successCount[provider]++
	m.mu.Unlock()
}

// recordError logs a failed API call
func (m *Manager) recordError(provider string, err error) {
	m.mu.Lock()
	m.errorCount[provider]++
	m.lastError[provider] = err.Error()
	m.mu.Unlock()
}

// GetDecision gets an action decision from the SLM with rate limiting
func (m *Manager) GetDecision(observation map[string]interface{}) (map[string]interface{}, error) {
	npcName := ""
	if name, ok := observation["name"].(string); ok {
		npcName = name
	}

	provider := m.GetProviderForNPC(npcName)
	if provider == nil {
		return DefaultDecision(observation), nil
	}

	m.rateLimiter.Wait(1)
	m.throttle()

	prompt := buildActionPrompt(observation)
	startTime := time.Now()

	response, err := m.callProviderWithRetry(provider, prompt, 2)
	latency := time.Since(startTime).Milliseconds()

	audit := GetAuditLog()

	if err != nil {
		log.Printf("❌ %s [%s] FAILED: %s", npcName, provider.Name, truncateError(err))
		m.recordError(provider.Name, err)
		audit.LogError(npcName, provider.Name, provider.Model, prompt, latency, err)

		// Try fallback providers
		for i, p := range m.slmProviders {
			if p.Name != provider.Name {
				startTime = time.Now()
				response, err = m.callProviderWithRetry(&m.slmProviders[i], prompt, 1)
				latency = time.Since(startTime).Milliseconds()

				if err == nil {
					log.Printf("✅ %s switched to backup: %s", npcName, p.Name)
					m.recordSuccess(p.Name)
					audit.LogSuccess(npcName, p.Name, p.Model, prompt, response, latency)
					break
				} else {
					m.recordError(p.Name, err)
					audit.LogError(npcName, p.Name, p.Model, prompt, latency, err)
				}
			}
		}
		if err != nil {
			return DefaultDecision(observation), err
		}
	} else {
		m.recordSuccess(provider.Name)
		audit.LogSuccess(npcName, provider.Name, provider.Model, prompt, response, latency)
	}

	return parseActionResponse(response, observation)
}

// GetStrategy gets strategic advice from the brain LLM
func (m *Manager) GetStrategy(summary string) (string, error) {
	if m.activeBrain == nil {
		return "Continue exploring systematically.", nil
	}

	m.rateLimiter.Wait(1)
	m.throttle()

	prompt := buildStrategyPrompt(summary)

	var response string
	var err error

	if m.activeBrain.Name == "gemini" {
		response, err = m.callGeminiWithRetry(m.activeBrain, prompt, 2)
	} else {
		response, err = m.callProviderWithRetry(m.activeBrain, prompt, 2)
	}

	if err != nil {
		log.Printf("❌ Brain [%s] FAILED: %s", m.activeBrain.Name, truncateError(err))
		m.recordError(m.activeBrain.Name, err)
		return "Continue exploring systematically.", err
	}

	m.recordSuccess(m.activeBrain.Name)
	return response, nil
}

func buildActionPrompt(obs map[string]interface{}) string {
	compact := map[string]interface{}{
		"id":    obs["npc_id"],
		"name":  obs["name"],
		"pos":   obs["pos"],
		"state": obs["state"],
	}
	if nearby, ok := obs["nearby_objects"]; ok {
		compact["near"] = nearby
	}

	obsJSON, _ := json.Marshal(compact)
	return fmt.Sprintf(`NPC decision. State: %s
Actions: move(target), explore, interact(target), idle
Reply JSON only: {"action":"...", "target":"...", "reason":"..."}`, string(obsJSON))
}

func buildStrategyPrompt(summary string) string {
	return fmt.Sprintf(`Team coordinator. Situation: %s
Give 1 sentence strategy.`, summary)
}

// truncateError shortens error messages for readable logs
func truncateError(err error) string {
	s := err.Error()
	if len(s) > 100 {
		return s[:100] + "..."
	}
	return s
}

// callProviderWithRetry calls the provider with exponential backoff retry
func (m *Manager) callProviderWithRetry(p *Provider, prompt string, maxRetries int) (string, error) {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			backoff := time.Duration(1<<uint(i-1)) * time.Second
			log.Printf("🔄 [%s] Retry %d/%d after %v", p.Name, i, maxRetries, backoff)
			time.Sleep(backoff)
		}

		response, err := m.callProvider(p, prompt)
		if err == nil {
			return response, nil
		}
		lastErr = err

		if !isRetryableError(err) {
			return "", err
		}
	}
	return "", lastErr
}

// callGeminiWithRetry calls Gemini with exponential backoff retry
func (m *Manager) callGeminiWithRetry(p *Provider, prompt string, maxRetries int) (string, error) {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			backoff := time.Duration(1<<uint(i-1)) * time.Second
			log.Printf("🔄 [%s] Retry %d/%d after %v", p.Name, i, maxRetries, backoff)
			time.Sleep(backoff)
		}

		response, err := m.callGemini(p, prompt)
		if err == nil {
			return response, nil
		}
		lastErr = err

		if !isRetryableError(err) {
			return "", err
		}
	}
	return "", lastErr
}

func isRetryableError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "rate") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "temporary") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "502")
}

// callProvider routes to the correct provider-specific implementation
func (m *Manager) callProvider(p *Provider, prompt string) (string, error) {
	switch p.Name {
	case "huggingface":
		return m.callHuggingFace(p, prompt)
	case "groq", "openrouter", "sambanova", "nebius":
		return m.callOpenAICompatible(p, prompt)
	default:
		return m.callOpenAICompatible(p, prompt)
	}
}

// callOpenAICompatible calls OpenAI-compatible APIs (Groq, OpenRouter, SambaNova, OpenAI)
func (m *Manager) callOpenAICompatible(p *Provider, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model": p.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.7,
		"max_tokens":  100,
	}

	body, _ := json.Marshal(reqBody)
	url := p.BaseURL + "/chat/completions"

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("request creation failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("[%s] HTTP %d: %s", p.Name, resp.StatusCode, truncateError(fmt.Errorf(string(respBody))))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("[%s] JSON parse error: %w", p.Name, err)
	}

	if result.Error.Message != "" {
		return "", fmt.Errorf("[%s] API error: %s", p.Name, result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("[%s] no response choices returned", p.Name)
	}

	return result.Choices[0].Message.Content, nil
}

// callHuggingFace calls HuggingFace Router API with correct format
func (m *Manager) callHuggingFace(p *Provider, prompt string) (string, error) {
	// HuggingFace Router API - model goes in the body, not URL
	url := "https://router.huggingface.co/v1/chat/completions"

	// OpenAI-compatible format
	reqBody := map[string]interface{}{
		"model": p.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  100,
		"temperature": 0.7,
		"stream":      false,
	}

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("request creation failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("[huggingface] HTTP %d: %s", resp.StatusCode, truncateError(fmt.Errorf(string(respBody))))
	}

	// Parse OpenAI-compatible response
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("[huggingface] JSON parse error: %w", err)
	}

	if result.Error.Message != "" {
		return "", fmt.Errorf("[huggingface] API error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("[huggingface] no response returned")
	}

	return result.Choices[0].Message.Content, nil
}

// callGemini calls Google's Gemini API
func (m *Manager) callGemini(p *Provider, prompt string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		p.Model, p.APIKey)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.7,
			"maxOutputTokens": 100,
		},
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("request creation failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("[gemini] HTTP %d: %s", resp.StatusCode, truncateError(fmt.Errorf(string(respBody))))
	}

	var result struct {
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

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("[gemini] JSON parse error: %w", err)
	}

	if result.Error.Message != "" {
		return "", fmt.Errorf("[gemini] API error: %s", result.Error.Message)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("[gemini] no response returned")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}

func parseActionResponse(response string, obs map[string]interface{}) (map[string]interface{}, error) {
	var action map[string]interface{}

	start := -1
	end := -1
	for i, c := range response {
		if c == '{' && start == -1 {
			start = i
		}
		if c == '}' {
			end = i + 1
		}
	}

	if start >= 0 && end > start {
		jsonStr := response[start:end]
		if err := json.Unmarshal([]byte(jsonStr), &action); err == nil {
			action["npc_id"] = obs["npc_id"]

			// VALIDATION: Fix self-targeting for talk/taunt actions (industry best practice)
			actionType, _ := action["action"].(string)
			if actionType == "talk" || actionType == "taunt" {
				target, _ := action["target"].(string)
				npcName, _ := obs["name"].(string)

				// Check if targeting self - this is invalid
				if target == npcName || target == "" {
					// Auto-correct to first nearby NPC
					if nearbyNPCs, ok := obs["nearby_npcs"].([]interface{}); ok && len(nearbyNPCs) > 0 {
						if firstNPC, ok := nearbyNPCs[0].(map[string]interface{}); ok {
							if validTarget, ok := firstNPC["name"].(string); ok {
								action["target"] = validTarget
							}
						}
					}
				}
			}

			return action, nil
		}
	}

	// Fallback: If response looks like plain text (taunt/talk), treat it as such
	trimmed := strings.TrimSpace(response)
	if len(trimmed) > 5 && !strings.HasPrefix(trimmed, "{") {
		// It's probably a taunt or talk that's just text
		// Find nearby NPC to target
		target := "opponent"
		if nearbyNPCs, ok := obs["nearby_npcs"].([]interface{}); ok && len(nearbyNPCs) > 0 {
			if firstNPC, ok := nearbyNPCs[0].(map[string]interface{}); ok {
				if name, ok := firstNPC["name"].(string); ok {
					target = name
				}
			}
		}

		// Clean up the message (remove quotes if present)
		message := strings.Trim(trimmed, "\"'")
		if len(message) > 100 {
			message = message[:100] + "..."
		}

		return map[string]interface{}{
			"npc_id":  obs["npc_id"],
			"action":  "taunt",
			"target":  target,
			"message": message,
		}, nil
	}

	return DefaultDecision(obs), nil
}

// DefaultDecision returns a fallback decision
func DefaultDecision(obs map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"npc_id": obs["npc_id"],
		"action": "explore",
		"reason": "Looking around...",
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// ============ PHASE 2: ENHANCED LLM INTEGRATION ============

var promptBuilder = &PromptBuilder{}

// GetEnhancedDecision uses the new context-rich prompts
func (m *Manager) GetEnhancedDecision(observation map[string]interface{}) (map[string]interface{}, error) {
	npcName := ""
	if name, ok := observation["name"].(string); ok {
		npcName = name
	}

	provider := m.GetProviderForNPC(npcName)
	if provider == nil {
		return DefaultDecision(observation), nil
	}

	m.rateLimiter.Wait(1)
	m.throttle()

	// Use enhanced prompt builder
	prompt := promptBuilder.BuildMovementPrompt(observation)
	startTime := time.Now()

	response, err := m.callProviderWithRetry(provider, prompt, 2)
	latency := time.Since(startTime).Milliseconds()

	audit := GetAuditLog()

	if err != nil {
		log.Printf("❌ %s [%s] FAILED: %s", npcName, provider.Name, truncateError(err))
		m.recordError(provider.Name, err)
		audit.LogError(npcName, provider.Name, provider.Model, "enhanced_prompt", latency, err)
		return DefaultDecision(observation), err
	}

	m.recordSuccess(provider.Name)
	audit.LogSuccess(npcName, provider.Name, provider.Model, "enhanced_prompt", response, latency)

	return parseActionResponse(response, observation)
}

// GetBatchDecision makes a single LLM call for multiple NPCs on the same team
// This reduces API calls from 4 per tick to 2 per tick
func (m *Manager) GetBatchDecision(observations []map[string]interface{}) ([]map[string]interface{}, error) {
	if len(observations) == 0 {
		return nil, nil
	}

	// Get team from first observation
	teamName := ""
	if team, ok := observations[0]["team"].(string); ok {
		teamName = team
	}

	// Use first NPC's provider for the batch
	npcName := ""
	if name, ok := observations[0]["name"].(string); ok {
		npcName = name
	}

	provider := m.GetProviderForNPC(npcName)
	if provider == nil {
		// Return default decisions for all
		results := make([]map[string]interface{}, len(observations))
		for i, obs := range observations {
			results[i] = DefaultDecision(obs)
		}
		return results, nil
	}

	m.rateLimiter.Wait(1)
	m.throttle()

	// Build batch prompt
	prompt := promptBuilder.BuildBatchPrompt(observations)
	startTime := time.Now()

	response, err := m.callProviderWithRetry(provider, prompt, 2)
	latency := time.Since(startTime).Milliseconds()

	audit := GetAuditLog()

	if err != nil {
		log.Printf("❌ Batch [%s] FAILED: %s", teamName, truncateError(err))
		m.recordError(provider.Name, err)
		audit.LogError("batch_"+teamName, provider.Name, provider.Model, "batch_prompt", latency, err)

		// Return default decisions
		results := make([]map[string]interface{}, len(observations))
		for i, obs := range observations {
			results[i] = DefaultDecision(obs)
		}
		return results, err
	}

	log.Printf("✅ Batch [%s] OK in %dms", teamName, latency)
	m.recordSuccess(provider.Name)
	audit.LogSuccess("batch_"+teamName, provider.Name, provider.Model, "batch_prompt", response, latency)

	return parseBatchResponse(response, observations)
}

// JudgeChallenge uses Gemini to evaluate challenge responses
func (m *Manager) JudgeChallenge(challenge, responses map[string]interface{}) (map[string]interface{}, error) {
	if m.activeBrain == nil {
		// Fallback to simple matching
		return simpleJudge(challenge, responses), nil
	}

	m.rateLimiter.Wait(1)
	m.throttle()

	prompt := promptBuilder.BuildJudgePrompt(challenge, responses)
	startTime := time.Now()

	var response string
	var err error

	if m.activeBrain.Name == "gemini" {
		response, err = m.callGeminiWithRetry(m.activeBrain, prompt, 2)
	} else {
		response, err = m.callProviderWithRetry(m.activeBrain, prompt, 2)
	}

	latency := time.Since(startTime).Milliseconds()

	if err != nil {
		log.Printf("❌ Judge [%s] FAILED: %s", m.activeBrain.Name, truncateError(err))
		m.recordError(m.activeBrain.Name, err)
		return simpleJudge(challenge, responses), err
	}

	log.Printf("✅ Judge [%s] OK in %dms", m.activeBrain.Name, latency)
	m.recordSuccess(m.activeBrain.Name)

	return parseJudgeResponse(response, challenge, responses)
}

// GetCommentary generates exciting play-by-play commentary
func (m *Manager) GetCommentary(events []map[string]interface{}, scores map[string]int) (string, error) {
	if m.activeBrain == nil {
		return "The game continues...", nil
	}

	m.rateLimiter.Wait(1)
	m.throttle()

	prompt := promptBuilder.BuildCommentaryPrompt(events, scores)

	var response string
	var err error

	if m.activeBrain.Name == "gemini" {
		response, err = m.callGeminiWithRetry(m.activeBrain, prompt, 1)
	} else {
		response, err = m.callProviderWithRetry(m.activeBrain, prompt, 1)
	}

	if err != nil {
		return "The game continues...", err
	}

	// Clean up response
	response = strings.TrimSpace(response)
	response = strings.Trim(response, "\"")

	return response, nil
}

// parseBatchResponse extracts individual decisions from a batch LLM response
func parseBatchResponse(response string, observations []map[string]interface{}) ([]map[string]interface{}, error) {
	// Try to find JSON in response
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start >= 0 && end > start {
		jsonStr := response[start : end+1]

		var parsed struct {
			Decisions []struct {
				NPC    string      `json:"npc"`
				Action string      `json:"action"`
				Target interface{} `json:"target"`
				Reason string      `json:"reason"`
			} `json:"decisions"`
			Strategy string `json:"strategy"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			results := make([]map[string]interface{}, len(observations))

			for i, obs := range observations {
				npcName := ""
				if name, ok := obs["name"].(string); ok {
					npcName = name
				}

				// Find matching decision
				found := false
				for _, dec := range parsed.Decisions {
					if dec.NPC == npcName {
						results[i] = map[string]interface{}{
							"npc_id": obs["npc_id"],
							"action": dec.Action,
							"target": dec.Target,
							"reason": dec.Reason,
						}
						found = true
						break
					}
				}

				if !found {
					results[i] = DefaultDecision(obs)
				}
			}

			return results, nil
		}
	}

	// Fallback to defaults
	results := make([]map[string]interface{}, len(observations))
	for i, obs := range observations {
		results[i] = DefaultDecision(obs)
	}
	return results, nil
}

// parseJudgeResponse extracts judgment from LLM response
func parseJudgeResponse(response string, challenge, responses map[string]interface{}) (map[string]interface{}, error) {
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start >= 0 && end > start {
		jsonStr := response[start : end+1]

		var parsed struct {
			Correct  bool    `json:"correct"`
			Feedback string  `json:"feedback"`
			Score    float64 `json:"score"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			return map[string]interface{}{
				"correct":  parsed.Correct,
				"feedback": parsed.Feedback,
				"score":    parsed.Score,
			}, nil
		}
	}

	// Fallback to simple judge
	return simpleJudge(challenge, responses), nil
}

// simpleJudge provides basic judgment without LLM
func simpleJudge(challenge, responses map[string]interface{}) map[string]interface{} {
	challengeType := ""
	if t, ok := challenge["type"].(string); ok {
		challengeType = t
	}

	switch challengeType {
	case "coordination":
		// All responses must match
		var firstResponse string
		allMatch := true
		for _, v := range responses {
			resp := fmt.Sprintf("%v", v)
			if firstResponse == "" {
				firstResponse = resp
			} else if resp != firstResponse {
				allMatch = false
				break
			}
		}
		return map[string]interface{}{
			"correct":  allMatch && firstResponse != "",
			"feedback": map[bool]string{true: "Perfect coordination!", false: "Different answers given"}[allMatch],
			"score":    map[bool]float64{true: 1.0, false: 0.0}[allMatch],
		}

	case "memory":
		// Check against solution
		solution := ""
		if s, ok := challenge["solution"].(string); ok {
			solution = s
		}
		for _, v := range responses {
			if fmt.Sprintf("%v", v) == solution {
				return map[string]interface{}{
					"correct":  true,
					"feedback": "Correct recall!",
					"score":    1.0,
				}
			}
		}
		return map[string]interface{}{
			"correct":  false,
			"feedback": "Incorrect answer",
			"score":    0.0,
		}
	}

	// Default: success
	return map[string]interface{}{
		"correct":  true,
		"feedback": "Challenge completed",
		"score":    0.5,
	}
}
