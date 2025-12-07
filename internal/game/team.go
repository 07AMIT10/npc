package game

// Team represents a team of NPCs working together
type Team struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Color   string   `json:"color"`
	Members []string `json:"members"` // NPC names
	Score   int      `json:"score"`
	Tokens  int      `json:"tokens"`
	Zones   []string `json:"zones"` // Zone IDs controlled by this team
}

// TeamProgress tracks team achievements
type TeamProgress struct {
	ChallengesSolved   int      `json:"challenges_solved"`
	ChallengesFailed   int      `json:"challenges_failed"`
	ZonesUnlocked      []string `json:"zones_unlocked"`
	CurrentStreak      int      `json:"current_streak"`
	BestStreak         int      `json:"best_streak"`
	TotalTokensEarned  int      `json:"total_tokens_earned"`
	TotalTokensSpent   int      `json:"total_tokens_spent"`
	CollaborationCount int      `json:"collaboration_count"` // Times both members worked together
}

// TeamManager handles team operations
type TeamManager struct {
	Teams    map[string]*Team         `json:"teams"`
	Progress map[string]*TeamProgress `json:"progress"`
}

// NewTeamManager creates a team manager with default 2v2 setup
func NewTeamManager() *TeamManager {
	tm := &TeamManager{
		Teams:    make(map[string]*Team),
		Progress: make(map[string]*TeamProgress),
	}

	// Create Team Red
	tm.Teams["red"] = &Team{
		ID:      "red",
		Name:    "Team Red",
		Color:   "#ef4444",
		Members: []string{"Explorer", "Scout"},
		Score:   0,
		Tokens:  50, // Starting tokens
		Zones:   []string{"start"},
	}

	// Create Team Blue
	tm.Teams["blue"] = &Team{
		ID:      "blue",
		Name:    "Team Blue",
		Color:   "#3b82f6",
		Members: []string{"Wanderer", "Seeker"},
		Score:   0,
		Tokens:  50,
		Zones:   []string{"start"},
	}

	// Initialize progress tracking
	tm.Progress["red"] = &TeamProgress{}
	tm.Progress["blue"] = &TeamProgress{}

	return tm
}

// GetTeamForNPC returns the team that contains the given NPC
func (tm *TeamManager) GetTeamForNPC(npcName string) *Team {
	for _, team := range tm.Teams {
		for _, member := range team.Members {
			if member == npcName {
				return team
			}
		}
	}
	return nil
}

// GetTeammate returns the teammate of the given NPC
func (tm *TeamManager) GetTeammate(npcName string) string {
	team := tm.GetTeamForNPC(npcName)
	if team == nil {
		return ""
	}
	for _, member := range team.Members {
		if member != npcName {
			return member
		}
	}
	return ""
}

// GetOpponentTeam returns the opposing team
func (tm *TeamManager) GetOpponentTeam(teamID string) *Team {
	for id, team := range tm.Teams {
		if id != teamID {
			return team
		}
	}
	return nil
}

// AwardTokens adds tokens to a team
func (tm *TeamManager) AwardTokens(teamID string, amount int, reason string) {
	if team, ok := tm.Teams[teamID]; ok {
		team.Tokens += amount
		team.Score += amount
		if progress, ok := tm.Progress[teamID]; ok {
			progress.TotalTokensEarned += amount
		}
	}
}

// SpendTokens deducts tokens from a team (for hints, skips, etc.)
func (tm *TeamManager) SpendTokens(teamID string, amount int) bool {
	if team, ok := tm.Teams[teamID]; ok {
		if team.Tokens >= amount {
			team.Tokens -= amount
			if progress, ok := tm.Progress[teamID]; ok {
				progress.TotalTokensSpent += amount
			}
			return true
		}
	}
	return false
}

// RecordChallengeSolved records a successful challenge completion
func (tm *TeamManager) RecordChallengeSolved(teamID string, tokensEarned int) {
	if progress, ok := tm.Progress[teamID]; ok {
		progress.ChallengesSolved++
		progress.CurrentStreak++
		if progress.CurrentStreak > progress.BestStreak {
			progress.BestStreak = progress.CurrentStreak
		}
	}
	tm.AwardTokens(teamID, tokensEarned, "challenge_solved")
}

// RecordChallengeFailed records a failed challenge attempt
func (tm *TeamManager) RecordChallengeFailed(teamID string) {
	if progress, ok := tm.Progress[teamID]; ok {
		progress.ChallengesFailed++
		progress.CurrentStreak = 0
	}
}

// ClaimZone marks a zone as controlled by a team
func (tm *TeamManager) ClaimZone(teamID, zoneID string) {
	if team, ok := tm.Teams[teamID]; ok {
		// Check if already claimed
		for _, z := range team.Zones {
			if z == zoneID {
				return
			}
		}
		team.Zones = append(team.Zones, zoneID)
		if progress, ok := tm.Progress[teamID]; ok {
			progress.ZonesUnlocked = append(progress.ZonesUnlocked, zoneID)
		}
	}
}

// GetLeaderboard returns teams sorted by score
func (tm *TeamManager) GetLeaderboard() []*Team {
	teams := make([]*Team, 0, len(tm.Teams))
	for _, team := range tm.Teams {
		teams = append(teams, team)
	}
	// Sort by score (descending)
	for i := 0; i < len(teams)-1; i++ {
		for j := i + 1; j < len(teams); j++ {
			if teams[j].Score > teams[i].Score {
				teams[i], teams[j] = teams[j], teams[i]
			}
		}
	}
	return teams
}
