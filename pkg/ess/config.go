package ess

import (
	"sync"

	"github.com/jameshartig/raterudder/pkg/types"
)

// Configured sets up the ESS system provider and defaults to a configured
// franklin system.
func Configured() *Map {
	p := NewMap()
	p.SetSystem(types.SiteIDNone, configuredFranklin())
	return p
}

// Map manages multiple ESS systems.
type Map struct {
	mu      sync.Mutex
	systems map[string]System
}

// NewMap creates a new ESS Map.
func NewMap() *Map {
	return &Map{
		systems: make(map[string]System),
	}
}

// Site returns the system for the given siteID.
// If the siteID is new, it creates a new system instance.
func (m *Map) Site(siteID string) System {
	m.mu.Lock()
	defer m.mu.Unlock()

	if siteID == "" {
		siteID = types.SiteIDNone
	}

	if sys, ok := m.systems[siteID]; ok {
		return sys
	}

	// TODO: we should check what kind of system this is
	m.systems[siteID] = newFranklin()
	return m.systems[siteID]
}

// SetSystem sets the system for a specific site. This is primarily used for testing.
func (m *Map) SetSystem(siteID string, sys System) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.systems[siteID] = sys
}
