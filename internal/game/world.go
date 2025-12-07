package game

import (
	"github.com/amit/npc/internal/challenge"
	"github.com/amit/npc/internal/config"
)

// World represents the game world state with teams and zones
type World struct {
	Width   int            `json:"width"`
	Height  int            `json:"height"`
	NPCs    []*NPC         `json:"npcs"`
	Objects []*WorldObject `json:"objects"`
	Tick    int            `json:"tick"`

	// New v2 systems
	Teams      *TeamManager                `json:"teams"`
	Zones      *ZoneManager                `json:"zones"`
	Challenges *challenge.ChallengeManager `json:"challenges"`
}

// NPC represents a non-player character
type NPC struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Pos       [2]float64 `json:"pos"`
	HP        int        `json:"hp"`
	Energy    int        `json:"energy"`
	State     string     `json:"state"`
	Inventory []string   `json:"inventory"`

	// New v2 fields
	Team        string    `json:"team"`         // Team ID
	CurrentZone string    `json:"current_zone"` // Zone ID
	MemoryCode  string    `json:"memory_code"`  // For memory challenges
	Messages    []Message `json:"messages"`     // Recent messages from teammate
}

// Message represents a chat message between NPCs
type Message struct {
	From    string `json:"from"`
	Content string `json:"content"`
	Time    int    `json:"time"` // Game tick when sent
}

// WorldObject represents an interactive object in the world
type WorldObject struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Pos       [2]float64 `json:"pos"`
	VisitedBy []string   `json:"visited_by"`
}

// NewWorld creates a new game world with v2 features
func NewWorld(cfg *config.Config) *World {
	world := &World{
		Width:      cfg.Game.WorldWidth,
		Height:     cfg.Game.WorldHeight,
		NPCs:       make([]*NPC, 0, cfg.NPCs.Count),
		Teams:      NewTeamManager(),
		Zones:      NewZoneManager(cfg.Game.WorldWidth, cfg.Game.WorldHeight),
		Challenges: challenge.NewChallengeManager(),
	}

	// Create NPCs in team positions
	// Team Red (Explorer, Scout) starts top-left
	// Team Blue (Wanderer, Seeker) starts bottom-right
	teamPositions := map[string][][2]float64{
		"red": {
			{150, 150}, // Explorer
			{250, 150}, // Scout (nearby)
		},
		"blue": {
			{float64(world.Width) - 150, float64(world.Height) - 150}, // Wanderer
			{float64(world.Width) - 250, float64(world.Height) - 150}, // Seeker (nearby)
		},
	}

	// Memory codes for memory challenges
	memoryCodes := []string{"A749", "B312", "C856", "D427"}

	npcIndex := 0
	for teamID, team := range world.Teams.Teams {
		positions := teamPositions[teamID]
		for i, npcName := range team.Members {
			if npcIndex >= cfg.NPCs.Count {
				break
			}

			pos := positions[i%len(positions)]
			npc := &NPC{
				ID:          "npc_" + string(rune('0'+npcIndex)),
				Name:        npcName,
				Pos:         pos,
				HP:          100,
				Energy:      100,
				State:       "idle",
				Inventory:   []string{},
				Team:        teamID,
				CurrentZone: "start",
				MemoryCode:  memoryCodes[npcIndex%len(memoryCodes)],
				Messages:    []Message{},
			}
			world.NPCs = append(world.NPCs, npc)
			npcIndex++
		}
	}

	// Create world objects (treasures, landmarks)
	objectTypes := []string{"treasure", "landmark", "resource", "mystery"}
	for i := 0; i < 12; i++ {
		obj := &WorldObject{
			ID:        "obj_" + string(rune('0'+i)),
			Type:      objectTypes[i%len(objectTypes)],
			Pos:       [2]float64{100 + float64(i*90), 100 + float64((i%4)*150)},
			VisitedBy: []string{},
		}
		world.Objects = append(world.Objects, obj)
	}

	return world
}

// GetNPCByName returns an NPC by name
func (w *World) GetNPCByName(name string) *NPC {
	for _, npc := range w.NPCs {
		if npc.Name == name {
			return npc
		}
	}
	return nil
}

// GetNPCByID returns an NPC by ID
func (w *World) GetNPCByID(id string) *NPC {
	for _, npc := range w.NPCs {
		if npc.ID == id {
			return npc
		}
	}
	return nil
}

// UpdateNPCZone updates which zone an NPC is in based on position
func (w *World) UpdateNPCZone(npc *NPC) {
	zone := w.Zones.GetZoneAt(npc.Pos[0], npc.Pos[1])
	if zone != nil {
		npc.CurrentZone = zone.ID
	}
}

// SendMessage sends a message from one NPC to another (teammate)
func (w *World) SendMessage(fromNPC, toNPC, content string) {
	to := w.GetNPCByName(toNPC)
	if to == nil {
		return
	}

	msg := Message{
		From:    fromNPC,
		Content: content,
		Time:    w.Tick,
	}
	to.Messages = append(to.Messages, msg)

	// Keep only last 5 messages
	if len(to.Messages) > 5 {
		to.Messages = to.Messages[len(to.Messages)-5:]
	}
}

// GetNearbyGatesForNPC returns gates near the NPC
func (w *World) GetNearbyGatesForNPC(npc *NPC, range_ float64) []*Gate {
	return w.Zones.GetNearbyGates(npc.Pos[0], npc.Pos[1], range_)
}

// GetGameState returns the current game state for broadcasting
func (w *World) GetGameState() map[string]interface{} {
	return map[string]interface{}{
		"tick":              w.Tick,
		"teams":             w.Teams.GetLeaderboard(),
		"zones":             w.Zones.Zones,
		"gates":             w.Zones.Gates,
		"npcs":              w.NPCs,
		"active_challenges": w.Challenges.ActiveChallenges,
	}
}

// GetTeamScores returns current team scores
func (w *World) GetTeamScores() map[string]int {
	scores := make(map[string]int)
	for id, team := range w.Teams.Teams {
		scores[id] = team.Score
	}
	return scores
}
