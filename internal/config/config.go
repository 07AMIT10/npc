package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Game           GameConfig          `yaml:"game"`
	NPCs           NPCConfig           `yaml:"npcs"`
	Teams          TeamsConfig         `yaml:"teams"`
	SLMProviders   []ProviderConfig    `yaml:"slm_providers"`
	BrainProviders []ProviderConfig    `yaml:"brain_providers"`
	ModelRoles     ModelRolesConfig    `yaml:"model_roles"`
	Observability  ObservabilityConfig `yaml:"observability"`
	Server         ServerConfig        `yaml:"server"`
}

type GameConfig struct {
	TickRate       int `yaml:"tick_rate"`
	DecisionRate   int `yaml:"decision_rate"`
	WorldWidth     int `yaml:"world_width"`
	WorldHeight    int `yaml:"world_height"`
	StartingTokens int `yaml:"starting_tokens"`
	HintCost       int `yaml:"hint_cost"`
	SkipCost       int `yaml:"skip_cost"`
}

type NPCConfig struct {
	Count int      `yaml:"count"`
	Names []string `yaml:"names"`
}

type TeamsConfig struct {
	Red  TeamConfig `yaml:"red"`
	Blue TeamConfig `yaml:"blue"`
}

type TeamConfig struct {
	Name    string   `yaml:"name"`
	Color   string   `yaml:"color"`
	Members []string `yaml:"members"`
}

type ProviderConfig struct {
	Name    string `yaml:"name"`
	Enabled bool   `yaml:"enabled"`
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
}

type ModelRolesConfig struct {
	Movement   RoleConfig `yaml:"movement"`
	Challenge  RoleConfig `yaml:"challenge"`
	Judge      RoleConfig `yaml:"judge"`
	ZoneGen    RoleConfig `yaml:"zone_generator"`
	Commentary RoleConfig `yaml:"commentary"`
}

type RoleConfig struct {
	Provider    string  `yaml:"provider"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

type ObservabilityConfig struct {
	TraceEnabled  bool   `yaml:"trace_enabled"`
	TracePath     string `yaml:"trace_path"`
	AuditEnabled  bool   `yaml:"audit_enabled"`
	AuditPath     string `yaml:"audit_path"`
	ReplayEnabled bool   `yaml:"replay_enabled"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func Default() *Config {
	return &Config{
		Game: GameConfig{
			TickRate:       60,
			DecisionRate:   2,
			WorldWidth:     1200,
			WorldHeight:    800,
			StartingTokens: 50,
			HintCost:       5,
			SkipCost:       20,
		},
		NPCs: NPCConfig{
			Count: 4,
			Names: []string{"Explorer", "Scout", "Wanderer", "Seeker"},
		},
		Teams: TeamsConfig{
			Red: TeamConfig{
				Name:    "Team Red",
				Color:   "#ef4444",
				Members: []string{"Explorer", "Scout"},
			},
			Blue: TeamConfig{
				Name:    "Team Blue",
				Color:   "#3b82f6",
				Members: []string{"Wanderer", "Seeker"},
			},
		},
		SLMProviders: []ProviderConfig{
			{Name: "groq", Enabled: true, BaseURL: "https://api.groq.com/openai/v1", Model: "llama-3.1-8b-instant"},
		},
		BrainProviders: []ProviderConfig{
			{Name: "gemini", Enabled: true, Model: "gemini-2.0-flash"},
		},
		ModelRoles: ModelRolesConfig{
			Movement:   RoleConfig{Provider: "groq", Model: "llama-3.1-8b-instant", MaxTokens: 50, Temperature: 0.3},
			Challenge:  RoleConfig{Provider: "groq", Model: "llama-3.1-8b-instant", MaxTokens: 200, Temperature: 0.7},
			Judge:      RoleConfig{Provider: "gemini", Model: "gemini-2.0-flash", MaxTokens: 100, Temperature: 0.1},
			ZoneGen:    RoleConfig{Provider: "gemini", Model: "gemini-2.0-flash", MaxTokens: 500, Temperature: 0.9},
			Commentary: RoleConfig{Provider: "groq", Model: "llama-3.1-8b-instant", MaxTokens: 30, Temperature: 0.8},
		},
		Observability: ObservabilityConfig{
			TraceEnabled:  true,
			TracePath:     "./logs/trace.jsonl",
			AuditEnabled:  true,
			AuditPath:     "./logs/audit.log",
			ReplayEnabled: true,
		},
		Server: ServerConfig{Port: 8080},
	}
}
