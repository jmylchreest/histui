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

// String returns the string representation of DisplayStatus.
func (s DisplayStatus) String() string {
	switch s {
	case DisplayStatusPending:
		return "pending"
	case DisplayStatusActive:
		return "active"
	case DisplayStatusExpired:
		return "expired"
	case DisplayStatusDismissed:
		return "dismissed"
	case DisplayStatusClosed:
		return "closed"
	default:
		return "unknown"
	}
}

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

// GetByHistuiID returns the display state for a histui ID.
func (m *DisplayStateManager) GetByHistuiID(histuiID string) *DisplayState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byHistuiID[histuiID]
}

// GetByDBusID returns the display state for a D-Bus ID.
func (m *DisplayStateManager) GetByDBusID(dbusID uint32) *DisplayState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	histuiID, exists := m.byDBusID[dbusID]
	if !exists {
		return nil
	}
	return m.byHistuiID[histuiID]
}

// GetHistuiIDByDBusID returns the histui ID for a D-Bus ID.
func (m *DisplayStateManager) GetHistuiIDByDBusID(dbusID uint32) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byDBusID[dbusID]
}

// GetDBusIDByHistuiID returns the D-Bus ID for a histui ID.
func (m *DisplayStateManager) GetDBusIDByHistuiID(histuiID string) (uint32, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.byHistuiID[histuiID]
	if !exists {
		return 0, false
	}
	return state.DBusID, true
}

// SetStatus updates the status of a notification.
func (m *DisplayStateManager) SetStatus(histuiID string, status DisplayStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.byHistuiID[histuiID]
	if !exists {
		return
	}

	state.Status = status
	if status == DisplayStatusDismissed || status == DisplayStatusExpired || status == DisplayStatusClosed {
		state.ClosedAt = time.Now()
	}
}

// SetStatusByDBusID updates the status by D-Bus ID.
func (m *DisplayStateManager) SetStatusByDBusID(dbusID uint32, status DisplayStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	histuiID, exists := m.byDBusID[dbusID]
	if !exists {
		return
	}

	state, exists := m.byHistuiID[histuiID]
	if !exists {
		return
	}

	state.Status = status
	if status == DisplayStatusDismissed || status == DisplayStatusExpired || status == DisplayStatusClosed {
		state.ClosedAt = time.Now()
	}
}

// Remove removes a display state entry.
func (m *DisplayStateManager) Remove(histuiID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.byHistuiID[histuiID]
	if !exists {
		return
	}

	delete(m.byDBusID, state.DBusID)
	delete(m.byHistuiID, histuiID)
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

// ActiveNotifications returns all currently active notification histui IDs.
func (m *DisplayStateManager) ActiveNotifications() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []string
	for id, state := range m.byHistuiID {
		if state.Status == DisplayStatusActive {
			active = append(active, id)
		}
	}
	return active
}

// Count returns the number of tracked notifications.
func (m *DisplayStateManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.byHistuiID)
}

// ActiveCount returns the number of active notifications.
func (m *DisplayStateManager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, state := range m.byHistuiID {
		if state.Status == DisplayStatusActive {
			count++
		}
	}
	return count
}
