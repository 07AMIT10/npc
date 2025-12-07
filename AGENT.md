# NPC Arena - AI Agent Context Document

> This document is designed for AI coding assistants to quickly understand the NPC Arena codebase.

## 🎯 Project Summary

**NPC Arena** is an AI-powered browser game where 4 LLM-controlled NPCs compete in team-based challenges.

```
Core Concept:
- 4 NPCs (2 teams: Red vs Blue)
- NPCs make decisions via LLM API calls
- Goal: Unlock gates by solving challenges
- Teams compete for highest score
```

---

## 🏗️ Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      FRONTEND                                │
│  ┌────────────────┐  OR  ┌────────────────────────────┐     │
│  │ web/ (Vanilla) │      │ web-react/ (React+Canvas)  │     │
│  │  - game.js     │      │  - Zustand state           │     │
│  │  - index.html  │      │  - Framer Motion anims     │     │
│  │  - style.css   │      │  - Particle system         │     │
│  └────────────────┘      └────────────────────────────┘     │
│                              │                               │
│                        WebSocket                             │
│                              │                               │
├──────────────────────────────┼──────────────────────────────┤
│                      BACKEND (Go)                            │
│                              │                               │
│  ┌───────────────────────────▼───────────────────────────┐  │
│  │              cmd/server/main.go                        │  │
│  │  - Fiber HTTP server                                   │  │
│  │  - WebSocket handler                                   │  │
│  │  - Message routing (decision_request, batch_decisions) │  │
│  └───────────────────────────┬───────────────────────────┘  │
│                              │                               │
│  ┌───────────────────────────▼───────────────────────────┐  │
│  │                internal/api/                           │  │
│  │  manager.go   - LLM orchestration, retries, fallbacks  │  │
│  │  batch.go     - Multi-NPC single prompt (cost optim)   │  │
│  │  prompts.go   - Prompt templates for each action type  │  │
│  │  audit.go     - Logging/auditing of LLM calls          │  │
│  └───────────────────────────┬───────────────────────────┘  │
│                              │                               │
│  ┌───────────────────────────▼───────────────────────────┐  │
│  │                internal/llm/                           │  │
│  │  router.go       - Load balancing, provider selection  │  │
│  │  balancer.go     - Weighted round-robin algorithm      │  │
│  │  openai_adapter  - OpenAI-compatible API calls         │  │
│  │  gemini_adapter  - Google Gemini API calls             │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                internal/game/                         │   │
│  │  world.go    - Game state, NPCs, zones               │   │
│  │  team.go     - Team management, scores               │   │
│  │  zone.go     - Zone definitions, gate logic          │   │
│  │  npc.go      - NPC struct, position, state           │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## 📁 Key Files Reference

### Backend (Go)

| File | Purpose | Key Functions |
|------|---------|---------------|
| `cmd/server/main.go` | Entry point | Fiber setup, WebSocket, routes |
| `internal/api/manager.go` | LLM orchestration | `GetEnhancedDecision()`, `GetStrategy()` |
| `internal/api/batch.go` | **Cost optimization** | `GetBatchDecisions()` - single LLM call for all NPCs |
| `internal/api/prompts.go` | Prompt templates | `BuildMovementPrompt()`, `BuildChallengePrompt()` |
| `internal/llm/router.go` | Load balancer | `Complete()`, `CompleteWithProvider()` |
| `internal/llm/balancer.go` | Round-robin | Nginx-style weighted balancing |
| `internal/game/world.go` | Game state | `GetGameState()`, `GetNPCByName()` |
| `config.yaml` | Configuration | LLM providers, weights, game params |

### Frontend (React)

| File | Purpose | Key Exports |
|------|---------|-------------|
| `src/store/gameStore.ts` | Zustand state | `useGameStore`, `NPC`, `CONFIG` |
| `src/hooks/useWebSocket.ts` | WS connection | `useWebSocket()` → `requestBatchDecisions()` |
| `src/components/GameCanvas/` | Canvas renderer | Zones, gates, NPCs, particles |
| `src/components/Header/` | Top bar | Animated scores, controls |
| `src/components/Sidebar/` | Side panel | Team info, live feed, commentary |
| `src/components/Overlay/` | Floating UI | Tooltips, minimap |

---

## 🔄 Data Flow

### 1. Decision Request Flow (Cost-Optimized Batch)

```
Frontend (every 4 seconds):
  │
  ▼
requestBatchDecisions()
  │
  ├─ Collect observations from all 4 NPCs
  │
  ▼
WebSocket: { type: "batch_decisions", observations: [...] }
  │
  ▼
main.go: case "batch_decisions"
  │
  ▼
batchSystem.GetBatchDecisions(ctx, observations)
  │
  ├─ Check cache for each NPC
  │   └─ If cached: return immediately (no API call)
  │
  ├─ Build multi-NPC prompt
  │   └─ "You control 4 NPCs... respond with JSON array"
  │
  ├─ Call LLM with fallback providers
  │   └─ Try primary → fallback1 → fallback2
  │
  ├─ Parse response, map decisions to NPCs
  │   └─ Cache new decisions (10s TTL)
  │
  ▼
WebSocket: { type: "batch_decisions", decisions: [...] }
  │
  ▼
Frontend: handleDecision(decision) for each NPC
  │
  ├─ Update NPC target position
  ├─ Add particle effects
  └─ Update UI state
```

### 2. NPC Observation Structure

```typescript
{
  npc_id: "npc_0",
  name: "Explorer",
  team: "red",
  pos: [150, 150],
  hp: 100,
  energy: 95,
  state: "idle",
  memory_code: "A749",
  nearby_npcs: [
    { id: "npc_1", name: "Scout", team: "red", distance: 100, isTeammate: true }
  ],
  nearby_gates: [
    { id: "gate_1_2", distance: 350, unlocked: false, requiresTeamwork: false }
  ]
}
```

### 3. LLM Decision Response

```typescript
{
  action: "move" | "challenge" | "talk" | "taunt" | "wait" | "explore",
  target: [x, y] | "gate_id" | "npc_name" | null,
  reason: "brief explanation",
  message?: "for talk/taunt actions"
}
```

---

## ⚙️ Configuration

### config.yaml Structure

```yaml
game:
  tick_rate: 60           # FPS for game loop
  decision_rate: 0.25     # Decisions per second per NPC (now batched)

# LLM providers with weighted load balancing
slm_providers:            # Fast decision-making (SLM = Small Language Model)
  - name: groq
    model: llama-3.1-8b-instant
    weight: 10            # Higher weight = more requests
    api_key_env: GROQ_API_KEY

brain_providers:          # Strategic thinking (slower, smarter)
  - name: gemini
    model: gemini-2.0-flash
    weight: 10
    api_key_env: GEMINI_API_KEY

# Per-NPC provider overrides (optional)
npc_providers:
  Explorer: groq          # Force specific NPC to use specific provider
```

### Environment Variables

```bash
GROQ_API_KEY=xxx          # Required: Fast LLM
GEMINI_API_KEY=xxx        # Required: Strategic LLM
PORT=8080                 # Server port (default 8080)
```

---

## 🎮 Game Mechanics

### NPCs
- 4 NPCs: Explorer, Scout (Red team) + Wanderer, Seeker (Blue team)
- Each has: position, energy, HP, state, thought
- States: `idle`, `moving`, `challenging`

### Zones
- 4 zones: Starting Grounds, Crystal Caverns, Whispering Woods, The Nexus
- Zones unlock when connected gate is solved

### Gates
- Connect zones, start locked
- Some require teamwork (2 NPCs)
- Challenges: puzzles, memory, coordination

### Scoring
- Solve challenge: +10 points
- First to zone: +5 points
- Team with most points wins

---

## 🧩 Extension Points

### Adding a New LLM Provider

1. Create adapter in `internal/llm/`:
```go
// anthropic_adapter.go
type AnthropicAdapter struct { ... }
func (a *AnthropicAdapter) Complete(ctx, prompt) (string, error) { ... }
```

2. Register in `internal/llm/router.go`
3. Add to `config.yaml`

### Adding a New NPC Action

1. Add prompt example in `internal/api/prompts.go`:
```go
{\"action\": \"dance\", \"target\": null, \"reason\": \"celebrating\"}
```

2. Handle in frontend `handleAIResponse()` (game.js or useWebSocket.ts)
3. Add visual effect if needed

### Adding More NPCs

No code changes needed! The batch system auto-configures:
- Add NPC in `internal/game/world.go` initialization
- Add to frontend initial state
- Prompts auto-include new NPCs

---

## 🔍 Debugging Tips

### View LLM Calls
```bash
tail -f logs/audit.log
```

### Check Batch Stats
```
GET /stats → batch_stats: { cache_hit_rate, cost_savings }
```

### Test Providers
```
GET /test → { provider_name: { status, latency, response } }
```

### Frontend Console
```
📦 Batch request: 4 NPCs
📦 Batch response: 4 decisions (2 cached)
```

---

## 📊 Performance Optimizations

| Optimization | Implementation | Impact |
|--------------|----------------|--------|
| **Batch Decisions** | `batch.go` - single prompt for all NPCs | 75% fewer API calls |
| **Decision Cache** | 10s TTL, hash-based key | 30% cache hits |
| **Context Cancellation** | Go context in LLM calls | Cancel on user reset |
| **Provider Fallback** | Try next provider on failure | Higher reliability |
| **Weighted Load Balancing** | Round-robin with weights | Distribute load |

---

## 🚀 Deployment

### Docker Build
```bash
docker build -t npc-arena .
# Multi-stage: React build → Go build → Alpine runtime
# Final image ~50MB
```

### Render (Free Tier)
- Uses `render.yaml` blueprint
- Auto-deploys from main branch
- Add UptimeRobot ping to prevent sleep

---

## 📝 Common Tasks for AI Assistants

### "Fix a bug where NPC targets are invalid"
→ Check `internal/api/prompts.go` validation
→ Check `batch.go` response parsing
→ Frontend: `handleAIResponse()` target handling

### "Add a new visual effect"
→ `web-react/src/components/GameCanvas/` for canvas effects
→ Add particle type in `gameStore.ts`
→ Emit particle in `useWebSocket.ts` on action

### "Reduce LLM costs further"
→ Increase cache TTL in `batch.go`
→ Reduce decision frequency in config
→ Optimize prompt tokens in `prompts.go`

### "Support a new LLM provider"
→ Create adapter in `internal/llm/`
→ Implement `Provider` interface
→ Add to `config.yaml`

---

*Last updated: December 2024*
