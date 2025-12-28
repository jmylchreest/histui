// Package display implements GTK4/libadwaita notification popup display.
package display

import (
	"container/list"
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/dbus"
	"github.com/jmylchreest/histui/internal/icon"
	"github.com/jmylchreest/histui/internal/model"
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
	DBusID        uint32
	HistuiID      string
	Popup         *Popup
	Notification  *dbus.DBusNotification // Stored for duplicate detection
	Urgency       int                    // Cached urgency level for preemption comparison
	ClientTimeout int32                  // Client's requested timeout (-1=server decides, 0=never, >0=ms)
	CreatedAt     time.Time
	ExpiresAt     time.Time // Zero means never expires
	Paused        bool      // Timeout paused (e.g., on hover)
	StackCount    int       // Number of stacked identical notifications
	ActualHeight  int       // Measured height after GTK layout (for dynamic stacking)
	ActualWidth   int       // Measured width after GTK layout (for unified stack width)
}

// CloseCallback is called when a popup is closed.
type CloseCallback func(dbusID uint32, reason dbus.CloseReason)

// ActionCallback is called when an action is invoked.
type ActionCallback func(dbusID uint32, actionKey string)

// Manager manages notification popup windows with memory-efficient queuing.
// Uses a single-window approach where all notifications are widgets inside one
// layer-shell window, eliminating gaps between notifications.
// Only MaxVisible popups exist as GTK objects at any time.
// Additional notifications are queued and displayed when space becomes available.
type Manager struct {
	app          *gtk.Application
	config       *config.DaemonConfig
	logger       *slog.Logger
	display      *gdk.Display
	iconResolver *icon.Resolver

	// Icon aliases hot-reloading
	aliasesWatcher *icon.AliasesWatcher

	// Single window containing all notifications
	stack *NotificationStack

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
	stopOnce  sync.Once
}

// NewManager creates a new display manager.
func NewManager(app *gtk.Application, cfg *config.DaemonConfig, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg == nil {
		cfg = config.DefaultDaemonConfig()
	}

	// Create icon resolver with default mappings and load user aliases from file
	iconResolver, err := icon.NewResolverWithAliases()
	if err != nil {
		logger.Warn("failed to load icon aliases file", "error", err)
	}

	m := &Manager{
		app:          app,
		config:       cfg,
		logger:       logger,
		iconResolver: iconResolver,
		popups:       make(map[uint32]*PopupState),
		queue:        list.New(),
		queueIndex:   make(map[uint32]*list.Element),
		timeoutCh:    make(chan uint32, 100),
		stopCh:       make(chan struct{}),
	}

	// Create icon aliases watcher for hot-reloading
	aliasesWatcher, err := icon.NewAliasesWatcher(logger)
	if err != nil {
		logger.Warn("failed to create icon aliases watcher", "error", err)
	} else {
		m.aliasesWatcher = aliasesWatcher
		// Set callback to reload aliases into the resolver
		aliasesWatcher.SetReloadCallback(func(aliases map[string]string) {
			iconResolver.SetUserAliases(aliases)
		})
		aliasesWatcher.SetErrorCallback(func(err error) {
			logger.Warn("failed to reload icon aliases", "error", err)
		})
	}

	// Apply color scheme preference
	ApplyColorScheme(cfg.Theme.ColorScheme, logger)

	// Create the single notification stack window
	m.stack = NewNotificationStack(app, cfg, logger)

	return m
}

// ApplyColorScheme sets the libadwaita color scheme based on config.
// Exported for use in hot-reload callbacks.
func ApplyColorScheme(scheme string, logger *slog.Logger) {
	styleManager := adw.StyleManagerGetDefault()
	if styleManager == nil {
		return
	}

	switch config.ColorScheme(scheme) {
	case config.ColorSchemeLight:
		styleManager.SetColorScheme(adw.ColorSchemeForceLight)
		logger.Debug("applied color scheme", "scheme", "light")
	case config.ColorSchemeDark:
		styleManager.SetColorScheme(adw.ColorSchemeForceDark)
		logger.Debug("applied color scheme", "scheme", "dark")
	default:
		styleManager.SetColorScheme(adw.ColorSchemeDefault)
		logger.Debug("applied color scheme", "scheme", "system")
	}
}

// Start initializes the display manager.
func (m *Manager) Start(ctx context.Context) error {
	m.display = gdk.DisplayGetDefault()
	if m.display == nil {
		return &DisplayError{Message: "no display available"}
	}

	// Start icon aliases watcher for hot-reloading
	if m.aliasesWatcher != nil {
		if err := m.aliasesWatcher.Start(ctx); err != nil {
			m.logger.Warn("failed to start icon aliases watcher", "error", err)
		}
	}

	// Start timeout handler goroutine
	go m.handleTimeouts()

	m.logger.Info("display manager started")
	return nil
}

// Stop shuts down the display manager.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
		m.CloseAll(dbus.CloseReasonUndefined)

		// Stop icon aliases watcher
		if m.aliasesWatcher != nil {
			m.aliasesWatcher.Stop()
		}

		// Clear the queue
		m.mu.Lock()
		m.queue.Init()
		m.queueIndex = make(map[uint32]*list.Element)
		m.mu.Unlock()

		// Destroy the notification stack window
		if m.stack != nil {
			m.stack.Destroy()
		}

		m.logger.Info("display manager stopped")
	})
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
// Stack tag matching: notifications with the same stack tag replace each other.
// If stacking is enabled and a duplicate exists, the stack count is incremented instead.
func (m *Manager) Show(notification *dbus.DBusNotification, dbusID uint32, histuiID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this notification is replacing an existing one via replaces_id
	if state, exists := m.popups[dbusID]; exists {
		// It's already visible - remove old and re-show
		m.stack.Remove(dbusID)
		state.Popup.Close()
		delete(m.popups, dbusID)
		// Re-show immediately
		return m.showPopupLocked(notification, dbusID, histuiID)
	}

	// Check for stack tag replacement (dunst-compatible)
	// Notifications with the same stack tag replace each other in place
	stackTag := notification.StackTag()
	if stackTag != "" {
		for existingID, state := range m.popups {
			existingTag := state.Notification.StackTag()
			// Match on stack tag AND app name (for safety)
			if existingTag == stackTag && state.Notification.AppName == notification.AppName {
				// Update the existing popup's content in place
				state.Notification = notification
				state.Popup.UpdateContent(notification)

				// Reset the timeout
				if timeout := m.config.GetTimeoutForUrgency(notification.Urgency(), notification.ExpireTimeout); timeout > 0 {
					state.ExpiresAt = time.Now().Add(time.Duration(timeout) * time.Millisecond)
				}

				m.logger.Debug("replaced notification via stack tag",
					"stack_tag", stackTag,
					"new_dbus_id", dbusID,
					"existing_dbus_id", existingID,
				)

				// Close the incoming notification immediately since we're reusing the existing one
				if m.onClose != nil {
					go m.onClose(dbusID, dbus.CloseReasonDismissed)
				}

				return nil
			}
		}
	}

	// Check for duplicate stacking if enabled
	if m.config.Behavior.StackDuplicates {
		for _, state := range m.popups {
			if isDuplicate(state.Notification, notification) {
				// Stack onto existing popup
				state.StackCount++
				state.Popup.IncrementStackCount()

				// Reset the timeout for the stacked notification
				if timeout := m.config.GetTimeoutForUrgency(notification.Urgency(), notification.ExpireTimeout); timeout > 0 {
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
				m.stack.Remove(preemptedID)
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

// showPopupLocked creates and displays a popup widget. Caller must hold the lock.
// Uses single-window mode where notifications are widgets inside a shared container.
func (m *Manager) showPopupLocked(notification *dbus.DBusNotification, dbusID uint32, histuiID string) error {
	// Create the popup widget (no window - will be added to stack container)
	popup, err := NewPopupWidget(notification, m.config, m.logger, m.iconResolver)
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
		go m.CloseAll(dbus.CloseReasonDismissed)
	})

	// Calculate expiration time
	clientTimeout := notification.ExpireTimeout
	timeout := m.config.GetTimeoutForUrgency(notification.Urgency(), clientTimeout)
	var expiresAt time.Time
	if timeout > 0 {
		expiresAt = time.Now().Add(time.Duration(timeout) * time.Millisecond)
	}

	// Store state
	state := &PopupState{
		DBusID:        dbusID,
		HistuiID:      histuiID,
		Popup:         popup,
		Notification:  notification,
		Urgency:       notification.Urgency(),
		ClientTimeout: clientTimeout,
		CreatedAt:     time.Now(),
		ExpiresAt:     expiresAt,
		StackCount:    1,
	}
	m.popups[dbusID] = state

	// Initialize the popup's stack count (shows badge only if count > 1)
	popup.SetStackCount(1)

	// Add the widget to the notification stack
	widget := &NotificationWidget{
		DBusID:    dbusID,
		HistuiID:  histuiID,
		Container: popup.Widget(),
		Popup:     popup,
	}
	m.stack.Add(widget)

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

	m.logger.Debug("showed popup widget",
		"dbus_id", dbusID,
		"histui_id", histuiID,
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
	return urgency >= model.UrgencyCritical
}

// findLowestPriorityPopupLocked finds the visible popup with lowest urgency.
// Critical notifications cannot be preempted.
// Among equal urgency, the oldest notification is selected.
// Returns 0 if no suitable popup found. Caller must hold the lock.
func (m *Manager) findLowestPriorityPopupLocked() uint32 {
	var lowestID uint32
	lowestUrgency := model.UrgencyCritical + 1 // Higher than any valid urgency
	var oldestTime time.Time

	for id, state := range m.popups {
		// Critical notifications cannot be preempted
		if state.Urgency >= model.UrgencyCritical {
			continue
		}

		// Find lowest urgency, or oldest if same urgency
		if state.Urgency < lowestUrgency ||
			(state.Urgency == lowestUrgency && (oldestTime.IsZero() || state.CreatedAt.Before(oldestTime))) {
			lowestID = id
			lowestUrgency = state.Urgency
			oldestTime = state.CreatedAt
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

	// Remove from notification stack
	m.stack.RemoveByHistuiID(histuiID)

	state.Popup.Close()
	state.Popup = nil // Help GC

	if m.onClose != nil {
		m.onClose(dbusID, reason)
	}

	// Try to show next queued notification
	m.showNextQueued()

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
		// Remove from notification stack
		m.stack.Remove(dbusID)

		state.Popup.Close()
		// Set popup to nil to help GC
		state.Popup = nil

		if m.onClose != nil {
			m.onClose(dbusID, reason)
		}

		// Try to show next queued notification
		m.showNextQueued()
	}
}

// CloseAll closes all popups and clears the queue.
// Returns the number of popups closed.
func (m *Manager) CloseAll(reason dbus.CloseReason) int {
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

	// Clear the notification stack (removes all widgets from container)
	m.stack.Clear()

	for _, state := range popups {
		state.Popup.Close()
		state.Popup = nil // Help GC
		if m.onClose != nil {
			m.onClose(state.DBusID, reason)
		}
	}

	return len(popups)
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

	// Remove from notification stack
	m.stack.Remove(dbusID)

	if exists && m.onClose != nil {
		m.onClose(dbusID, reason)
	}

	// Show next queued notification
	m.showNextQueued()
}

// handleHover handles hover state changes for pause-on-hover.
// When ANY notification in the stack is hovered, ALL notifications are paused.
func (m *Manager) handleHover(_ uint32, hovering bool) {
	if !m.config.Behavior.PauseOnHover {
		return
	}

	m.mu.Lock()

	// Collect notifications that need new timeout goroutines
	var needsReschedule []struct {
		dbusID  uint32
		timeout int
	}

	// Pause or unpause ALL notifications in the stack
	for id, state := range m.popups {
		state.Paused = hovering
		if !hovering && !state.ExpiresAt.IsZero() {
			// Resume timeout from now - use each notification's own urgency and client timeout
			timeout := m.config.GetTimeoutForUrgency(state.Urgency, state.ClientTimeout)
			if timeout > 0 {
				state.ExpiresAt = time.Now().Add(time.Duration(timeout) * time.Millisecond)
				needsReschedule = append(needsReschedule, struct {
					dbusID  uint32
					timeout int
				}{id, timeout})
			}
		}
	}

	m.logger.Debug("hover state changed for stack",
		"hovering", hovering,
		"affected_popups", len(m.popups),
	)

	m.mu.Unlock()

	// Schedule new timeout goroutines for resumed notifications (outside lock)
	for _, item := range needsReschedule {
		id := item.dbusID
		timeout := item.timeout
		go func() {
			time.Sleep(time.Duration(timeout) * time.Millisecond)
			select {
			case m.timeoutCh <- id:
			case <-m.stopCh:
			}
		}()
	}
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

	// Update stack configuration (position, offsets)
	m.stack.UpdateConfig(cfg)

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
