package challenge

import (
	"time"
)

// ChallengeType defines the category of challenge
type ChallengeType string

const (
	TypeCoordination  ChallengeType = "coordination"   // Both teammates choose same option
	TypeMemory        ChallengeType = "memory"         // Remember something from earlier
	TypeSpatial       ChallengeType = "spatial"        // Navigate/pathfind
	TypeInfoAsymmetry ChallengeType = "info_asymmetry" // Combine split information
	TypeEncoding      ChallengeType = "encoding"       // Create/decode secret messages
	TypeDebate        ChallengeType = "debate"         // Argue for a position
)

// ChallengeStatus tracks the state of an active challenge
type ChallengeStatus string

const (
	StatusPending   ChallengeStatus = "pending"
	StatusActive    ChallengeStatus = "active"
	StatusWaiting   ChallengeStatus = "waiting" // Waiting for second teammate
	StatusCompleted ChallengeStatus = "completed"
	StatusFailed    ChallengeStatus = "failed"
	StatusExpired   ChallengeStatus = "expired"
)

// Challenge represents a puzzle that NPCs must solve
type Challenge struct {
	ID          string        `json:"id"`
	Type        ChallengeType `json:"type"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Difficulty  int           `json:"difficulty"` // 1-5

	// The actual challenge content
	Prompt   string   `json:"prompt"`
	Options  []string `json:"options,omitempty"`  // For multi-choice
	Solution string   `json:"solution,omitempty"` // Expected answer (for auto-validation)

	// Requirements
	RequiresTeamwork bool          `json:"requires_teamwork"`
	TimeLimit        time.Duration `json:"time_limit"`

	// Rewards
	TokenReward int `json:"token_reward"`

	// Metadata
	Hints    []string `json:"hints,omitempty"`
	HintCost int      `json:"hint_cost"`
}

// ActiveChallenge tracks an in-progress challenge attempt
type ActiveChallenge struct {
	Challenge *Challenge      `json:"challenge"`
	GateID    string          `json:"gate_id"`
	Status    ChallengeStatus `json:"status"`

	// Participants
	Participants []string `json:"participants"` // NPC names
	TeamID       string   `json:"team_id"`

	// Responses
	Responses map[string]string `json:"responses"` // NPC name -> response

	// Timing
	StartedAt   time.Time  `json:"started_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Result
	Success      bool   `json:"success"`
	Feedback     string `json:"feedback"`
	TokensEarned int    `json:"tokens_earned"`
	HintsUsed    int    `json:"hints_used"`
}

// ChallengeResult is returned after validating a challenge attempt
type ChallengeResult struct {
	Success       bool    `json:"success"`
	Feedback      string  `json:"feedback"`
	TokensEarned  int     `json:"tokens_earned"`
	PartialCredit float64 `json:"partial_credit"` // 0.0 to 1.0
}

// ChallengeManager handles all challenge operations
type ChallengeManager struct {
	Challenges       map[string]*Challenge       `json:"challenges"`
	ActiveChallenges map[string]*ActiveChallenge `json:"active_challenges"` // gate_id -> active
}

// NewChallengeManager creates a manager with default challenges
func NewChallengeManager() *ChallengeManager {
	cm := &ChallengeManager{
		Challenges:       make(map[string]*Challenge),
		ActiveChallenges: make(map[string]*ActiveChallenge),
	}

	// Create default challenges
	cm.registerDefaultChallenges()

	return cm
}

func (cm *ChallengeManager) registerDefaultChallenges() {
	// Challenge 1: Coordination Game
	cm.Challenges["challenge_coordination"] = &Challenge{
		ID:          "challenge_coordination",
		Type:        TypeCoordination,
		Name:        "The Sync Test",
		Description: "Both teammates must choose the same option without communicating",
		Difficulty:  2,
		Prompt: `You and your teammate must choose the SAME option without communicating.
Think: what would your teammate most likely choose? 
Choose wisely - you only get one chance.`,
		Options:          []string{"ALPHA", "BETA", "GAMMA"},
		RequiresTeamwork: false, // Can attempt solo, but coordination version is harder
		TimeLimit:        30 * time.Second,
		TokenReward:      25,
		Hints:            []string{"Think about what's most 'default' or 'first'", "Consider alphabetical order"},
		HintCost:         5,
	}

	// Challenge 2: Teamwork Gate
	cm.Challenges["challenge_teamwork"] = &Challenge{
		ID:          "challenge_teamwork",
		Type:        TypeCoordination,
		Name:        "The Bond Test",
		Description: "Requires both teammates present to attempt",
		Difficulty:  3,
		Prompt: `This gate requires true coordination.
Both teammates must be here AND choose the same color.
The gate will only open if you think alike.`,
		Options:          []string{"RED", "BLUE", "GREEN"},
		RequiresTeamwork: true,
		TimeLimit:        45 * time.Second,
		TokenReward:      40,
		Hints:            []string{"Your team has a color...", "Think about team identity"},
		HintCost:         8,
	}

	// Challenge 3: Memory Test
	cm.Challenges["challenge_memory"] = &Challenge{
		ID:          "challenge_memory",
		Type:        TypeMemory,
		Name:        "The Recall",
		Description: "Remember the code you were given at the start",
		Difficulty:  3,
		Prompt: `Earlier in your journey, you were given a code.
What was it? Enter the exact code to proceed.`,
		RequiresTeamwork: false,
		TimeLimit:        20 * time.Second,
		TokenReward:      35,
		Hints:            []string{"It was 4 characters", "Format: LETTER-NUMBER-NUMBER-NUMBER"},
		HintCost:         7,
	}

	// Challenge 4: Spatial Navigation
	cm.Challenges["challenge_spatial"] = &Challenge{
		ID:          "challenge_spatial",
		Type:        TypeSpatial,
		Name:        "The Pathfinder",
		Description: "Find the optimal path avoiding obstacles",
		Difficulty:  4,
		Prompt: `You are at position A. Target is at position B.
Obstacles block direct paths. 
Describe the optimal route (e.g., "right 2, down 3, right 1").`,
		RequiresTeamwork: true,
		TimeLimit:        60 * time.Second,
		TokenReward:      50,
		Hints:            []string{"Draw it out mentally", "Sometimes going around is faster"},
		HintCost:         10,
	}
}

// GetChallenge returns a challenge by ID
func (cm *ChallengeManager) GetChallenge(id string) *Challenge {
	return cm.Challenges[id]
}

// StartChallenge initiates a challenge attempt
func (cm *ChallengeManager) StartChallenge(gateID, challengeID, npcName, teamID string) (*ActiveChallenge, error) {
	challenge := cm.GetChallenge(challengeID)
	if challenge == nil {
		return nil, nil
	}

	// Check if already active
	if active, exists := cm.ActiveChallenges[gateID]; exists {
		if active.Status == StatusActive || active.Status == StatusWaiting {
			// Add participant if it's a teamwork challenge
			if challenge.RequiresTeamwork {
				found := false
				for _, p := range active.Participants {
					if p == npcName {
						found = true
						break
					}
				}
				if !found {
					active.Participants = append(active.Participants, npcName)
				}
			}
			return active, nil
		}
	}

	// Create new active challenge
	now := time.Now()
	active := &ActiveChallenge{
		Challenge:    challenge,
		GateID:       gateID,
		Status:       StatusActive,
		Participants: []string{npcName},
		TeamID:       teamID,
		Responses:    make(map[string]string),
		StartedAt:    now,
		ExpiresAt:    now.Add(challenge.TimeLimit),
	}

	if challenge.RequiresTeamwork {
		active.Status = StatusWaiting // Waiting for teammate
	}

	cm.ActiveChallenges[gateID] = active
	return active, nil
}

// SubmitResponse records an NPC's response to a challenge
func (cm *ChallengeManager) SubmitResponse(gateID, npcName, response string) (bool, string) {
	active, exists := cm.ActiveChallenges[gateID]
	if !exists {
		return false, "No active challenge at this gate"
	}

	if time.Now().After(active.ExpiresAt) {
		active.Status = StatusExpired
		return false, "Challenge expired"
	}

	active.Responses[npcName] = response

	// Check if all required responses are in
	challenge := active.Challenge
	if challenge.RequiresTeamwork {
		if len(active.Responses) < 2 {
			return true, "Response recorded. Waiting for teammate..."
		}
	}

	return true, "Response recorded"
}

// EvaluateChallenge checks if the challenge was solved
func (cm *ChallengeManager) EvaluateChallenge(gateID string) *ChallengeResult {
	active, exists := cm.ActiveChallenges[gateID]
	if !exists {
		return nil
	}

	challenge := active.Challenge
	result := &ChallengeResult{}

	switch challenge.Type {
	case TypeCoordination:
		// All responses must match
		var firstResponse string
		allMatch := true
		for _, resp := range active.Responses {
			if firstResponse == "" {
				firstResponse = resp
			} else if resp != firstResponse {
				allMatch = false
				break
			}
		}
		result.Success = allMatch && firstResponse != ""
		if result.Success {
			result.Feedback = "Perfect coordination! Both chose: " + firstResponse
			result.TokensEarned = challenge.TokenReward
		} else {
			result.Feedback = "Coordination failed - different choices"
		}

	case TypeMemory:
		// Check if any response matches the solution
		for _, resp := range active.Responses {
			if resp == challenge.Solution {
				result.Success = true
				result.Feedback = "Correct! You remembered the code."
				result.TokensEarned = challenge.TokenReward
				break
			}
		}
		if !result.Success {
			result.Feedback = "Incorrect code"
		}

	default:
		// For other types, might need LLM judging
		result.Feedback = "Challenge evaluation pending..."
	}

	// Apply hint penalty
	hintPenalty := active.HintsUsed * challenge.HintCost
	result.TokensEarned = max(0, result.TokensEarned-hintPenalty)

	// Update active challenge status
	if result.Success {
		active.Status = StatusCompleted
		active.Success = true
	} else {
		active.Status = StatusFailed
		active.Success = false
	}
	now := time.Now()
	active.CompletedAt = &now
	active.Feedback = result.Feedback
	active.TokensEarned = result.TokensEarned

	return result
}

// UseHint provides a hint and deducts from potential reward
func (cm *ChallengeManager) UseHint(gateID string, hintIndex int) (string, bool) {
	active, exists := cm.ActiveChallenges[gateID]
	if !exists {
		return "", false
	}

	hints := active.Challenge.Hints
	if hintIndex >= len(hints) {
		return "No more hints available", false
	}

	active.HintsUsed++
	return hints[hintIndex], true
}

// GetActiveChallenge returns the active challenge at a gate
func (cm *ChallengeManager) GetActiveChallenge(gateID string) *ActiveChallenge {
	return cm.ActiveChallenges[gateID]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
