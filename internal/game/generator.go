package game

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// ZoneGeneratorConfig holds generation settings
type ZoneGeneratorConfig struct {
	Enabled              bool          `json:"enabled"`
	TriggerInterval      time.Duration `json:"trigger_interval"`
	ExplorationThreshold float64       `json:"exploration_threshold"`
	ScoreGapThreshold    int           `json:"score_gap_threshold"`
	MaxZones             int           `json:"max_zones"`
}

// ZoneGenerator creates new zones dynamically using LLM
type ZoneGenerator struct {
	config      ZoneGeneratorConfig
	lastGenTime time.Time
	zoneCount   int
	genFunc     func(prompt string) (string, error) // LLM call function
}

// NewZoneGenerator creates a generator with default settings
func NewZoneGenerator() *ZoneGenerator {
	return &ZoneGenerator{
		config: ZoneGeneratorConfig{
			Enabled:              true,
			TriggerInterval:      5 * time.Minute,
			ExplorationThreshold: 0.8,
			ScoreGapThreshold:    50,
			MaxZones:             8,
		},
		lastGenTime: time.Now(),
		zoneCount:   4, // Starting with 4 zones
	}
}

// SetLLMFunc sets the function used to call the LLM
func (zg *ZoneGenerator) SetLLMFunc(fn func(prompt string) (string, error)) {
	zg.genFunc = fn
}

// CheckTriggers evaluates if a new zone should be generated
func (zg *ZoneGenerator) CheckTriggers(world *World) TriggerResult {
	if !zg.config.Enabled || zg.zoneCount >= zg.config.MaxZones {
		return TriggerResult{ShouldGenerate: false}
	}

	// Check time-based trigger
	if time.Since(zg.lastGenTime) >= zg.config.TriggerInterval {
		return TriggerResult{
			ShouldGenerate: true,
			Reason:         "timer",
			Description:    "Periodic zone generation",
		}
	}

	// Check exploration threshold
	unlockedCount := 0
	for _, zone := range world.Zones.Zones {
		if zone.Unlocked {
			unlockedCount++
		}
	}
	explorationRatio := float64(unlockedCount) / float64(len(world.Zones.Zones))
	if explorationRatio >= zg.config.ExplorationThreshold {
		return TriggerResult{
			ShouldGenerate: true,
			Reason:         "exploration",
			Description:    fmt.Sprintf("%.0f%% of zones explored", explorationRatio*100),
		}
	}

	// Check score gap
	var redScore, blueScore int
	if red, ok := world.Teams.Teams["red"]; ok {
		redScore = red.Score
	}
	if blue, ok := world.Teams.Teams["blue"]; ok {
		blueScore = blue.Score
	}
	scoreGap := abs(redScore - blueScore)
	if scoreGap >= zg.config.ScoreGapThreshold {
		return TriggerResult{
			ShouldGenerate: true,
			Reason:         "score_gap",
			Description:    fmt.Sprintf("Score gap: %d points", scoreGap),
		}
	}

	return TriggerResult{ShouldGenerate: false}
}

// TriggerResult contains the outcome of trigger evaluation
type TriggerResult struct {
	ShouldGenerate bool   `json:"should_generate"`
	Reason         string `json:"reason"`
	Description    string `json:"description"`
}

// GenerateZone creates a new zone using the LLM
func (zg *ZoneGenerator) GenerateZone(world *World, trigger TriggerResult) (*GeneratedZone, error) {
	if zg.genFunc == nil {
		return nil, fmt.Errorf("LLM function not set")
	}

	prompt := zg.buildGenerationPrompt(world, trigger)

	response, err := zg.genFunc(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	generated, err := zg.parseGeneratedZone(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Validate and adjust bounds
	generated = zg.validateBounds(generated, world)

	zg.lastGenTime = time.Now()
	zg.zoneCount++

	log.Printf("🌍 Generated new zone: %s (%s)", generated.Zone.Name, generated.Zone.Theme)

	return generated, nil
}

// GeneratedZone contains the full generation result
type GeneratedZone struct {
	Zone       ZoneDefinition        `json:"zone"`
	Challenges []ChallengeDefinition `json:"challenges"`
	Gate       GateDefinition        `json:"gate"`
}

// ZoneDefinition from LLM
type ZoneDefinition struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Theme       string  `json:"theme"`
	Description string  `json:"description"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Rewards     int     `json:"rewards"`
}

// ChallengeDefinition from LLM
type ChallengeDefinition struct {
	Type             string   `json:"type"`
	Name             string   `json:"name"`
	Prompt           string   `json:"prompt"`
	Options          []string `json:"options,omitempty"`
	Difficulty       int      `json:"difficulty"`
	RequiresTeamwork bool     `json:"requires_teamwork"`
	TokenReward      int      `json:"token_reward"`
}

// GateDefinition from LLM
type GateDefinition struct {
	FromZone string     `json:"from_zone"`
	Position [2]float64 `json:"position"`
}

func (zg *ZoneGenerator) buildGenerationPrompt(world *World, trigger TriggerResult) string {
	var sb strings.Builder

	sb.WriteString(`# ROLE
You are the WORLD BUILDER for a competitive AI arena game.
Create engaging, balanced zones that enhance gameplay.

`)

	// Current state
	sb.WriteString(fmt.Sprintf(`# CURRENT WORLD STATE
- Map size: %dx%d
- Existing zones: %d
- Trigger reason: %s (%s)

`, world.Width, world.Height, len(world.Zones.Zones), trigger.Reason, trigger.Description))

	// Team scores
	sb.WriteString("## Team Scores\n")
	for id, team := range world.Teams.Teams {
		sb.WriteString(fmt.Sprintf("- %s: %d points, controls %d zones\n", id, team.Score, len(team.Zones)))
	}
	sb.WriteString("\n")

	// Existing zones
	sb.WriteString("## Existing Zone Bounds (avoid overlap)\n")
	for _, zone := range world.Zones.Zones {
		sb.WriteString(fmt.Sprintf("- %s: x=%v, y=%v, w=%v, h=%v\n",
			zone.ID, zone.Bounds.X, zone.Bounds.Y, zone.Bounds.Width, zone.Bounds.Height))
	}
	sb.WriteString("\n")

	// Balance guidance
	if trigger.Reason == "score_gap" {
		sb.WriteString(`## BALANCE GUIDANCE
One team is far ahead. Create a zone that gives the losing team a chance to catch up.
Consider: easier challenges, higher rewards, or strategic positioning.

`)
	}

	// Challenge type rotation
	challengeTypes := []string{"coordination", "memory", "spatial", "encoding"}
	nextType := challengeTypes[zg.zoneCount%len(challengeTypes)]
	sb.WriteString(fmt.Sprintf("## Suggested Challenge Type: %s\n\n", nextType))

	sb.WriteString(`# TASK
Create ONE new zone. Be creative with the theme and name!

# OUTPUT FORMAT (JSON only)
{
  "zone": {
    "name": "Creative Fantasy Name",
    "theme": "crystal/forest/void/fire/ice/shadow",
    "description": "1-2 sentence atmospheric description",
    "x": <number>,
    "y": <number>,
    "width": <number 150-300>,
    "height": <number 150-250>,
    "rewards": <20-60>
  },
  "challenges": [{
    "type": "coordination|memory|spatial",
    "name": "Challenge Name",
    "prompt": "The challenge description",
    "options": ["A", "B", "C"],
    "difficulty": 1-5,
    "requires_teamwork": true/false,
    "token_reward": 20-50
  }],
  "gate": {
    "from_zone": "existing_zone_id",
    "position": [x, y]
  }
}
`)

	return sb.String()
}

func (zg *ZoneGenerator) parseGeneratedZone(response string) (*GeneratedZone, error) {
	// Find JSON in response
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonStr := response[start : end+1]

	var generated GeneratedZone
	if err := json.Unmarshal([]byte(jsonStr), &generated); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}

	// Assign ID
	generated.Zone.ID = fmt.Sprintf("zone_%d", zg.zoneCount+1)

	return &generated, nil
}

func (zg *ZoneGenerator) validateBounds(generated *GeneratedZone, world *World) *GeneratedZone {
	zone := &generated.Zone

	// Ensure minimum size
	if zone.Width < 150 {
		zone.Width = 150
	}
	if zone.Height < 150 {
		zone.Height = 150
	}

	// Ensure within world bounds
	if zone.X < 0 {
		zone.X = 0
	}
	if zone.Y < 0 {
		zone.Y = 0
	}
	if zone.X+zone.Width > float64(world.Width) {
		zone.X = float64(world.Width) - zone.Width
	}
	if zone.Y+zone.Height > float64(world.Height) {
		zone.Y = float64(world.Height) - zone.Height
	}

	return generated
}

// ApplyGeneratedZone adds the generated zone to the world
func (zg *ZoneGenerator) ApplyGeneratedZone(world *World, generated *GeneratedZone) {
	// Add zone
	world.Zones.Zones[generated.Zone.ID] = &Zone{
		ID:          generated.Zone.ID,
		Name:        generated.Zone.Name,
		Theme:       generated.Zone.Theme,
		Description: generated.Zone.Description,
		Bounds: Rectangle{
			X:      generated.Zone.X,
			Y:      generated.Zone.Y,
			Width:  generated.Zone.Width,
			Height: generated.Zone.Height,
		},
		Unlocked: false,
		Rewards:  generated.Zone.Rewards,
	}

	// Add gate
	gateID := fmt.Sprintf("gate_%s_%s", generated.Gate.FromZone, generated.Zone.ID)
	challengeID := fmt.Sprintf("challenge_%s", generated.Zone.ID)

	requiresTeamwork := false
	if len(generated.Challenges) > 0 {
		requiresTeamwork = generated.Challenges[0].RequiresTeamwork
	}

	world.Zones.Gates[gateID] = &Gate{
		ID:               gateID,
		FromZone:         generated.Gate.FromZone,
		ToZone:           generated.Zone.ID,
		Position:         generated.Gate.Position,
		ChallengeID:      challengeID,
		Unlocked:         false,
		RequiresTeamwork: requiresTeamwork,
	}

	log.Printf("✅ Applied zone: %s with gate %s", generated.Zone.Name, gateID)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
