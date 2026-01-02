// Package daemon provides the main orchestration for histuid.
package daemon

import (
	"sync"
	"time"
)

// DisplayStatus represents the status of a notification in the display system.
type DisplayStatus int

const (
	// DisplayStatusPending means the notification is queued for display.
	DisplayStatusPending DisplayStatus = iota
	// DisplayStatusActive means the notification is currently displayed.
	DisplayStatusActive
	// DisplayStatusExpired means the notification timed out.
	DisplayStatusExpired
	// DisplayStatusDismissed means the user dismissed the notification.
	DisplayStatusDismissed
	// DisplayStatusClosed means the notification was closed programmatically.
	DisplayStatusClosed
)

// DisplayState tracks the state of a notification in the display system.
// This maps between the histui ULID and the D-Bus notification ID.
type DisplayState struct {
	HistuiID  string        // The histui ULID
	DBusID    uint32        // The D-Bus notification ID
	Status    DisplayStatus // Current display status
	CreatedAt time.Time     // When the notification was received
	ExpiresAt time.Time     // When the popup should timeout (zero = never)
	ClosedAt  time.Time     // When the notification was closed
}

// DisplayStateManager manages the mapping between histui IDs and display state.
type DisplayStateManager struct {
	mu sync.RWMutex

	// Map histui ID to display state
	byHistuiID map[string]*DisplayState

	// Map D-Bus ID to histui ID (for reverse lookup)
	byDBusID map[uint32]string
}

// NewDisplayStateManager creates a new DisplayStateManager.
func NewDisplayStateManager() *DisplayStateManager {
	return &DisplayStateManager{
		byHistuiID: make(map[string]*DisplayState),
		byDBusID:   make(map[uint32]string),
	}
}

// Register adds a new display state for a notification.
func (m *DisplayStateManager) Register(histuiID string, dbusID uint32, expiresAt time.Time) *DisplayState {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already exists (replacement)
	if old, exists := m.byHistuiID[histuiID]; exists {
		// Clean up old D-Bus ID mapping
		delete(m.byDBusID, old.DBusID)
	}

	state := &DisplayState{
		HistuiID:  histuiID,
		DBusID:    dbusID,
		Status:    DisplayStatusActive,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	m.byHistuiID[histuiID] = state
	m.byDBusID[dbusID] = histuiID

	return state
}

// RemoveByDBusID removes a display state entry by D-Bus ID.
func (m *DisplayStateManager) RemoveByDBusID(dbusID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	histuiID, exists := m.byDBusID[dbusID]
	if !exists {
		return
	}

	delete(m.byDBusID, dbusID)
	delete(m.byHistuiID, histuiID)
}

// RemoveByHistuiID removes a display state entry by histui ID.
func (m *DisplayStateManager) RemoveByHistuiID(histuiID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.byHistuiID[histuiID]
	if !exists {
		return
	}

	delete(m.byHistuiID, histuiID)
	delete(m.byDBusID, state.DBusID)
}
