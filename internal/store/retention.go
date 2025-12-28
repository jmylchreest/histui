package store

import (
	"log/slog"
	"sync"
	"time"
)

const (
	// DefaultDebounceInterval is the minimum time between prune operations.
	DefaultDebounceInterval = 5 * time.Minute

	// DefaultBufferPercent is the percentage over max before pruning triggers.
	// With 10%, if max is 1000, we only prune when count exceeds 1100.
	DefaultBufferPercent = 10
)

// RetentionManager handles automatic pruning of notifications and cascading cleanup.
type RetentionManager struct {
	mu sync.Mutex

	persistence  Persistence
	imageStore   *ImageStore
	maxNotify    int           // Maximum notifications to keep (0 = unlimited)
	debounce     time.Duration // Minimum time between prunes
	bufferPct    int           // Buffer percentage before triggering prune
	lastPrune    time.Time     // Last prune time
	pendingPrune bool          // True if a prune is pending (debounced)
	logger       *slog.Logger
	closed       bool
}

// RetentionConfig configures the retention manager.
type RetentionConfig struct {
	MaxNotifications int           // 0 = unlimited
	DebounceInterval time.Duration // Time between prune operations
	BufferPercent    int           // % over max before pruning
}

// NewRetentionManager creates a new retention manager.
func NewRetentionManager(persistence Persistence, imageStore *ImageStore, cfg RetentionConfig, logger *slog.Logger) *RetentionManager {
	debounce := cfg.DebounceInterval
	if debounce == 0 {
		debounce = DefaultDebounceInterval
	}

	bufferPct := cfg.BufferPercent
	if bufferPct == 0 {
		bufferPct = DefaultBufferPercent
	}

	return &RetentionManager{
		persistence: persistence,
		imageStore:  imageStore,
		maxNotify:   cfg.MaxNotifications,
		debounce:    debounce,
		bufferPct:   bufferPct,
		logger:      logger,
	}
}

// PruneOnStartup runs pruning at daemon startup.
// This always runs regardless of debounce timer.
func (m *RetentionManager) PruneOnStartup() error {
	return m.pruneNow(true)
}

// TriggerPrune requests a prune operation, respecting debounce.
// Call this after adding new notifications.
func (m *RetentionManager) TriggerPrune() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed || m.maxNotify <= 0 {
		return // Unlimited, no pruning
	}

	// Check if we're within debounce window
	if time.Since(m.lastPrune) < m.debounce {
		m.pendingPrune = true
		return
	}

	// Run prune in background
	go func() {
		if err := m.pruneNow(false); err != nil && m.logger != nil {
			m.logger.Error("failed to prune notifications", "error", err)
		}
	}()
}

// pruneNow performs the actual prune operation.
// If force is true, ignores the buffer threshold.
func (m *RetentionManager) pruneNow(force bool) error {
	m.mu.Lock()
	if m.closed || m.maxNotify <= 0 {
		m.mu.Unlock()
		return nil
	}

	m.lastPrune = time.Now()
	m.pendingPrune = false
	maxNotify := m.maxNotify
	bufferPct := m.bufferPct
	m.mu.Unlock()

	// Calculate threshold with buffer
	threshold := maxNotify
	if !force {
		threshold = maxNotify + (maxNotify * bufferPct / 100)
	}

	// Prune from persistence
	prunedIDs, err := m.persistence.Prune(maxNotify)
	if err != nil {
		return err
	}

	if len(prunedIDs) == 0 {
		return nil
	}

	if m.logger != nil {
		m.logger.Info("pruned old notifications",
			"count", len(prunedIDs),
			"max", maxNotify,
			"threshold", threshold,
		)
	}

	// Cascade cleanup: delete images for pruned notifications
	if m.imageStore != nil {
		deleted, err := m.imageStore.DeleteByIDs(prunedIDs)
		if err != nil {
			if m.logger != nil {
				m.logger.Warn("failed to cleanup some images", "error", err)
			}
		} else if deleted > 0 && m.logger != nil {
			m.logger.Debug("cleaned up images for pruned notifications", "count", deleted)
		}
	}

	return nil
}

// RunPendingPrune runs any pending prune that was debounced.
// Call this periodically (e.g., on a timer or at shutdown).
func (m *RetentionManager) RunPendingPrune() error {
	m.mu.Lock()
	pending := m.pendingPrune
	m.mu.Unlock()

	if !pending {
		return nil
	}

	return m.pruneNow(false)
}

// SetMaxNotifications updates the maximum notification count.
func (m *RetentionManager) SetMaxNotifications(max int) {
	m.mu.Lock()
	m.maxNotify = max
	m.mu.Unlock()
}

// Close stops the retention manager.
func (m *RetentionManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}
	m.closed = true

	// Run any pending prune before closing
	if m.pendingPrune && m.maxNotify > 0 {
		// Best effort, don't hold up shutdown
		go func() {
			if err := m.pruneNow(false); err != nil && m.logger != nil {
				m.logger.Debug("failed to run pending prune on close", "error", err)
			}
		}()
	}

	return nil
}
