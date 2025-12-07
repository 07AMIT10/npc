package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// BatchDecisionSystem handles multi-NPC decisions in a single LLM call
// with caching, fallback mechanisms, and auto-configuration
type BatchDecisionSystem struct {
	manager       *Manager
	cache         *DecisionCache
	promptBuilder *PromptBuilder
	mu            sync.RWMutex

	// Statistics
	batchCalls     int
	cachHits       int
	fallbackUsed   int
	totalDecisions int
}

// DecisionCache stores recent decisions to avoid redundant API calls
type DecisionCache struct {
	mu      sync.RWMutex
	entries map[string]*CachedDecision
	maxSize int
	ttl     time.Duration
}

// CachedDecision represents a cached NPC decision
type CachedDecision struct {
	Decision  map[string]interface{}
	CreatedAt time.Time
	HitCount  int
}

// NewBatchDecisionSystem creates a new batch decision system
func NewBatchDecisionSystem(manager *Manager) *BatchDecisionSystem {
	return &BatchDecisionSystem{
		manager:       manager,
		cache:         NewDecisionCache(100, 10*time.Second),
		promptBuilder: &PromptBuilder{},
	}
}

// NewDecisionCache creates a cache for NPC decisions
func NewDecisionCache(maxSize int, ttl time.Duration) *DecisionCache {
	return &DecisionCache{
		entries: make(map[string]*CachedDecision),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// BatchDecisionRequest represents a request for multiple NPC decisions
type BatchDecisionRequest struct {
	Observations []map[string]interface{}
}

// BatchDecisionResponse contains decisions for all NPCs
type BatchDecisionResponse struct {
	Decisions []map[string]interface{} `json:"decisions"`
	Strategy  string                   `json:"strategy,omitempty"`
	FromCache []bool                   // Which decisions came from cache
	Error     error
}

// GetBatchDecisions gets decisions for ALL NPCs in a single optimized call
// Auto-configures prompt based on number of NPCs - no manual changes needed!
func (bds *BatchDecisionSystem) GetBatchDecisions(ctx context.Context, observations []map[string]interface{}) *BatchDecisionResponse {
	if len(observations) == 0 {
		return &BatchDecisionResponse{Error: fmt.Errorf("no observations provided")}
	}

	bds.mu.Lock()
	bds.totalDecisions += len(observations)
	bds.mu.Unlock()

	// Check context before proceeding
	if ctx.Err() != nil {
		return &BatchDecisionResponse{Error: ctx.Err()}
	}

	// Phase 1: Check cache for each NPC
	response := &BatchDecisionResponse{
		Decisions: make([]map[string]interface{}, len(observations)),
		FromCache: make([]bool, len(observations)),
	}

	var uncachedIndices []int
	var uncachedObs []map[string]interface{}

	for i, obs := range observations {
		hash := bds.hashObservation(obs)
		if cached, ok := bds.cache.Get(hash); ok {
			response.Decisions[i] = cached.Decision
			response.FromCache[i] = true
			bds.mu.Lock()
			bds.cachHits++
			bds.mu.Unlock()
			log.Printf("📦 Cache hit for %s", getString(obs, "name"))
		} else {
			uncachedIndices = append(uncachedIndices, i)
			uncachedObs = append(uncachedObs, obs)
		}
	}

	// If all cached, return immediately
	if len(uncachedObs) == 0 {
		return response
	}

	// Check context again before API call
	if ctx.Err() != nil {
		return &BatchDecisionResponse{Error: ctx.Err()}
	}

	// Phase 2: Build dynamic prompt for uncached NPCs
	prompt := bds.buildFlexibleMultiNPCPrompt(uncachedObs)

	// Phase 3: Call LLM with timeout context
	callCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	llmResponse, err := bds.callLLMWithFallback(callCtx, prompt, len(uncachedObs))
	if err != nil {
		// Fallback: Generate default decisions
		log.Printf("⚠️ Batch LLM failed, using fallback: %v", err)
		bds.mu.Lock()
		bds.fallbackUsed++
		bds.mu.Unlock()

		for _, idx := range uncachedIndices {
			response.Decisions[idx] = DefaultDecision(observations[idx])
		}
		return response
	}

	bds.mu.Lock()
	bds.batchCalls++
	bds.mu.Unlock()

	// Phase 4: Parse and distribute decisions
	decisions := bds.parseMultiNPCResponse(llmResponse, uncachedObs)

	for i, idx := range uncachedIndices {
		if i < len(decisions) {
			response.Decisions[idx] = decisions[i]
			// Cache this decision
			hash := bds.hashObservation(observations[idx])
			bds.cache.Set(hash, decisions[i])
		} else {
			// Not enough decisions returned, use fallback
			response.Decisions[idx] = DefaultDecision(observations[idx])
		}
	}

	return response
}

// buildFlexibleMultiNPCPrompt creates a prompt that auto-configures based on NPC count
// This is the KEY function that makes adding NPCs automatic!
func (bds *BatchDecisionSystem) buildFlexibleMultiNPCPrompt(observations []map[string]interface{}) string {
	var sb strings.Builder

	// Dynamic header based on NPC count
	sb.WriteString(fmt.Sprintf(`You control %d NPCs in a competitive arena game. Make optimal decisions for ALL of them.

## GAME RULES
- Teams compete to unlock gates and score points
- Some gates require 2-player teamwork
- NPCs can challenge gates when within 60 units
- Social actions (talk/taunt) for when opponents are near

`, len(observations)))

	// Auto-generate NPC sections
	sb.WriteString("## YOUR NPCs\n\n")

	for i, obs := range observations {
		name := getString(obs, "name")
		team := getString(obs, "team")
		pos := getArray(obs, "pos")
		energy := getInt(obs, "energy")
		state := getString(obs, "state")

		// Safely get position values
		posX, posY := 0.0, 0.0
		if len(pos) >= 2 {
			if v, ok := pos[0].(float64); ok {
				posX = v
			}
			if v, ok := pos[1].(float64); ok {
				posY = v
			}
		}

		sb.WriteString(fmt.Sprintf("### NPC %d: %s\n", i+1, name))
		sb.WriteString(fmt.Sprintf("- Team: %s | Pos: (%.0f, %.0f) | Energy: %d%% | State: %s\n",
			team, posX, posY, energy, state))

		// Nearby gates
		nearbyGates := getArrayOfMaps(obs, "nearby_gates")
		if len(nearbyGates) > 0 {
			var gateInfo []string
			for _, g := range nearbyGates {
				if !getBool(g, "unlocked") {
					gateID := getString(g, "id")
					dist := getFloat(g, "distance")
					tw := ""
					if getBool(g, "requiresTeamwork") {
						tw = " [2P]"
					}
					gateInfo = append(gateInfo, fmt.Sprintf("%s:%.0fu%s", gateID, dist, tw))
				}
			}
			if len(gateInfo) > 0 {
				sb.WriteString(fmt.Sprintf("- Gates: %s\n", strings.Join(gateInfo, ", ")))
			}
		}

		// Nearby NPCs
		nearbyNPCs := getArrayOfMaps(obs, "nearby_npcs")
		if len(nearbyNPCs) > 0 {
			var npcInfo []string
			for _, n := range nearbyNPCs {
				npcName := getString(n, "name")
				dist := getFloat(n, "distance")
				isTeammate := getBool(n, "isTeammate")
				marker := "⚔️"
				if isTeammate {
					marker = "👥"
				}
				npcInfo = append(npcInfo, fmt.Sprintf("%s%s:%.0fu", marker, npcName, dist))
			}
			sb.WriteString(fmt.Sprintf("- Nearby: %s\n", strings.Join(npcInfo, ", ")))
		}

		sb.WriteString("\n")
	}

	// Actions section
	sb.WriteString(`## AVAILABLE ACTIONS
- move: {"action":"move","target":[x,y],"reason":"..."} - Move to coordinates
- challenge: {"action":"challenge","target":"gate_id","reason":"..."} - Attempt gate (must be within 60 units!)
- talk: {"action":"talk","target":"NPC_name","message":"..."} - Talk to nearby NPC
- taunt: {"action":"taunt","target":"NPC_name","message":"..."} - Taunt opponent
- wait: {"action":"wait","target":null,"reason":"..."} - Stay and wait
- explore: {"action":"explore","target":null,"reason":"..."} - Random exploration

## STRATEGY TIPS
- Prioritize gates that are close (< 150 units)
- If 2 teammates near a [2P] gate, coordinate!
- Taunt opponents when you're winning
- Don't waste moves on already-unlocked gates

`)

	// Dynamic output format based on NPC count
	sb.WriteString("## RESPOND WITH JSON ONLY\n")
	sb.WriteString("```json\n{\n  \"decisions\": [\n")

	for i, obs := range observations {
		name := getString(obs, "name")
		npcID := getString(obs, "npc_id")
		comma := ","
		if i == len(observations)-1 {
			comma = ""
		}
		sb.WriteString(fmt.Sprintf(`    {"npc_id":"%s","npc":"%s","action":"...","target":...,"reason":"..."}%s
`, npcID, name, comma))
	}

	sb.WriteString(`  ],
  "strategy": "Brief team strategy (optional)"
}
` + "```")

	return sb.String()
}

// callLLMWithFallback tries primary provider, then falls back to others
func (bds *BatchDecisionSystem) callLLMWithFallback(ctx context.Context, prompt string, expectedCount int) (string, error) {
	// Try primary SLM provider
	if bds.manager.activeSLM != nil {
		response, err := bds.callWithContext(ctx, bds.manager.activeSLM, prompt)
		if err == nil {
			return response, nil
		}
		log.Printf("⚠️ Primary provider failed: %v", err)
	}

	// Try fallback providers
	for i := range bds.manager.slmProviders {
		p := &bds.manager.slmProviders[i]
		if p.Name == bds.manager.activeSLM.Name {
			continue // Skip already-tried primary
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		response, err := bds.callWithContext(ctx, p, prompt)
		if err == nil {
			log.Printf("✅ Fallback to %s successful", p.Name)
			return response, nil
		}
		log.Printf("⚠️ Fallback %s failed: %v", p.Name, err)
	}

	return "", fmt.Errorf("all providers failed")
}

// callWithContext wraps the API call with context cancellation
func (bds *BatchDecisionSystem) callWithContext(ctx context.Context, p *Provider, prompt string) (string, error) {
	resultChan := make(chan struct {
		response string
		err      error
	}, 1)

	go func() {
		resp, err := bds.manager.callProviderWithRetry(p, prompt, 2)
		resultChan <- struct {
			response string
			err      error
		}{resp, err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-resultChan:
		return result.response, result.err
	}
}

// parseMultiNPCResponse extracts individual decisions from batch response
func (bds *BatchDecisionSystem) parseMultiNPCResponse(response string, observations []map[string]interface{}) []map[string]interface{} {
	// Try to find JSON in response
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start < 0 || end < start {
		log.Printf("⚠️ No JSON found in batch response")
		return bds.generateDefaultDecisions(observations)
	}

	jsonStr := response[start : end+1]

	var parsed struct {
		Decisions []map[string]interface{} `json:"decisions"`
		Strategy  string                   `json:"strategy"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		log.Printf("⚠️ Failed to parse batch JSON: %v", err)
		return bds.generateDefaultDecisions(observations)
	}

	// Map decisions back to NPCs by name or npc_id
	result := make([]map[string]interface{}, len(observations))

	for i, obs := range observations {
		npcID := getString(obs, "npc_id")
		npcName := getString(obs, "name")

		// Find matching decision
		found := false
		for _, dec := range parsed.Decisions {
			decNpcID := getString(dec, "npc_id")
			decNpcName := getString(dec, "npc")

			if decNpcID == npcID || decNpcName == npcName {
				dec["npc_id"] = npcID // Ensure npc_id is set
				result[i] = dec
				found = true
				break
			}
		}

		if !found {
			log.Printf("⚠️ No decision found for %s, using default", npcName)
			result[i] = DefaultDecision(obs)
		}
	}

	return result
}

// generateDefaultDecisions creates fallback decisions for all NPCs
func (bds *BatchDecisionSystem) generateDefaultDecisions(observations []map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, len(observations))
	for i, obs := range observations {
		result[i] = DefaultDecision(obs)
	}
	return result
}

// hashObservation creates a cache key from observation
// Only includes position (rounded) and nearby gates - things that actually matter for decisions
func (bds *BatchDecisionSystem) hashObservation(obs map[string]interface{}) string {
	key := make(map[string]interface{})

	// Round position to grid (reduces cache variations)
	pos := getArray(obs, "pos")
	if len(pos) >= 2 {
		if x, ok := pos[0].(float64); ok {
			key["x"] = int(x/50) * 50 // Round to 50-unit grid
		}
		if y, ok := pos[1].(float64); ok {
			key["y"] = int(y/50) * 50
		}
	}

	key["name"] = getString(obs, "name")

	// Include only locked nearby gates
	nearbyGates := getArrayOfMaps(obs, "nearby_gates")
	var gateKeys []string
	for _, g := range nearbyGates {
		if !getBool(g, "unlocked") {
			gateID := getString(g, "id")
			dist := int(getFloat(g, "distance")/50) * 50 // Round distance
			gateKeys = append(gateKeys, fmt.Sprintf("%s:%d", gateID, dist))
		}
	}
	sort.Strings(gateKeys)
	key["gates"] = strings.Join(gateKeys, ",")

	// Hash the key
	keyJSON, _ := json.Marshal(key)
	hash := sha256.Sum256(keyJSON)
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes
}

// Cache methods

func (c *DecisionCache) Get(key string) (*CachedDecision, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// Check TTL
	if time.Since(entry.CreatedAt) > c.ttl {
		return nil, false
	}

	entry.HitCount++
	return entry, true
}

func (c *DecisionCache) Set(key string, decision map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict old entries if at max size
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = &CachedDecision{
		Decision:  decision,
		CreatedAt: time.Now(),
		HitCount:  0,
	}
}

func (c *DecisionCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// GetStats returns batch system statistics
func (bds *BatchDecisionSystem) GetStats() map[string]interface{} {
	bds.mu.RLock()
	defer bds.mu.RUnlock()

	cacheHitRate := 0.0
	if bds.totalDecisions > 0 {
		cacheHitRate = float64(bds.cachHits) / float64(bds.totalDecisions) * 100
	}

	return map[string]interface{}{
		"batch_calls":     bds.batchCalls,
		"cache_hits":      bds.cachHits,
		"total_decisions": bds.totalDecisions,
		"cache_hit_rate":  fmt.Sprintf("%.1f%%", cacheHitRate),
		"fallback_used":   bds.fallbackUsed,
		"cost_savings":    fmt.Sprintf("%.0f%%", (1-float64(bds.batchCalls)/float64(max(1, bds.totalDecisions)))*100),
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
