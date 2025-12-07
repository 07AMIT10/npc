# Smart NPC Explorer

A browser-based game where AI-driven NPCs explore a 2D world using LLM-powered decision making.

## Quick Start

```bash
# Option 1: Without API keys (demo mode)
./start.sh

# Option 2: With API keys (AI mode)
export GROQ_API_KEY=your_groq_key      # Fast action decisions
export GEMINI_API_KEY=your_gemini_key  # Strategic brain
./start.sh
```

Then open http://localhost:8080

## How It Works

```
┌──────────────┐     ┌─────────────┐     ┌──────────────┐
│  Browser UI  │ ←→  │  Go Server  │ ←→  │  LLM APIs    │
│  (Canvas)    │     │  (Fiber)    │     │  (SLM/Brain) │
└──────────────┘     └─────────────┘     └──────────────┘
```

- **4 NPCs** explore a world with treasures, landmarks, and mysteries
- **SLM (fast)**: Makes quick action decisions (move, explore, interact)
- **Brain LLM (smart)**: Coordinates team strategy

## API Providers

### Fast SLM (pick one)
- Groq: `GROQ_API_KEY`
- SambaNova: `SAMBANOVA_API_KEY`
- OpenRouter: `OPENROUTER_API_KEY`

### Brain LLM (pick one)
- Gemini (recommended): `GEMINI_API_KEY`
- OpenAI: `OPENAI_API_KEY`

## Project Structure

```
npc/
├── cmd/server/main.go      # Server entry point
├── internal/
│   ├── api/manager.go      # LLM API integrations
│   ├── config/config.go    # Configuration
│   └── game/world.go       # Game state
├── web/
│   ├── index.html          # Game page
│   ├── game.js             # Canvas + game logic
│   └── style.css           # Dark theme
└── config.yaml             # Settings
```

## Controls

- **Start**: Begin simulation
- **Pause**: Freeze NPCs
- **Reset**: Restart world

## Future Ideas

- Combat mechanics
- Open world support
- More NPC personalities
- Memory/learning
