package llm

import (
	"context"
	"testing"
)

// mockProvider for testing
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string                          { return m.name }
func (m *mockProvider) Protocol() Protocol                    { return ProtocolOpenAI }
func (m *mockProvider) HealthCheck(ctx context.Context) error { return nil }
func (m *mockProvider) Complete(ctx context.Context, prompt string, opts CompletionOpts) (*CompletionResult, error) {
	return &CompletionResult{Content: "mock", Provider: m.name}, nil
}

func TestBalancer_WeightedRoundRobin(t *testing.T) {
	// Create 3 providers with weights 3, 2, 1
	providers := []Provider{
		&mockProvider{name: "heavy"},  // weight 3
		&mockProvider{name: "medium"}, // weight 2
		&mockProvider{name: "light"},  // weight 1
	}
	weights := map[string]int{
		"heavy":  3,
		"medium": 2,
		"light":  1,
	}

	b := NewBalancer(providers, weights)

	// Track selections over 60 calls (should see ratio ~3:2:1)
	counts := make(map[string]int)
	for i := 0; i < 60; i++ {
		p := b.Next()
		counts[p.Name()]++
	}

	// Check approximate distribution
	// Weight 3 should get ~30, weight 2 ~20, weight 1 ~10
	if counts["heavy"] < 25 || counts["heavy"] > 35 {
		t.Errorf("heavy got %d calls, expected ~30", counts["heavy"])
	}
	if counts["medium"] < 15 || counts["medium"] > 25 {
		t.Errorf("medium got %d calls, expected ~20", counts["medium"])
	}
	if counts["light"] < 5 || counts["light"] > 15 {
		t.Errorf("light got %d calls, expected ~10", counts["light"])
	}

	t.Logf("Distribution: heavy=%d, medium=%d, light=%d", counts["heavy"], counts["medium"], counts["light"])
}

func TestBalancer_SingleProvider(t *testing.T) {
	providers := []Provider{&mockProvider{name: "only"}}
	weights := map[string]int{"only": 1}

	b := NewBalancer(providers, weights)

	for i := 0; i < 10; i++ {
		p := b.Next()
		if p.Name() != "only" {
			t.Errorf("expected 'only', got '%s'", p.Name())
		}
	}
}

func TestBalancer_EmptyProviders(t *testing.T) {
	b := NewBalancer(nil, nil)
	p := b.Next()
	if p != nil {
		t.Error("expected nil from empty balancer")
	}
}

func TestBalancer_GetByName(t *testing.T) {
	providers := []Provider{
		&mockProvider{name: "groq"},
		&mockProvider{name: "gemini"},
	}
	weights := map[string]int{"groq": 1, "gemini": 1}

	b := NewBalancer(providers, weights)

	if p := b.GetByName("groq"); p == nil || p.Name() != "groq" {
		t.Error("failed to get groq provider")
	}
	if p := b.GetByName("unknown"); p != nil {
		t.Error("expected nil for unknown provider")
	}
}
