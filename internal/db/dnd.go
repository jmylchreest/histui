package db

import (
	"sync"
)

// DnDManager manages Do Not Disturb state in memory.
// State is controlled via IPC and does not persist across daemon restarts.
type DnDManager struct {
	mu      sync.RWMutex
	enabled bool
}

// NewDnDManager creates a new DnD manager with DnD disabled by default.
func NewDnDManager() *DnDManager {
	return &DnDManager{
		enabled: false,
	}
}

// Enabled returns the current DnD state.
func (m *DnDManager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// SetEnabled sets the DnD state.
func (m *DnDManager) SetEnabled(enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
	return nil
}

// Toggle toggles the DnD state and returns the new state.
func (m *DnDManager) Toggle() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = !m.enabled
	return m.enabled, nil
}
