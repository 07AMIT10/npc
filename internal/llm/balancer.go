package llm

import (
	"sync"
)

// Balancer implements weighted round-robin load balancing.
// Uses nginx-style algorithm where weight determines request distribution.
type Balancer struct {
	providers []weightedProvider
	mu        sync.Mutex

	// Weighted round-robin state
	currentWeight int
	maxWeight     int
	gcd           int
	lastIndex     int
}

type weightedProvider struct {
	provider Provider
	weight   int
}

// NewBalancer creates a balancer from provider configs
func NewBalancer(providers []Provider, weights map[string]int) *Balancer {
	b := &Balancer{
		providers: make([]weightedProvider, 0, len(providers)),
		lastIndex: -1,
	}

	for _, p := range providers {
		weight := weights[p.Name()]
		if weight <= 0 {
			weight = 1 // Default weight
		}
		b.providers = append(b.providers, weightedProvider{
			provider: p,
			weight:   weight,
		})
	}

	// Calculate GCD and max weight for nginx-style algorithm
	if len(b.providers) > 0 {
		b.maxWeight = b.providers[0].weight
		b.gcd = b.providers[0].weight

		for i := 1; i < len(b.providers); i++ {
			w := b.providers[i].weight
			if w > b.maxWeight {
				b.maxWeight = w
			}
			b.gcd = gcd(b.gcd, w)
		}
	}

	return b
}

// Next returns the next provider using weighted round-robin.
// Algorithm: nginx-style smooth weighted round-robin
func (b *Balancer) Next() Provider {
	if len(b.providers) == 0 {
		return nil
	}
	if len(b.providers) == 1 {
		return b.providers[0].provider
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Nginx-style weighted round-robin
	for {
		b.lastIndex = (b.lastIndex + 1) % len(b.providers)

		if b.lastIndex == 0 {
			b.currentWeight -= b.gcd
			if b.currentWeight <= 0 {
				b.currentWeight = b.maxWeight
			}
		}

		if b.providers[b.lastIndex].weight >= b.currentWeight {
			return b.providers[b.lastIndex].provider
		}
	}
}

// GetAll returns all registered providers
func (b *Balancer) GetAll() []Provider {
	result := make([]Provider, len(b.providers))
	for i, wp := range b.providers {
		result[i] = wp.provider
	}
	return result
}

// GetByName returns a specific provider by name
func (b *Balancer) GetByName(name string) Provider {
	for _, wp := range b.providers {
		if wp.provider.Name() == name {
			return wp.provider
		}
	}
	return nil
}

// Len returns the number of providers
func (b *Balancer) Len() int {
	return len(b.providers)
}

// gcd calculates greatest common divisor using Euclidean algorithm
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
