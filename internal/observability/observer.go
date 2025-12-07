package observability

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// TraceEntry records a single LLM API call
type TraceEntry struct {
	Timestamp time.Time `json:"ts"`
	TraceID   string    `json:"id"`
	Role      string    `json:"role"` // movement, challenge, judge, etc.
	NPC       string    `json:"npc,omitempty"`
	Team      string    `json:"team,omitempty"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	Prompt    string    `json:"prompt"`
	Response  string    `json:"response"`
	LatencyMs int64     `json:"latency_ms"`
	TokensIn  int       `json:"tokens_in,omitempty"`
	TokensOut int       `json:"tokens_out,omitempty"`
	CostUSD   float64   `json:"cost_usd,omitempty"`
	Error     string    `json:"error,omitempty"`
	Success   bool      `json:"success"`
}

// AuditEntry records a game event
type AuditEntry struct {
	Timestamp time.Time              `json:"ts"`
	Event     string                 `json:"event"`
	NPC       string                 `json:"npc,omitempty"`
	Team      string                 `json:"team,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// Observer handles all observability operations
type Observer struct {
	traceFile  *os.File
	auditFile  *os.File
	mu         sync.Mutex
	enabled    bool
	traceCount int

	// Stats
	TotalCalls   int     `json:"total_calls"`
	TotalLatency int64   `json:"total_latency_ms"`
	TotalCost    float64 `json:"total_cost_usd"`
	ErrorCount   int     `json:"error_count"`

	// Recent entries for quick access
	recentTraces []TraceEntry
	recentAudits []AuditEntry
	maxRecent    int
}

// Config for observer
type ObserverConfig struct {
	Enabled        bool
	TracePath      string
	AuditPath      string
	IncludePrompts bool
}

var (
	globalObserver *Observer
	observerOnce   sync.Once
)

// GetObserver returns the singleton observer instance
func GetObserver() *Observer {
	observerOnce.Do(func() {
		globalObserver = &Observer{
			enabled:      true,
			maxRecent:    100,
			recentTraces: make([]TraceEntry, 0, 100),
			recentAudits: make([]AuditEntry, 0, 100),
		}
	})
	return globalObserver
}

// Initialize sets up file outputs for the observer
func (o *Observer) Initialize(cfg ObserverConfig) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.enabled = cfg.Enabled
	if !o.enabled {
		return nil
	}

	if cfg.TracePath != "" {
		f, err := os.OpenFile(cfg.TracePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open trace file: %w", err)
		}
		o.traceFile = f
	}

	if cfg.AuditPath != "" {
		f, err := os.OpenFile(cfg.AuditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open audit file: %w", err)
		}
		o.auditFile = f
	}

	return nil
}

// TraceCall records an LLM API call
func (o *Observer) TraceCall(entry TraceEntry) {
	if !o.enabled {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	// Generate trace ID
	o.traceCount++
	entry.TraceID = fmt.Sprintf("trace_%06d", o.traceCount)
	entry.Timestamp = time.Now()

	// Update stats
	o.TotalCalls++
	o.TotalLatency += entry.LatencyMs
	o.TotalCost += entry.CostUSD
	if entry.Error != "" || !entry.Success {
		o.ErrorCount++
	}

	// Store in recent
	if len(o.recentTraces) >= o.maxRecent {
		o.recentTraces = o.recentTraces[1:]
	}
	o.recentTraces = append(o.recentTraces, entry)

	// Write to file
	if o.traceFile != nil {
		data, _ := json.Marshal(entry)
		o.traceFile.Write(append(data, '\n'))
	}
}

// Audit records a game event
func (o *Observer) Audit(event, npc, team string, data map[string]interface{}) {
	if !o.enabled {
		return
	}

	entry := AuditEntry{
		Timestamp: time.Now(),
		Event:     event,
		NPC:       npc,
		Team:      team,
		Data:      data,
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	// Store in recent
	if len(o.recentAudits) >= o.maxRecent {
		o.recentAudits = o.recentAudits[1:]
	}
	o.recentAudits = append(o.recentAudits, entry)

	// Write to file
	if o.auditFile != nil {
		data, _ := json.Marshal(entry)
		o.auditFile.Write(append(data, '\n'))
	}
}

// GetStats returns current statistics
func (o *Observer) GetStats() map[string]interface{} {
	o.mu.Lock()
	defer o.mu.Unlock()

	avgLatency := float64(0)
	if o.TotalCalls > 0 {
		avgLatency = float64(o.TotalLatency) / float64(o.TotalCalls)
	}

	errorRate := float64(0)
	if o.TotalCalls > 0 {
		errorRate = float64(o.ErrorCount) / float64(o.TotalCalls) * 100
	}

	return map[string]interface{}{
		"total_calls":    o.TotalCalls,
		"avg_latency_ms": avgLatency,
		"total_cost_usd": o.TotalCost,
		"error_count":    o.ErrorCount,
		"error_rate_pct": errorRate,
	}
}

// GetRecentTraces returns the most recent trace entries
func (o *Observer) GetRecentTraces(limit int) []TraceEntry {
	o.mu.Lock()
	defer o.mu.Unlock()

	if limit > len(o.recentTraces) {
		limit = len(o.recentTraces)
	}
	start := len(o.recentTraces) - limit
	return o.recentTraces[start:]
}

// GetRecentAudits returns the most recent audit entries
func (o *Observer) GetRecentAudits(limit int) []AuditEntry {
	o.mu.Lock()
	defer o.mu.Unlock()

	if limit > len(o.recentAudits) {
		limit = len(o.recentAudits)
	}
	start := len(o.recentAudits) - limit
	return o.recentAudits[start:]
}

// Close closes the observer's file handles
func (o *Observer) Close() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.traceFile != nil {
		o.traceFile.Close()
	}
	if o.auditFile != nil {
		o.auditFile.Close()
	}
}

// Convenience functions for common audit events

func (o *Observer) AuditNPCMove(npc, team string, from, to [2]float64) {
	o.Audit("npc_move", npc, team, map[string]interface{}{
		"from": from,
		"to":   to,
	})
}

func (o *Observer) AuditChallengeStart(npc, team, gateID, challengeType string) {
	o.Audit("challenge_start", npc, team, map[string]interface{}{
		"gate_id":        gateID,
		"challenge_type": challengeType,
	})
}

func (o *Observer) AuditChallengeComplete(npc, team, gateID string, success bool, tokensEarned int) {
	o.Audit("challenge_complete", npc, team, map[string]interface{}{
		"gate_id":       gateID,
		"success":       success,
		"tokens_earned": tokensEarned,
	})
}

func (o *Observer) AuditZoneUnlock(team, zoneID, unlockedBy string) {
	o.Audit("zone_unlocked", unlockedBy, team, map[string]interface{}{
		"zone_id": zoneID,
	})
}

func (o *Observer) AuditTeamMessage(fromNPC, team, message string) {
	o.Audit("team_message", fromNPC, team, map[string]interface{}{
		"message": message,
	})
}
