package utility

import (
	"fmt"
	"sync"
)

// Configured sets up the utility provider based on flags.
func Configured() *Map {
	m := NewMap()
	m.SetProvider("comed_hourly", configuredComEd())
	return m
}

// Map manages multiple utility providers.
type Map struct {
	mu        sync.Mutex
	providers map[string]Provider
}

// NewMap creates a new ESS Map.
func NewMap() *Map {
	return &Map{
		providers: make(map[string]Provider),
	}
}

// Provider returns the provider for the given name.
func (m *Map) Provider(name string) (Provider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if prov, ok := m.providers[name]; ok {
		return prov, nil
	}
	return nil, fmt.Errorf("unknown utility provider: %s", name)
}

// SetProvider sets the provider for the given name. This is primarily used for testing.
func (m *Map) SetProvider(name string, provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[name] = provider
}
