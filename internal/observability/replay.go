package observability

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Replay system for game state snapshots

// GameSnapshot represents a point-in-time game state for replay
type GameSnapshot struct {
	Timestamp time.Time              `json:"timestamp"`
	Tick      int                    `json:"tick"`
	State     map[string]interface{} `json:"state"`
}

// ReplayManager handles game state snapshots
type ReplayManager struct {
	snapshots        []GameSnapshot
	maxSnapshots     int
	snapshotInterval time.Duration
	lastSnapshotTime time.Time
	mu               sync.RWMutex
	enabled          bool
	filePath         string
}

// NewReplayManager creates a new replay manager
func NewReplayManager(enabled bool, filePath string) *ReplayManager {
	return &ReplayManager{
		snapshots:        make([]GameSnapshot, 0, 100),
		maxSnapshots:     100,
		snapshotInterval: 5 * time.Second,
		enabled:          enabled,
		filePath:         filePath,
	}
}

// ShouldSnapshot checks if it's time to create a new snapshot
func (rm *ReplayManager) ShouldSnapshot() bool {
	if !rm.enabled {
		return false
	}
	return time.Since(rm.lastSnapshotTime) >= rm.snapshotInterval
}

// CreateSnapshot saves the current game state
func (rm *ReplayManager) CreateSnapshot(tick int, state map[string]interface{}) {
	if !rm.enabled {
		return
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	snapshot := GameSnapshot{
		Timestamp: time.Now(),
		Tick:      tick,
		State:     state,
	}

	// Add to list
	rm.snapshots = append(rm.snapshots, snapshot)

	// Trim if too many
	if len(rm.snapshots) > rm.maxSnapshots {
		rm.snapshots = rm.snapshots[1:]
	}

	rm.lastSnapshotTime = time.Now()
}

// GetSnapshots returns all available snapshots
func (rm *ReplayManager) GetSnapshots() []GameSnapshot {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]GameSnapshot, len(rm.snapshots))
	copy(result, rm.snapshots)
	return result
}

// GetSnapshotAt returns the snapshot closest to the given timestamp
func (rm *ReplayManager) GetSnapshotAt(timestamp time.Time) *GameSnapshot {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if len(rm.snapshots) == 0 {
		return nil
	}

	// Find closest snapshot
	closest := &rm.snapshots[0]
	closestDiff := abs64(timestamp.UnixMilli() - closest.Timestamp.UnixMilli())

	for i := range rm.snapshots {
		diff := abs64(timestamp.UnixMilli() - rm.snapshots[i].Timestamp.UnixMilli())
		if diff < closestDiff {
			closest = &rm.snapshots[i]
			closestDiff = diff
		}
	}

	return closest
}

// GetSnapshotByTick returns the snapshot at or before the given tick
func (rm *ReplayManager) GetSnapshotByTick(tick int) *GameSnapshot {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if len(rm.snapshots) == 0 {
		return nil
	}

	// Find snapshot at or before tick
	var result *GameSnapshot
	for i := range rm.snapshots {
		if rm.snapshots[i].Tick <= tick {
			result = &rm.snapshots[i]
		} else {
			break
		}
	}

	return result
}

// SaveToFile writes all snapshots to a file
func (rm *ReplayManager) SaveToFile() error {
	if rm.filePath == "" {
		return nil
	}

	rm.mu.RLock()
	data, err := json.Marshal(rm.snapshots)
	rm.mu.RUnlock()

	if err != nil {
		return err
	}

	return os.WriteFile(rm.filePath, data, 0644)
}

// LoadFromFile reads snapshots from a file
func (rm *ReplayManager) LoadFromFile() error {
	if rm.filePath == "" {
		return nil
	}

	data, err := os.ReadFile(rm.filePath)
	if err != nil {
		return err
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	return json.Unmarshal(data, &rm.snapshots)
}

// GetTimeline returns a summary of available replay points
func (rm *ReplayManager) GetTimeline() []map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	timeline := make([]map[string]interface{}, len(rm.snapshots))
	for i, snap := range rm.snapshots {
		timeline[i] = map[string]interface{}{
			"tick":      snap.Tick,
			"timestamp": snap.Timestamp.Format(time.RFC3339),
			"index":     i,
		}
	}
	return timeline
}

// Clear removes all snapshots
func (rm *ReplayManager) Clear() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.snapshots = make([]GameSnapshot, 0, rm.maxSnapshots)
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
