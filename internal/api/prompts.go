package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PromptRole defines the type of LLM task
type PromptRole string

const (
	RoleMovement   PromptRole = "movement"
	RoleChallenge  PromptRole = "challenge"
	RoleJudge      PromptRole = "judge"
	RoleCommentary PromptRole = "commentary"
	RoleStrategy   PromptRole = "strategy"
)

// PromptBuilder creates well-structured prompts using proven techniques
type PromptBuilder struct{}

// BuildMovementPrompt creates a context-rich prompt for NPC movement decisions
func (pb *PromptBuilder) BuildMovementPrompt(obs map[string]interface{}) string {
	name := getString(obs, "name")
	team := getString(obs, "team")
	pos := getArray(obs, "pos")
	energy := getInt(obs, "energy")
	memoryCode := getString(obs, "memory_code")

	myX := int(pos[0].(float64))
	myY := int(pos[1].(float64))

	nearbyNPCs := getArrayOfMaps(obs, "nearby_npcs")
	nearbyGates := getArrayOfMaps(obs, "nearby_gates")

	var sb strings.Builder

	// PERSONALITY based on name
	personality := map[string]string{
		"Explorer": "bold and confident, loves to take risks",
		"Scout":    "cautious and observant, good at spotting opportunities",
		"Wanderer": "laid-back but competitive, enjoys taunting rivals",
		"Seeker":   "focused and strategic, always has a plan",
	}
	myPersonality := personality[name]
	if myPersonality == "" {
		myPersonality = "competitive and determined"
	}

	// ROLE - storytelling with personality
	sb.WriteString(fmt.Sprintf(`You are %s from Team %s. 
Personality: %s

YOUR POSITION: (%d, %d), Energy: %d%%
`, name, strings.ToUpper(team), myPersonality, myX, myY, energy))

	// Find teammates and opponents
	var teammate map[string]interface{}
	var opponents []map[string]interface{}
	var teammateNear, opponentNear bool

	for _, npc := range nearbyNPCs {
		if getBool(npc, "isTeammate") {
			teammate = npc
			if getFloat(npc, "distance") < 100 {
				teammateNear = true
			}
		} else {
			opponents = append(opponents, npc)
			if getFloat(npc, "distance") < 80 {
				opponentNear = true
			}
		}
	}

	// SOCIAL SITUATION
	sb.WriteString("\n## WHO'S AROUND YOU\n")

	if teammate != nil {
		tName := getString(teammate, "name")
		tDist := getFloat(teammate, "distance")
		if teammateNear {
			sb.WriteString(fmt.Sprintf("👥 TEAMMATE %s is right here (%.0f units)! You can work together.\n", tName, tDist))
		} else {
			sb.WriteString(fmt.Sprintf("→ Teammate %s is %.0f units away\n", tName, tDist))
		}
	}

	if len(opponents) > 0 {
		sb.WriteString("\n⚔️ OPPONENTS SPOTTED:\n")
		for _, opp := range opponents {
			oppName := getString(opp, "name")
			oppDist := getFloat(opp, "distance")
			oppState := getString(opp, "state")
			if oppDist < 80 {
				sb.WriteString(fmt.Sprintf("� %s is RIGHT NEXT TO YOU (%.0f units, %s) - SAY SOMETHING!\n", oppName, oppDist, oppState))
			} else {
				sb.WriteString(fmt.Sprintf("- %s: %.0f units away, %s\n", oppName, oppDist, oppState))
			}
		}
	}

	// Find closest gate
	var closestGate map[string]interface{}
	closestDist := 9999.0
	needsTeamwork := false
	for _, gate := range nearbyGates {
		if !getBool(gate, "unlocked") {
			dist := getFloat(gate, "distance")
			if dist < closestDist {
				closestDist = dist
				closestGate = gate
				needsTeamwork = getBool(gate, "requiresTeamwork")
			}
		}
	}

	// GATES
	if len(nearbyGates) > 0 {
		sb.WriteString("\n## NEARBY GATES\n")
		for _, gate := range nearbyGates {
			if getBool(gate, "unlocked") {
				continue
			}
			gateID := getString(gate, "id")
			dist := getFloat(gate, "distance")
			tw := ""
			if getBool(gate, "requiresTeamwork") {
				tw = " [2-PLAYER]"
			}
			sb.WriteString(fmt.Sprintf("- %s: %.0f units%s\n", gateID, dist, tw))
		}
	}

	// DECISION GUIDANCE
	sb.WriteString("\n## WHAT SHOULD YOU DO?\n")

	// Priority 1: Social interaction if opponent very close
	if opponentNear {
		sb.WriteString(`🗣️ An OPPONENT is right next to you! This is a competitive game.
OPTIONS:
1. TAUNT them - say something competitive/playful
2. TALK - make conversation (if you're feeling friendly)
3. RACE them to the nearest gate!
`)
	}

	// Priority 2: Teammate coordination
	if teammateNear && closestGate != nil && needsTeamwork && closestDist < 100 {
		gateID := getString(closestGate, "id")
		sb.WriteString(fmt.Sprintf(`
👥 Your teammate is here and gate %s needs 2 players!
Talk to your teammate or start the challenge together!
`, gateID))
	}

	// Priority 3: Gate challenge
	if closestGate != nil && closestDist < 60 {
		gateID := getString(closestGate, "id")
		sb.WriteString(fmt.Sprintf("🔒 You're at gate %s! Attempt the challenge.\n", gateID))
	} else if closestGate != nil {
		gateID := getString(closestGate, "id")
		sb.WriteString(fmt.Sprintf("→ Move toward gate %s (%.0f units)\n", gateID, closestDist))
	}

	sb.WriteString(fmt.Sprintf("\nYour secret code: %s (for memory challenges)\n", memoryCode))

	// Build explicit valid targets list (industry best practice: constrained generation)
	var validTargets []string
	if teammate != nil {
		validTargets = append(validTargets, getString(teammate, "name"))
	}
	for _, opp := range opponents {
		validTargets = append(validTargets, getString(opp, "name"))
	}

	if len(validTargets) > 0 {
		sb.WriteString(fmt.Sprintf("\n⚠️ VALID TARGETS for talk/taunt: %v\n", validTargets))
		sb.WriteString(fmt.Sprintf("🚫 NEVER target yourself (%s) - that makes no sense!\n", name))
	}

	// OUTPUT FORMAT with social actions
	sb.WriteString(`
## OUTPUT (JSON only)
EXAMPLES:
{"action": "move", "target": [400, 200], "reason": "heading to gate"}
{"action": "challenge", "target": "gate_1_2", "reason": "solving puzzle"}
{"action": "talk", "target": "Scout", "message": "Let's team up!"}
{"action": "taunt", "target": "Wanderer", "message": "You're too slow!"}
{"action": "wait", "target": null, "reason": "waiting for teammate"}

RULES:
- Use REAL numbers in target, NOT expressions like [x+100, y-50]
- For talk/taunt, target must be someone ELSE - never yourself!
- Keep messages short and punchy
`)

	return sb.String()
}

// BuildChallengePrompt creates a prompt for solving a challenge
func (pb *PromptBuilder) BuildChallengePrompt(challenge map[string]interface{}, npcContext map[string]interface{}) string {
	challengeType := getString(challenge, "type")
	prompt := getString(challenge, "prompt")
	options := getStringArray(challenge, "options")

	npcName := getString(npcContext, "name")
	team := getString(npcContext, "team")
	memoryCode := getString(npcContext, "memory_code")

	var sb strings.Builder

	// ROLE
	sb.WriteString(fmt.Sprintf(`# ROLE
You are %s (Team %s), attempting to solve a challenge.

`, npcName, strings.ToUpper(team)))

	// CHALLENGE
	sb.WriteString(fmt.Sprintf(`# CHALLENGE TYPE: %s

%s

`, strings.ToUpper(challengeType), prompt))

	// Add memory hint for memory challenges
	if challengeType == "memory" {
		sb.WriteString(fmt.Sprintf(`# HINT
You were given a code earlier: %s
Use this to solve the challenge.

`, memoryCode))
	}

	// Options if available
	if len(options) > 0 {
		sb.WriteString("# OPTIONS\n")
		for _, opt := range options {
			sb.WriteString(fmt.Sprintf("- %s\n", opt))
		}
		sb.WriteString("\n")
	}

	// Chain of thought
	sb.WriteString(`# YOUR APPROACH
Think step by step:
1. What is the core problem?
2. What information do I have?
3. What's the most logical answer?

# OUTPUT FORMAT (JSON only)
{"thinking": "your reasoning (max 30 words)", "answer": "your final answer"}
`)

	return sb.String()
}

// BuildJudgePrompt creates a prompt for evaluating challenge responses
func (pb *PromptBuilder) BuildJudgePrompt(challenge, responses map[string]interface{}) string {
	challengeType := getString(challenge, "type")
	prompt := getString(challenge, "prompt")
	solution := getString(challenge, "solution")
	requiresTeamwork := getBool(challenge, "requires_teamwork")

	var sb strings.Builder

	// ROLE
	sb.WriteString(`# ROLE
You are an impartial judge evaluating challenge responses in a game.
Be fair but strict. Partial credit is allowed.

`)

	// EXAMPLES (few-shot)
	sb.WriteString(`# EXAMPLES OF CORRECT JUDGMENTS

## Coordination Challenge
Challenge: "Both players must choose the same color"
Responses: {"Player1": "BLUE", "Player2": "BLUE"}
Judgment: {"correct": true, "feedback": "Perfect coordination!", "score": 1.0}

## Coordination Challenge (Failed)
Responses: {"Player1": "RED", "Player2": "BLUE"}
Judgment: {"correct": false, "feedback": "Different choices", "score": 0.0}

## Memory Challenge
Expected: "A749"
Response: "A749"
Judgment: {"correct": true, "feedback": "Correct recall", "score": 1.0}

`)

	// CURRENT CHALLENGE
	sb.WriteString(fmt.Sprintf(`# NOW JUDGE THIS

Challenge Type: %s
Challenge: %s
`, strings.ToUpper(challengeType), prompt))

	if solution != "" {
		sb.WriteString(fmt.Sprintf("Expected Answer: %s\n", solution))
	}

	sb.WriteString(fmt.Sprintf("Requires Teamwork: %v\n\n", requiresTeamwork))

	// Responses
	sb.WriteString("## Responses Received\n")
	responsesJSON, _ := json.MarshalIndent(responses, "", "  ")
	sb.WriteString(string(responsesJSON))
	sb.WriteString("\n\n")

	// OUTPUT
	sb.WriteString(`# OUTPUT FORMAT (JSON only)
{"correct": true/false, "feedback": "brief explanation", "score": 0.0-1.0}
`)

	return sb.String()
}

// BuildCommentaryPrompt creates a prompt for generating play-by-play commentary
func (pb *PromptBuilder) BuildCommentaryPrompt(events []map[string]interface{}, scores map[string]int) string {
	var sb strings.Builder

	sb.WriteString(`# ROLE
You are an exciting sports commentator for an AI arena game.
Be dramatic, fun, and concise!

`)

	sb.WriteString(fmt.Sprintf(`# CURRENT SCORES
Team Red: %d | Team Blue: %d

`, scores["red"], scores["blue"]))

	sb.WriteString("# RECENT EVENTS\n")
	for _, event := range events {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", event["event"], event["description"]))
	}

	sb.WriteString(`
# TASK
Generate ONE exciting commentary line (max 15 words).
Use emojis sparingly. Be dramatic but accurate.

Example outputs:
- "🔥 Explorer makes a BOLD move toward the Crystal Gate!"
- "Can Team Blue recover from this setback??"
- "What strategy! Scout and Explorer coordinate perfectly!"
`)

	return sb.String()
}

// BuildBatchPrompt creates a single prompt for multiple NPCs on the same team
func (pb *PromptBuilder) BuildBatchPrompt(observations []map[string]interface{}) string {
	if len(observations) == 0 {
		return ""
	}

	team := getString(observations[0], "team")

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`# ROLE
You are the strategist for Team %s, making decisions for BOTH team members.

`, strings.ToUpper(team)))

	sb.WriteString("# TEAM MEMBERS\n\n")

	for i, obs := range observations {
		name := getString(obs, "name")
		pos := getArray(obs, "pos")
		energy := getInt(obs, "energy")

		sb.WriteString(fmt.Sprintf("## Member %d: %s\n", i+1, name))
		sb.WriteString(fmt.Sprintf("- Position: (%v, %v)\n", pos[0], pos[1]))
		sb.WriteString(fmt.Sprintf("- Energy: %d%%\n", energy))

		nearbyGates := getArrayOfMaps(obs, "nearby_gates")
		if len(nearbyGates) > 0 {
			sb.WriteString("- Nearby gates: ")
			var gateStrs []string
			for _, g := range nearbyGates {
				gateStrs = append(gateStrs, fmt.Sprintf("%s (%.0f units)", getString(g, "id"), getFloat(g, "distance")))
			}
			sb.WriteString(strings.Join(gateStrs, ", "))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`# TASK
Coordinate both team members efficiently:
- Should they go to the same gate (teamwork challenge)?
- Should they split up to cover more ground?
- Is one closer to a gate they should prioritize?

# OUTPUT FORMAT (JSON only)
{
  "decisions": [
    {"npc": "Name1", "action": "move|challenge|wait", "target": [x,y] or "gate_id", "reason": "5 words max"},
    {"npc": "Name2", "action": "move|challenge|wait", "target": [x,y] or "gate_id", "reason": "5 words max"}
  ],
  "strategy": "brief team strategy (10 words max)"
}
`)

	return sb.String()
}

// Helper functions for safe type extraction

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		}
	}
	return 0
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func getArray(m map[string]interface{}, key string) []interface{} {
	if v, ok := m[key]; ok {
		if arr, ok := v.([]interface{}); ok {
			return arr
		}
	}
	return []interface{}{0, 0}
}

func getStringArray(m map[string]interface{}, key string) []string {
	if v, ok := m[key]; ok {
		if arr, ok := v.([]interface{}); ok {
			result := make([]string, len(arr))
			for i, item := range arr {
				if s, ok := item.(string); ok {
					result[i] = s
				}
			}
			return result
		}
	}
	return nil
}

func getArrayOfMaps(m map[string]interface{}, key string) []map[string]interface{} {
	if v, ok := m[key]; ok {
		if arr, ok := v.([]interface{}); ok {
			result := make([]map[string]interface{}, 0, len(arr))
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					result = append(result, m)
				}
			}
			return result
		}
	}
	return nil
}
