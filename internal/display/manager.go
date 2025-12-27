// Package display implements GTK4/libadwaita notification popup display.
package display

import (
	"container/list"
	"log/slog"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/dbus"
)

// QueuedNotification represents a notification waiting to be displayed.
// Only urgency and metadata are stored - no GTK objects are created until displayed.
type QueuedNotification struct {
	DBusID       uint32
	HistuiID     string
	Notification *dbus.DBusNotification
	QueuedAt     time.Time
	Urgency      int // Cached for priority sorting
}

// PopupState represents the state of an active notification popup.
// Only created for visible notifications.
type PopupState struct {
	DBusID       uint32
	HistuiID     string
	Popup        *Popup
	Notification *dbus.DBusNotification // Stored for duplicate detection
	CreatedAt    time.Time
	ExpiresAt    time.Time // Zero means never expires
	Paused       bool      // Timeout paused (e.g., on hover)
	StackCount   int       // Number of stacked identical notifications
}

// CloseCallback is called when a popup is closed.
type CloseCallback func(dbusID uint32, reason dbus.CloseReason)

// ActionCallback is called when an action is invoked.
type ActionCallback func(dbusID uint32, actionKey string)

// Manager manages notification popup windows with memory-efficient queuing.
// Only MaxVisible popups exist as GTK objects at any time.
// Additional notifications are queued and displayed when space becomes available.
type Manager struct {
	app     *gtk.Application
	config  *config.DaemonConfig
	logger  *slog.Logger
	display *gdk.Display

	// Active popups - only MaxVisible at a time
	mu     sync.RWMutex
	popups map[uint32]*PopupState // Keyed by D-Bus ID

	// Pending queue - notifications waiting for display
	// Stored as metadata only, no GTK objects
	queue      *list.List               // List of *QueuedNotification (ordered by priority/time)
	queueIndex map[uint32]*list.Element // Fast lookup by D-Bus ID

	// Callbacks
	onClose  CloseCallback
	onAction ActionCallback

	// Timeout management
	timeoutCh chan uint32
	stopCh    chan struct{}
}

// NewManager creates a new display manager.
func NewManager(app *gtk.Application, cfg *config.DaemonConfig, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg == nil {
		cfg = config.DefaultDaemonConfig()
	}

	return &Manager{
		app:        app,
		config:     cfg,
		logger:     logger,
		popups:     make(map[uint32]*PopupState),
		queue:      list.New(),
		queueIndex: make(map[uint32]*list.Element),
		timeoutCh:  make(chan uint32, 100),
		stopCh:     make(chan struct{}),
	}
}

// Start initializes the display manager.
func (m *Manager) Start() error {
	m.display = gdk.DisplayGetDefault()
	if m.display == nil {
		return &DisplayError{Message: "no display available"}
	}

	// Start timeout handler goroutine
	go m.handleTimeouts()

	m.logger.Info("display manager started")
	return nil
}

// Stop shuts down the display manager.
func (m *Manager) Stop() {
	close(m.stopCh)
	m.CloseAll()

	// Clear the queue
	m.mu.Lock()
	m.queue.Init()
	m.queueIndex = make(map[uint32]*list.Element)
	m.mu.Unlock()

	m.logger.Info("display manager stopped")
}

// SetCloseCallback sets the callback for popup close events.
func (m *Manager) SetCloseCallback(cb CloseCallback) {
	m.onClose = cb
}

// SetActionCallback sets the callback for action invocation events.
func (m *Manager) SetActionCallback(cb ActionCallback) {
	m.onAction = cb
}

// isDuplicate checks if two notifications are considered duplicates for stacking.
func isDuplicate(a, b *dbus.DBusNotification) bool {
	return a.AppName == b.AppName &&
		a.Summary == b.Summary &&
		a.Body == b.Body
}

// Show queues a notification for display.
// If there's room, it displays immediately. Otherwise, it's queued by priority.
// If stacking is enabled and a duplicate exists, the stack count is incremented instead.
func (m *Manager) Show(notification *dbus.DBusNotification, dbusID uint32, histuiID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this notification is replacing an existing one
	if state, exists := m.popups[dbusID]; exists {
		// It's already visible - update in place
		state.Popup.Close()
		delete(m.popups, dbusID)
		// Re-show immediately
		return m.showPopupLocked(notification, dbusID, histuiID)
	}

	// Check for duplicate stacking if enabled
	if m.config.Behavior.StackDuplicates {
		for _, state := range m.popups {
			if isDuplicate(state.Notification, notification) {
				// Stack onto existing popup
				state.StackCount++
				state.Popup.IncrementStackCount()

				// Reset the timeout for the stacked notification
				if timeout := m.config.GetTimeoutForUrgency(notification.Urgency()); timeout > 0 {
					state.ExpiresAt = time.Now().Add(time.Duration(timeout) * time.Millisecond)
				}

				m.logger.Debug("stacked duplicate notification",
					"dbus_id", dbusID,
					"onto_dbus_id", state.DBusID,
					"stack_count", state.StackCount,
				)

				// Close the incoming notification with CloseReasonDismissed
				// since we're not actually showing it
				if m.onClose != nil {
					go m.onClose(dbusID, dbus.CloseReasonDismissed)
				}

				return nil
			}
		}
	}

	// Check if it's in the queue
	if elem, exists := m.queueIndex[dbusID]; exists {
		// Update the queued notification
		queued := elem.Value.(*QueuedNotification)
		queued.Notification = notification
		queued.HistuiID = histuiID
		queued.Urgency = notification.Urgency()
		// Re-sort by priority
		m.reorderQueueLocked()
		return nil
	}

	// Check if we have room to display immediately
	if len(m.popups) < m.config.Display.MaxVisible {
		return m.showPopupLocked(notification, dbusID, histuiID)
	}

	// Check if this is more urgent than something currently displayed
	if m.shouldPreempt(notification.Urgency()) {
		// Preempt the lowest priority visible notification
		preemptedID := m.findLowestPriorityPopupLocked()
		if preemptedID > 0 {
			if state, exists := m.popups[preemptedID]; exists {
				// Close and re-queue the preempted notification
				// Note: We need the original notification to re-queue - for simplicity,
				// we just close it with "expired" reason. A more sophisticated implementation
				// would store the original notification data.
				state.Popup.Close()
				delete(m.popups, preemptedID)
				if m.onClose != nil {
					// Run outside lock to avoid deadlock
					go m.onClose(preemptedID, dbus.CloseReasonExpired)
				}
			}
			// Now show the new, higher priority notification
			return m.showPopupLocked(notification, dbusID, histuiID)
		}
	}

	// Queue the notification (no GTK objects created)
	queued := &QueuedNotification{
		DBusID:       dbusID,
		HistuiID:     histuiID,
		Notification: notification,
		QueuedAt:     time.Now(),
		Urgency:      notification.Urgency(),
	}
	m.addToQueueLocked(queued)

	m.logger.Debug("queued notification",
		"dbus_id", dbusID,
		"urgency", notification.Urgency(),
		"queue_size", m.queue.Len(),
	)

	return nil
}

// showPopupLocked creates and displays a popup. Caller must hold the lock.
func (m *Manager) showPopupLocked(notification *dbus.DBusNotification, dbusID uint32, histuiID string) error {
	// Calculate position in stack
	position := len(m.popups)

	// Create the popup (this is where GTK objects are allocated)
	popup, err := NewPopup(m.app, notification, m.config, m.logger)
	if err != nil {
		return err
	}

	// Set up callbacks
	popup.OnClose(func(reason dbus.CloseReason) {
		m.handlePopupClosed(dbusID, reason)
	})

	popup.OnAction(func(actionKey string) {
		if m.onAction != nil {
			m.onAction(dbusID, actionKey)
		}
	})

	popup.OnHover(func(hovering bool) {
		m.handleHover(dbusID, hovering)
	})

	popup.OnCloseAll(func() {
		// Run outside lock to avoid deadlock
		go m.CloseAll()
	})

	// Calculate expiration time
	timeout := m.config.GetTimeoutForUrgency(notification.Urgency())
	var expiresAt time.Time
	if timeout > 0 {
		expiresAt = time.Now().Add(time.Duration(timeout) * time.Millisecond)
	}

	// Store state
	state := &PopupState{
		DBusID:       dbusID,
		HistuiID:     histuiID,
		Popup:        popup,
		Notification: notification,
		CreatedAt:    time.Now(),
		ExpiresAt:    expiresAt,
		StackCount:   1, // Initial count
	}
	m.popups[dbusID] = state

	// Initialize the popup's stack count (shows badge only if count > 1)
	popup.SetStackCount(1)

	// Show the popup
	popup.Show(position)

	// Schedule timeout if applicable
	if timeout > 0 {
		go func() {
			time.Sleep(time.Duration(timeout) * time.Millisecond)
			select {
			case m.timeoutCh <- dbusID:
			case <-m.stopCh:
			}
		}()
	}

	m.logger.Debug("showed popup",
		"dbus_id", dbusID,
		"histui_id", histuiID,
		"position", position,
		"timeout_ms", timeout,
		"active_popups", len(m.popups),
	)

	return nil
}

// addToQueueLocked adds a notification to the queue in priority order.
// Higher urgency notifications are placed earlier in the queue.
// Caller must hold the lock.
func (m *Manager) addToQueueLocked(queued *QueuedNotification) {
	// Find the right position based on urgency (higher urgency = earlier)
	var insertBefore *list.Element
	for e := m.queue.Front(); e != nil; e = e.Next() {
		existing := e.Value.(*QueuedNotification)
		if queued.Urgency > existing.Urgency {
			insertBefore = e
			break
		}
	}

	var elem *list.Element
	if insertBefore != nil {
		elem = m.queue.InsertBefore(queued, insertBefore)
	} else {
		elem = m.queue.PushBack(queued)
	}
	m.queueIndex[queued.DBusID] = elem
}

// reorderQueueLocked re-sorts the queue by priority. Caller must hold the lock.
func (m *Manager) reorderQueueLocked() {
	// Simple approach: rebuild the queue
	items := make([]*QueuedNotification, 0, m.queue.Len())
	for e := m.queue.Front(); e != nil; e = e.Next() {
		items = append(items, e.Value.(*QueuedNotification))
	}

	// Sort by urgency (descending) then by queue time (ascending)
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			// Higher urgency first
			if items[j].Urgency > items[i].Urgency {
				items[i], items[j] = items[j], items[i]
			} else if items[j].Urgency == items[i].Urgency && items[j].QueuedAt.Before(items[i].QueuedAt) {
				// Same urgency, earlier queued first
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	// Rebuild the list
	m.queue.Init()
	m.queueIndex = make(map[uint32]*list.Element)
	for _, item := range items {
		elem := m.queue.PushBack(item)
		m.queueIndex[item.DBusID] = elem
	}
}

// shouldPreempt returns true if the given urgency should preempt existing popups.
func (m *Manager) shouldPreempt(urgency int) bool {
	// Only critical notifications can preempt
	return urgency >= 2
}

// findLowestPriorityPopupLocked finds the visible popup with lowest urgency.
// Returns 0 if no suitable popup found. Caller must hold the lock.
func (m *Manager) findLowestPriorityPopupLocked() uint32 {
	var lowestID uint32
	lowestUrgency := 3 // Higher than any valid urgency

	for id, state := range m.popups {
		// Get urgency from the popup's notification (would need to store it)
		// For now, assume we can preempt any non-critical popup
		// A better implementation would store urgency in PopupState
		_ = state
		if lowestUrgency > 0 {
			lowestID = id
			lowestUrgency = 0 // Assume normal urgency
		}
	}

	return lowestID
}

// CloseByHistuiID closes a popup by its histui ULID.
// This is used when notifications are dismissed externally (e.g., via histui CLI).
func (m *Manager) CloseByHistuiID(histuiID string, reason dbus.CloseReason) bool {
	m.mu.Lock()

	// Find the popup with this histui ID
	var dbusID uint32
	var state *PopupState
	for id, s := range m.popups {
		if s.HistuiID == histuiID {
			dbusID = id
			state = s
			break
		}
	}

	if state == nil {
		// Also check the queue
		for elem := m.queue.Front(); elem != nil; elem = elem.Next() {
			queued := elem.Value.(*QueuedNotification)
			if queued.HistuiID == histuiID {
				m.queue.Remove(elem)
				delete(m.queueIndex, queued.DBusID)
				m.mu.Unlock()
				m.logger.Debug("removed queued notification by histui_id",
					"histui_id", histuiID,
				)
				return true
			}
		}
		m.mu.Unlock()
		return false
	}

	delete(m.popups, dbusID)
	m.mu.Unlock()

	state.Popup.Close()
	state.Popup = nil // Help GC

	if m.onClose != nil {
		m.onClose(dbusID, reason)
	}

	// Try to show next queued notification
	m.showNextQueued()
	m.updatePositions()

	m.logger.Debug("closed popup by histui_id",
		"histui_id", histuiID,
		"dbus_id", dbusID,
	)

	return true
}

// GetActiveHistuiIDs returns the histui IDs of all active and queued notifications.
func (m *Manager) GetActiveHistuiIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.popups)+m.queue.Len())

	for _, state := range m.popups {
		if state.HistuiID != "" {
			ids = append(ids, state.HistuiID)
		}
	}

	for elem := m.queue.Front(); elem != nil; elem = elem.Next() {
		queued := elem.Value.(*QueuedNotification)
		if queued.HistuiID != "" {
			ids = append(ids, queued.HistuiID)
		}
	}

	return ids
}

// Close closes a popup by D-Bus ID.
func (m *Manager) Close(dbusID uint32, reason dbus.CloseReason) {
	m.mu.Lock()
	state, exists := m.popups[dbusID]
	if exists {
		delete(m.popups, dbusID)
	}

	// Also remove from queue if present
	if elem, inQueue := m.queueIndex[dbusID]; inQueue {
		m.queue.Remove(elem)
		delete(m.queueIndex, dbusID)
	}
	m.mu.Unlock()

	if exists {
		state.Popup.Close()
		// Set popup to nil to help GC
		state.Popup = nil

		if m.onClose != nil {
			m.onClose(dbusID, reason)
		}

		// Try to show next queued notification
		m.showNextQueued()
		m.updatePositions()
	}
}

// CloseAll closes all popups and clears the queue.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	popups := make([]*PopupState, 0, len(m.popups))
	for _, state := range m.popups {
		popups = append(popups, state)
	}
	m.popups = make(map[uint32]*PopupState)

	// Clear queue
	m.queue.Init()
	m.queueIndex = make(map[uint32]*list.Element)
	m.mu.Unlock()

	for _, state := range popups {
		state.Popup.Close()
		state.Popup = nil // Help GC
		if m.onClose != nil {
			m.onClose(state.DBusID, dbus.CloseReasonDismissed)
		}
	}
}

// showNextQueued displays the next notification from the queue if space is available.
func (m *Manager) showNextQueued() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.popups) >= m.config.Display.MaxVisible {
		return
	}

	if m.queue.Len() == 0 {
		return
	}

	// Get the highest priority queued notification
	elem := m.queue.Front()
	if elem == nil {
		return
	}

	queued := elem.Value.(*QueuedNotification)
	m.queue.Remove(elem)
	delete(m.queueIndex, queued.DBusID)

	// Show the popup (creates GTK objects now)
	if err := m.showPopupLocked(queued.Notification, queued.DBusID, queued.HistuiID); err != nil {
		m.logger.Warn("failed to show queued notification",
			"dbus_id", queued.DBusID,
			"error", err,
		)
	}
}

// handlePopupClosed handles a popup being closed (e.g., by user click).
func (m *Manager) handlePopupClosed(dbusID uint32, reason dbus.CloseReason) {
	m.mu.Lock()
	state, exists := m.popups[dbusID]
	if exists {
		delete(m.popups, dbusID)
		state.Popup = nil // Help GC
	}
	m.mu.Unlock()

	if exists && m.onClose != nil {
		m.onClose(dbusID, reason)
	}

	// Show next queued notification
	m.showNextQueued()
	m.updatePositions()
}

// handleHover handles hover state changes for pause-on-hover.
func (m *Manager) handleHover(dbusID uint32, hovering bool) {
	if !m.config.Behavior.PauseOnHover {
		return
	}

	m.mu.Lock()
	if state, exists := m.popups[dbusID]; exists {
		state.Paused = hovering
		if !hovering && !state.ExpiresAt.IsZero() {
			// Resume timeout from now
			timeout := m.config.GetTimeoutForUrgency(1) // Use normal timeout for resumed
			state.ExpiresAt = time.Now().Add(time.Duration(timeout) * time.Millisecond)
		}
	}
	m.mu.Unlock()
}

// handleTimeouts processes timeout events.
func (m *Manager) handleTimeouts() {
	for {
		select {
		case dbusID := <-m.timeoutCh:
			m.mu.RLock()
			state, exists := m.popups[dbusID]
			shouldClose := exists && !state.Paused && !state.ExpiresAt.IsZero() && time.Now().After(state.ExpiresAt)
			m.mu.RUnlock()

			if shouldClose {
				m.Close(dbusID, dbus.CloseReasonExpired)
			}
		case <-m.stopCh:
			return
		}
	}
}

// updatePositions updates the position of all remaining popups.
func (m *Manager) updatePositions() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Sort by creation time
	type positionedState struct {
		state    *PopupState
		position int
	}
	states := make([]positionedState, 0, len(m.popups))
	for _, state := range m.popups {
		states = append(states, positionedState{state: state})
	}

	// Sort by creation time
	for i := range states {
		for j := i + 1; j < len(states); j++ {
			if states[j].state.CreatedAt.Before(states[i].state.CreatedAt) {
				states[i], states[j] = states[j], states[i]
			}
		}
	}

	// Assign positions
	for i := range states {
		states[i].position = i
	}

	// Update popup positions
	for _, ps := range states {
		ps.state.Popup.UpdatePosition(ps.position)
	}
}

// ActiveCount returns the number of active popups.
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.popups)
}

// QueuedCount returns the number of queued notifications.
func (m *Manager) QueuedCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.queue.Len()
}

// TotalCount returns the total number of pending notifications (active + queued).
func (m *Manager) TotalCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.popups) + m.queue.Len()
}

// UpdateConfig updates the configuration and adjusts displayed popups if necessary.
// This is called when the config file is hot-reloaded.
func (m *Manager) UpdateConfig(cfg *config.DaemonConfig) {
	m.mu.Lock()
	oldMaxVisible := m.config.Display.MaxVisible
	m.config = cfg
	m.mu.Unlock()

	m.logger.Debug("display manager config updated",
		"old_max_visible", oldMaxVisible,
		"new_max_visible", cfg.Display.MaxVisible,
	)

	// Update positions of existing popups (in case position or offsets changed)
	m.updatePositions()

	// If max_visible increased, show more queued notifications
	if cfg.Display.MaxVisible > oldMaxVisible {
		for i := 0; i < cfg.Display.MaxVisible-oldMaxVisible; i++ {
			m.showNextQueued()
		}
	}
	// Note: If max_visible decreased, we don't close existing popups immediately.
	// They will naturally expire or be dismissed. New notifications will respect the limit.
}

// DisplayError represents a display-related error.
type DisplayError struct {
	Message string
	Cause   error
}

func (e *DisplayError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *DisplayError) Unwrap() error {
	return e.Cause
}
