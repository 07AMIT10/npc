package game

// Zone represents an area in the game world
type Zone struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Theme        string    `json:"theme"`
	Description  string    `json:"description"`
	Bounds       Rectangle `json:"bounds"`
	Unlocked     bool      `json:"unlocked"`
	ControlledBy string    `json:"controlled_by"` // Team ID or empty
	Rewards      int       `json:"rewards"`       // Token reward for unlocking
}

// Rectangle represents zone boundaries
type Rectangle struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Gate represents a barrier between zones that requires solving a challenge
type Gate struct {
	ID               string     `json:"id"`
	FromZone         string     `json:"from_zone"`
	ToZone           string     `json:"to_zone"`
	Position         [2]float64 `json:"position"`
	ChallengeID      string     `json:"challenge_id"`
	Unlocked         bool       `json:"unlocked"`
	UnlockedBy       string     `json:"unlocked_by"`       // Team or NPC that solved it
	RequiresTeamwork bool       `json:"requires_teamwork"` // Both teammates needed
}

// ZoneManager handles zone and gate operations
type ZoneManager struct {
	Zones map[string]*Zone `json:"zones"`
	Gates map[string]*Gate `json:"gates"`
}

// NewZoneManager creates a zone manager with default layout
func NewZoneManager(worldWidth, worldHeight int) *ZoneManager {
	zm := &ZoneManager{
		Zones: make(map[string]*Zone),
		Gates: make(map[string]*Gate),
	}

	halfW := float64(worldWidth) / 2
	halfH := float64(worldHeight) / 2

	// Create 4-zone layout
	// Zone 1: Start (unlocked for everyone)
	zm.Zones["start"] = &Zone{
		ID:          "start",
		Name:        "Starting Grounds",
		Theme:       "neutral",
		Description: "Where all explorers begin their journey",
		Bounds:      Rectangle{X: 0, Y: 0, Width: halfW, Height: halfH},
		Unlocked:    true,
		Rewards:     0,
	}

	// Zone 2: Eastern Challenge
	zm.Zones["zone_2"] = &Zone{
		ID:          "zone_2",
		Name:        "Crystal Caverns",
		Theme:       "crystal",
		Description: "Glittering caves with hidden treasures",
		Bounds:      Rectangle{X: halfW, Y: 0, Width: halfW, Height: halfH},
		Unlocked:    false,
		Rewards:     30,
	}

	// Zone 3: Southern Challenge
	zm.Zones["zone_3"] = &Zone{
		ID:          "zone_3",
		Name:        "Whispering Woods",
		Theme:       "forest",
		Description: "Ancient forest requiring teamwork to traverse",
		Bounds:      Rectangle{X: 0, Y: halfH, Width: halfW, Height: halfH},
		Unlocked:    false,
		Rewards:     40,
	}

	// Zone 4: Final Challenge
	zm.Zones["zone_4"] = &Zone{
		ID:          "zone_4",
		Name:        "The Nexus",
		Theme:       "void",
		Description: "The ultimate destination with the greatest rewards",
		Bounds:      Rectangle{X: halfW, Y: halfH, Width: halfW, Height: halfH},
		Unlocked:    false,
		Rewards:     50,
	}

	// Create gates between zones
	zm.Gates["gate_1_2"] = &Gate{
		ID:               "gate_1_2",
		FromZone:         "start",
		ToZone:           "zone_2",
		Position:         [2]float64{halfW, halfH / 2},
		ChallengeID:      "challenge_coordination",
		Unlocked:         false,
		RequiresTeamwork: false,
	}

	zm.Gates["gate_1_3"] = &Gate{
		ID:               "gate_1_3",
		FromZone:         "start",
		ToZone:           "zone_3",
		Position:         [2]float64{halfW / 2, halfH},
		ChallengeID:      "challenge_teamwork",
		Unlocked:         false,
		RequiresTeamwork: true, // Both teammates needed!
	}

	zm.Gates["gate_2_4"] = &Gate{
		ID:               "gate_2_4",
		FromZone:         "zone_2",
		ToZone:           "zone_4",
		Position:         [2]float64{halfW + halfW/2, halfH},
		ChallengeID:      "challenge_memory",
		Unlocked:         false,
		RequiresTeamwork: false,
	}

	zm.Gates["gate_3_4"] = &Gate{
		ID:               "gate_3_4",
		FromZone:         "zone_3",
		ToZone:           "zone_4",
		Position:         [2]float64{halfW, halfH + halfH/2},
		ChallengeID:      "challenge_spatial",
		Unlocked:         false,
		RequiresTeamwork: true,
	}

	return zm
}

// GetZoneAt returns the zone at the given position
func (zm *ZoneManager) GetZoneAt(x, y float64) *Zone {
	for _, zone := range zm.Zones {
		if zm.IsInZone(x, y, zone) {
			return zone
		}
	}
	return nil
}

// IsInZone checks if a position is within a zone
func (zm *ZoneManager) IsInZone(x, y float64, zone *Zone) bool {
	return x >= zone.Bounds.X &&
		x <= zone.Bounds.X+zone.Bounds.Width &&
		y >= zone.Bounds.Y &&
		y <= zone.Bounds.Y+zone.Bounds.Height
}

// GetNearbyGates returns gates within range of a position
func (zm *ZoneManager) GetNearbyGates(x, y, range_ float64) []*Gate {
	var nearby []*Gate
	for _, gate := range zm.Gates {
		dx := gate.Position[0] - x
		dy := gate.Position[1] - y
		dist := dx*dx + dy*dy
		if dist <= range_*range_ {
			nearby = append(nearby, gate)
		}
	}
	return nearby
}

// UnlockGate marks a gate as unlocked and the destination zone as accessible
func (zm *ZoneManager) UnlockGate(gateID, unlockedBy string) bool {
	gate, ok := zm.Gates[gateID]
	if !ok || gate.Unlocked {
		return false
	}

	gate.Unlocked = true
	gate.UnlockedBy = unlockedBy

	// Unlock the destination zone
	if zone, ok := zm.Zones[gate.ToZone]; ok {
		zone.Unlocked = true
	}

	return true
}

// CanAccessZone checks if a team can enter a zone
func (zm *ZoneManager) CanAccessZone(zoneID, teamID string) bool {
	zone, ok := zm.Zones[zoneID]
	if !ok {
		return false
	}

	// Start zone is always accessible
	if zone.ID == "start" {
		return true
	}

	// Check if any gate leading to this zone is unlocked
	for _, gate := range zm.Gates {
		if gate.ToZone == zoneID && gate.Unlocked {
			return true
		}
	}

	return false
}

// GetGateForChallenge finds the gate associated with a challenge
func (zm *ZoneManager) GetGateForChallenge(challengeID string) *Gate {
	for _, gate := range zm.Gates {
		if gate.ChallengeID == challengeID {
			return gate
		}
	}
	return nil
}
