package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEntry represents a single API call audit log entry
type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	NPC       string `json:"npc"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Prompt    string `json:"prompt"`
	Response  string `json:"response"`
	LatencyMs int64  `json:"latency_ms"`
	Status    string `json:"status"` // "success" or "error"
	Error     string `json:"error,omitempty"`
}

// AuditLog manages API call audit logging
type AuditLog struct {
	entries    []AuditEntry
	maxEntries int
	logFile    string
	mu         sync.Mutex
}

var globalAuditLog *AuditLog

// InitAuditLog initializes the global audit log
func InitAuditLog() *AuditLog {
	// Ensure logs directory exists
	logsDir := "logs"
	os.MkdirAll(logsDir, 0755)

	globalAuditLog = &AuditLog{
		entries:    make([]AuditEntry, 0),
		maxEntries: 100, // Keep last 100 entries in memory
		logFile:    filepath.Join(logsDir, "audit.log"),
	}
	return globalAuditLog
}

// GetAuditLog returns the global audit log
func GetAuditLog() *AuditLog {
	if globalAuditLog == nil {
		return InitAuditLog()
	}
	return globalAuditLog
}

// Log adds an entry to the audit log
func (a *AuditLog) Log(entry AuditEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Set timestamp if not set
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().Format("2006-01-02 15:04:05.000")
	}

	// Truncate long prompts/responses for memory storage
	entry.Prompt = truncateStr(entry.Prompt, 200)
	entry.Response = truncateStr(entry.Response, 200)

	// Add to in-memory buffer
	a.entries = append(a.entries, entry)
	if len(a.entries) > a.maxEntries {
		a.entries = a.entries[1:]
	}

	// Also write to file (full entry)
	a.writeToFile(entry)
}

// LogSuccess logs a successful API call
func (a *AuditLog) LogSuccess(npc, provider, model, prompt, response string, latencyMs int64) {
	a.Log(AuditEntry{
		NPC:       npc,
		Provider:  provider,
		Model:     model,
		Prompt:    prompt,
		Response:  response,
		LatencyMs: latencyMs,
		Status:    "success",
	})
}

// LogError logs a failed API call
func (a *AuditLog) LogError(npc, provider, model, prompt string, latencyMs int64, err error) {
	a.Log(AuditEntry{
		NPC:       npc,
		Provider:  provider,
		Model:     model,
		Prompt:    prompt,
		LatencyMs: latencyMs,
		Status:    "error",
		Error:     err.Error(),
	})
}

// GetEntries returns recent audit entries
func (a *AuditLog) GetEntries(limit int) []AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()

	if limit <= 0 || limit > len(a.entries) {
		limit = len(a.entries)
	}

	// Return most recent entries (reversed order)
	result := make([]AuditEntry, limit)
	for i := 0; i < limit; i++ {
		result[i] = a.entries[len(a.entries)-1-i]
	}
	return result
}

// GetStats returns summary statistics
func (a *AuditLog) GetStats() map[string]interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()

	successByProvider := make(map[string]int)
	errorByProvider := make(map[string]int)
	latencyByProvider := make(map[string][]int64)

	for _, e := range a.entries {
		if e.Status == "success" {
			successByProvider[e.Provider]++
			latencyByProvider[e.Provider] = append(latencyByProvider[e.Provider], e.LatencyMs)
		} else {
			errorByProvider[e.Provider]++
		}
	}

	// Calculate average latencies
	avgLatency := make(map[string]int64)
	for provider, latencies := range latencyByProvider {
		if len(latencies) > 0 {
			var sum int64
			for _, l := range latencies {
				sum += l
			}
			avgLatency[provider] = sum / int64(len(latencies))
		}
	}

	return map[string]interface{}{
		"total_entries":       len(a.entries),
		"success_by_provider": successByProvider,
		"error_by_provider":   errorByProvider,
		"avg_latency_ms":      avgLatency,
	}
}

// writeToFile appends an entry to the log file
func (a *AuditLog) writeToFile(entry AuditEntry) {
	f, err := os.OpenFile(a.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	// Write as JSON line
	jsonData, _ := json.Marshal(entry)
	f.WriteString(string(jsonData) + "\n")
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// FormatEntry formats an entry for display
func (e AuditEntry) FormatEntry() string {
	status := "✅"
	if e.Status == "error" {
		status = "❌"
	}

	msg := fmt.Sprintf("%s %s | %s | %s (%s) | %dms",
		e.Timestamp, status, e.NPC, e.Provider, e.Model, e.LatencyMs)

	if e.Status == "error" {
		msg += fmt.Sprintf("\n   Error: %s", truncateStr(e.Error, 100))
	} else if e.Response != "" {
		msg += fmt.Sprintf("\n   Response: %s", truncateStr(e.Response, 80))
	}

	return msg
}
