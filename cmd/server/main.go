package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/amit/npc/internal/api"
	"github.com/amit/npc/internal/config"
	"github.com/amit/npc/internal/game"
	"github.com/amit/npc/internal/observability"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/websocket/v2"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Printf("Warning: Could not load config: %v, using defaults", err)
		cfg = config.Default()
	}

	// Initialize observability
	observer := observability.GetObserver()
	if err := observer.Initialize(observability.ObserverConfig{
		Enabled:   cfg.Observability.TraceEnabled,
		TracePath: cfg.Observability.TracePath,
		AuditPath: cfg.Observability.AuditPath,
	}); err != nil {
		log.Printf("Warning: Could not initialize observability: %v", err)
	}
	defer observer.Close()
	log.Println("📊 Observability initialized")

	// Initialize game world with v2 features
	world := game.NewWorld(cfg)
	log.Printf("🎮 Game world initialized with %d NPCs in %d zones", len(world.NPCs), len(world.Zones.Zones))
	log.Printf("🔴 Team Red: %v", world.Teams.Teams["red"].Members)
	log.Printf("🔵 Team Blue: %v", world.Teams.Teams["blue"].Members)

	// Initialize API manager (handles multiple providers)
	apiManager := api.NewManager(cfg)
	log.Printf("🤖 API Manager ready - SLM: %s, Brain: %s",
		apiManager.GetActiveSLM(), apiManager.GetActiveBrain())

	// Initialize batch decision system (cost optimization)
	batchSystem := api.NewBatchDecisionSystem(apiManager)
	log.Println("💰 Batch decision system ready (cost optimization enabled)")

	// Initialize zone generator (Phase 3)
	zoneGen := game.NewZoneGenerator()
	zoneGen.SetLLMFunc(func(prompt string) (string, error) {
		return apiManager.GetStrategy(prompt) // Use brain for generation
	})
	log.Println("🌍 Zone generator initialized")

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: "NPC Arena v2",
	})

	// Middleware
	app.Use(logger.New())
	app.Use(cors.New())

	// Serve static files
	app.Static("/", "./web")

	// WebSocket endpoint
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		log.Println("WebSocket client connected")
		observer.Audit("client_connected", "", "", nil)

		// Send initial game state
		c.WriteJSON(fiber.Map{
			"type":  "init",
			"slm":   apiManager.GetActiveSLM(),
			"brain": apiManager.GetActiveBrain(),
			"teams": world.Teams.Teams,
			"zones": world.Zones.Zones,
			"gates": world.Zones.Gates,
		})

		for {
			var msg map[string]interface{}
			if err := c.ReadJSON(&msg); err != nil {
				log.Printf("WebSocket read error: %v", err)
				break
			}

			switch msg["type"] {
			case "decision_request":
				obs := msg["observation"].(map[string]interface{})
				npcName := ""
				if name, ok := obs["name"].(string); ok {
					npcName = name
				}

				// Get AI decision using enhanced prompts (Phase 2)
				decision, err := apiManager.GetEnhancedDecision(obs)
				if err != nil {
					log.Printf("Decision error for %s: %v", npcName, err)
					decision = api.DefaultDecision(obs)
				}

				// Send decision back
				decision["type"] = "decision"
				c.WriteJSON(decision)

			case "batch_decisions":
				// COST OPTIMIZATION: Get decisions for ALL NPCs in a single LLM call!
				// This reduces API calls by ~75% (4 calls → 1 call)
				observationsRaw, ok := msg["observations"].([]interface{})
				if !ok {
					log.Println("⚠️ batch_decisions: invalid observations format")
					break
				}

				observations := make([]map[string]interface{}, 0, len(observationsRaw))
				for _, obsRaw := range observationsRaw {
					if obs, ok := obsRaw.(map[string]interface{}); ok {
						observations = append(observations, obs)
					}
				}

				if len(observations) == 0 {
					break
				}

				// Use batch system with context for cancellation support
				ctx := context.Background()
				result := batchSystem.GetBatchDecisions(ctx, observations)

				if result.Error != nil {
					log.Printf("⚠️ Batch decision error: %v", result.Error)
				}

				// Send all decisions back
				c.WriteJSON(fiber.Map{
					"type":       "batch_decisions",
					"decisions":  result.Decisions,
					"from_cache": result.FromCache,
				})

			case "brain_request":
				summary := msg["summary"].(string)

				// Get strategic advice from brain LLM
				strategy, err := apiManager.GetStrategy(summary)
				if err != nil {
					log.Printf("Brain error: %v", err)
					strategy = "Continue exploring systematically."
				}

				c.WriteJSON(fiber.Map{
					"type":     "brain_strategy",
					"strategy": strategy,
				})

			case "challenge_start":
				// NPC is attempting a challenge
				gateID := msg["gate_id"].(string)
				npcName := msg["npc"].(string)
				npc := world.GetNPCByName(npcName)
				if npc == nil {
					break
				}

				gate := world.Zones.Gates[gateID]
				if gate == nil || gate.Unlocked {
					break
				}

				active, _ := world.Challenges.StartChallenge(gateID, gate.ChallengeID, npcName, npc.Team)
				if active != nil {
					observer.AuditChallengeStart(npcName, npc.Team, gateID, string(active.Challenge.Type))
					c.WriteJSON(fiber.Map{
						"type":      "challenge_active",
						"challenge": active.Challenge,
						"status":    active.Status,
						"gate_id":   gateID,
					})
				}

			case "challenge_response":
				// NPC is submitting a challenge answer
				gateID := msg["gate_id"].(string)
				npcName := msg["npc"].(string)
				response := msg["response"].(string)

				success, feedback := world.Challenges.SubmitResponse(gateID, npcName, response)

				// Check if ready to evaluate
				active := world.Challenges.GetActiveChallenge(gateID)
				if active == nil {
					break
				}

				needsEval := !active.Challenge.RequiresTeamwork || len(active.Responses) >= 2
				if needsEval && success {
					result := world.Challenges.EvaluateChallenge(gateID)
					if result != nil {
						npc := world.GetNPCByName(npcName)
						if npc != nil {
							observer.AuditChallengeComplete(npcName, npc.Team, gateID, result.Success, result.TokensEarned)

							if result.Success {
								world.Zones.UnlockGate(gateID, npc.Team)
								world.Teams.RecordChallengeSolved(npc.Team, result.TokensEarned)
								observer.AuditZoneUnlock(npc.Team, world.Zones.Gates[gateID].ToZone, npcName)
							} else {
								world.Teams.RecordChallengeFailed(npc.Team)
							}
						}

						c.WriteJSON(fiber.Map{
							"type":     "challenge_result",
							"gate_id":  gateID,
							"success":  result.Success,
							"feedback": result.Feedback,
							"tokens":   result.TokensEarned,
							"teams":    world.Teams.Teams,
						})
					}
				} else {
					c.WriteJSON(fiber.Map{
						"type":     "challenge_waiting",
						"gate_id":  gateID,
						"feedback": feedback,
					})
				}

			case "team_message":
				// NPC sending message to teammate
				fromNPC := msg["from"].(string)
				message := msg["message"].(string)
				npc := world.GetNPCByName(fromNPC)
				if npc != nil {
					teammate := world.Teams.GetTeammate(fromNPC)
					world.SendMessage(fromNPC, teammate, message)
					observer.AuditTeamMessage(fromNPC, npc.Team, message)

					c.WriteJSON(fiber.Map{
						"type":    "message_sent",
						"from":    fromNPC,
						"to":      teammate,
						"message": message,
					})
				}

			case "get_commentary":
				// Client requesting live commentary
				events := []map[string]interface{}{}
				if evts, ok := msg["events"].([]interface{}); ok {
					for _, e := range evts {
						if em, ok := e.(map[string]interface{}); ok {
							events = append(events, em)
						}
					}
				}
				scores := world.GetTeamScores()

				commentary, err := apiManager.GetCommentary(events, scores)
				if err != nil {
					commentary = "The game continues..."
				}

				c.WriteJSON(fiber.Map{
					"type":       "commentary",
					"commentary": commentary,
				})

			case "check_zone_generation":
				// Check if we should generate a new zone
				trigger := zoneGen.CheckTriggers(world)
				if trigger.ShouldGenerate {
					generated, err := zoneGen.GenerateZone(world, trigger)
					if err != nil {
						log.Printf("Zone generation failed: %v", err)
					} else {
						zoneGen.ApplyGeneratedZone(world, generated)
						observer.Audit("zone_generated", "", "", map[string]interface{}{
							"zone_id":   generated.Zone.ID,
							"zone_name": generated.Zone.Name,
							"trigger":   trigger.Reason,
						})

						c.WriteJSON(fiber.Map{
							"type":  "zone_generated",
							"zone":  generated.Zone,
							"gate":  generated.Gate,
							"zones": world.Zones.Zones,
							"gates": world.Zones.Gates,
						})
					}
				}

			case "get_state":
				// Client requesting current game state
				c.WriteJSON(fiber.Map{
					"type":  "game_state",
					"state": world.GetGameState(),
				})
			}
		}

		log.Println("WebSocket client disconnected")
		observer.Audit("client_disconnected", "", "", nil)
	}))

	// Health check with provider stats
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"slm":    apiManager.GetActiveSLM(),
			"brain":  apiManager.GetActiveBrain(),
			"stats":  apiManager.GetStats(),
		})
	})

	// Game state endpoint
	app.Get("/state", func(c *fiber.Ctx) error {
		return c.JSON(world.GetGameState())
	})

	// Teams and scores
	app.Get("/teams", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"teams":       world.Teams.Teams,
			"progress":    world.Teams.Progress,
			"leaderboard": world.Teams.GetLeaderboard(),
		})
	})

	// Observability stats
	app.Get("/stats", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"llm_stats":     observer.GetStats(),
			"game_stats":    world.GetTeamScores(),
			"batch_stats":   batchSystem.GetStats(), // Cost optimization metrics
			"recent_traces": observer.GetRecentTraces(10),
			"recent_events": observer.GetRecentAudits(20),
		})
	})

	// Test all providers endpoint
	app.Get("/test", func(c *fiber.Ctx) error {
		log.Println("🧪 Testing all providers...")
		results := apiManager.TestProviders()
		log.Println("🧪 Provider test complete")
		return c.JSON(results)
	})

	// Legacy audit log endpoint
	app.Get("/audit", func(c *fiber.Ctx) error {
		auditLog := api.GetAuditLog()
		return c.JSON(fiber.Map{
			"entries": auditLog.GetEntries(50),
			"stats":   auditLog.GetStats(),
		})
	})

	// Replay endpoints (Phase 4)
	replayManager := observability.NewReplayManager(cfg.Observability.ReplayEnabled, "./logs/replay.json")

	app.Get("/replay/timeline", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"timeline": replayManager.GetTimeline(),
		})
	})

	app.Get("/replay/snapshot/:tick", func(c *fiber.Ctx) error {
		tick, err := c.ParamsInt("tick")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid tick"})
		}
		snapshot := replayManager.GetSnapshotByTick(tick)
		if snapshot == nil {
			return c.Status(404).JSON(fiber.Map{"error": "Snapshot not found"})
		}
		return c.JSON(snapshot)
	})

	// Create snapshots periodically in WebSocket handler (already done via world.GetGameState())
	// The replayManager will be used to store snapshots during gameplay

	// Find available port
	port := findAvailablePort()

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("🎮 NPC Arena v2 starting on http://localhost:%s", port)
	log.Printf("📊 Stats dashboard: http://localhost:%s/stats", port)
	log.Printf("🧪 Test providers: http://localhost:%s/test", port)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Fatal(app.Listen(":" + port))
}

// findAvailablePort checks preferred port from env, then tries a range of ports
func findAvailablePort() string {
	preferredPort := os.Getenv("PORT")
	if preferredPort == "" {
		preferredPort = "8080"
	}

	if isPortAvailable(preferredPort) {
		return preferredPort
	}

	log.Printf("Port %s is in use, finding available port...", preferredPort)

	for p := 8081; p <= 8099; p++ {
		port := fmt.Sprintf("%d", p)
		if isPortAvailable(port) {
			return port
		}
	}

	return "0"
}

func isPortAvailable(port string) bool {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
