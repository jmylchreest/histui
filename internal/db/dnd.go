package db

import (
	"sync"
)

// DnDChangeCallback is called when DnD state changes.
type DnDChangeCallback func(enabled bool)

// DnDManager manages Do Not Disturb state in memory.
// State is controlled via D-Bus and does not persist across daemon restarts.
type DnDManager struct {
	mu             sync.RWMutex
	enabled        bool
	changeCallback DnDChangeCallback
}

// NewDnDManager creates a new DnD manager with DnD disabled by default.
func NewDnDManager() *DnDManager {
	return &DnDManager{
		enabled: false,
	}
}

// SetChangeCallback sets a callback to be called when DnD state changes.
func (m *DnDManager) SetChangeCallback(cb DnDChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.changeCallback = cb
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
	changed := m.enabled != enabled
	m.enabled = enabled
	cb := m.changeCallback
	m.mu.Unlock()

	// Call callback outside lock to avoid deadlocks
	if changed && cb != nil {
		cb(enabled)
	}
	return nil
}

// Toggle toggles the DnD state and returns the new state.
func (m *DnDManager) Toggle() (bool, error) {
	m.mu.Lock()
	m.enabled = !m.enabled
	newState := m.enabled
	cb := m.changeCallback
	m.mu.Unlock()

	// Call callback outside lock to avoid deadlocks
	if cb != nil {
		cb(newState)
	}
	return newState, nil
}
