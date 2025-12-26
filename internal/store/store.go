// Package store provides the history store for notifications.
package store

import (
	"sort"
	"sync"
	"time"

	"github.com/jmylchreest/histui/internal/model"
)

// ChangeType indicates the type of store change.
type ChangeType int

const (
	// ChangeTypeAdd indicates notifications were added.
	ChangeTypeAdd ChangeType = iota
	// ChangeTypeClear indicates all notifications were cleared.
	ChangeTypeClear
	// ChangeTypePrune indicates notifications were pruned.
	ChangeTypePrune
	// ChangeTypeDelete indicates a notification was deleted.
	ChangeTypeDelete
)

// ChangeEvent signals store content changes.
type ChangeEvent struct {
	Type   ChangeType
	Count  int
	Source string
}

// FilterOptions specifies criteria for filtering notifications.
type FilterOptions struct {
	Since     time.Duration // Filter to notifications newer than now-since (0=all)
	AppFilter string        // Exact match on app name
	Urgency   *int          // Filter by urgency level (nil=any)
	Limit     int           // Maximum results (0=unlimited)
	SortField string        // Field to sort by: "timestamp", "app", "urgency"
	SortOrder string        // "asc" or "desc" (default: "desc")
}

// Store manages the notification history with thread-safe operations.
type Store struct {
	mu            sync.RWMutex
	notifications []model.Notification
	index         map[string]int    // histui_id -> slice index
	hashIndex     map[string]int    // content_hash -> slice index (for deduplication)
	tombstones    map[string]bool   // content_hash -> true (for deleted items)

	persistence Persistence
	persistPath string

	subscribers []chan ChangeEvent
	closed      bool
}

// NewStore creates a new Store.
// If persistence is not nil, it will be used to persist notifications.
func NewStore(persistence Persistence) *Store {
	return &Store{
		notifications: make([]model.Notification, 0),
		index:         make(map[string]int),
		hashIndex:     make(map[string]int),
		tombstones:    make(map[string]bool),
		persistence:   persistence,
		subscribers:   make([]chan ChangeEvent, 0),
	}
}

// Add adds a single notification to the store.
func (s *Store) Add(n model.Notification) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	// Ensure content hash is computed for deduplication
	n.EnsureContentHash()

	// Check if this was previously deleted (tombstone)
	if s.tombstones[n.ContentHash] {
		return nil // Was deleted, don't reimport
	}

	// Check for duplicates by content hash (primary deduplication)
	if _, exists := s.hashIndex[n.ContentHash]; exists {
		return nil // Duplicate content, skip
	}

	// Also check by ULID (for safety)
	if _, exists := s.index[n.HistuiID]; exists {
		return nil // Already exists, skip
	}

	// Add to slice and indices
	idx := len(s.notifications)
	s.notifications = append(s.notifications, n)
	s.index[n.HistuiID] = idx
	s.hashIndex[n.ContentHash] = idx

	// Persist if enabled
	if s.persistence != nil {
		if err := s.persistence.Append(n); err != nil {
			return err
		}
	}

	// Notify subscribers
	s.notifyChange(ChangeEvent{
		Type:   ChangeTypeAdd,
		Count:  1,
		Source: n.HistuiSource,
	})

	return nil
}

// AddBatch adds multiple notifications efficiently.
func (s *Store) AddBatch(ns []model.Notification) error {
	if len(ns) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	// Filter out duplicates by content hash
	toAdd := make([]model.Notification, 0, len(ns))
	seenHashes := make(map[string]bool) // Track hashes within this batch too

	for i := range ns {
		// Ensure content hash is computed
		ns[i].EnsureContentHash()
		hash := ns[i].ContentHash

		// Skip if this was previously deleted (tombstone)
		if s.tombstones[hash] {
			continue
		}

		// Skip if already in store (by content hash)
		if _, exists := s.hashIndex[hash]; exists {
			continue
		}

		// Skip if already seen in this batch
		if seenHashes[hash] {
			continue
		}

		// Skip if already in store (by ULID, for safety)
		if _, exists := s.index[ns[i].HistuiID]; exists {
			continue
		}

		seenHashes[hash] = true
		toAdd = append(toAdd, ns[i])
	}

	if len(toAdd) == 0 {
		return nil
	}

	// Add to slice and update indices
	startIdx := len(s.notifications)
	s.notifications = append(s.notifications, toAdd...)
	for i, n := range toAdd {
		idx := startIdx + i
		s.index[n.HistuiID] = idx
		s.hashIndex[n.ContentHash] = idx
	}

	// Persist if enabled
	if s.persistence != nil {
		if err := s.persistence.AppendBatch(toAdd); err != nil {
			return err
		}
	}

	// Notify subscribers
	source := ""
	if len(toAdd) > 0 {
		source = toAdd[0].HistuiSource
	}
	s.notifyChange(ChangeEvent{
		Type:   ChangeTypeAdd,
		Count:  len(toAdd),
		Source: source,
	})

	return nil
}

// All returns all notifications sorted by timestamp (newest first by default).
func (s *Store) All() []model.Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return copy sorted by timestamp desc
	result := make([]model.Notification, len(s.notifications))
	copy(result, s.notifications)

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp > result[j].Timestamp
	})

	return result
}

// Filter returns notifications matching the criteria.
func (s *Store) Filter(opts FilterOptions) []model.Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var result []model.Notification

	for _, n := range s.notifications {
		// Time filter
		if opts.Since > 0 {
			cutoff := now.Add(-opts.Since)
			if time.Unix(n.Timestamp, 0).Before(cutoff) {
				continue
			}
		}

		// App filter
		if opts.AppFilter != "" && n.AppName != opts.AppFilter {
			continue
		}

		// Urgency filter
		if opts.Urgency != nil && n.Urgency != *opts.Urgency {
			continue
		}

		result = append(result, n)
	}

	// Sort
	sortField := opts.SortField
	if sortField == "" {
		sortField = "timestamp"
	}
	sortOrder := opts.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	sortNotifications(result, sortField, sortOrder)

	// Limit
	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}

	return result
}

// Lookup finds a notification by ULID or content match.
func (s *Store) Lookup(input string) *model.Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// First, try exact ULID match
	if idx, exists := s.index[input]; exists {
		n := s.notifications[idx]
		return &n
	}

	// Try to extract ULID from input (first 26 chars if it looks like a ULID)
	if len(input) >= 26 {
		potentialULID := input[:26]
		if idx, exists := s.index[potentialULID]; exists {
			n := s.notifications[idx]
			return &n
		}
	}

	// Content-based match (fallback)
	// This is less reliable, so we return the most recent match
	var bestMatch *model.Notification
	for i := len(s.notifications) - 1; i >= 0; i-- {
		n := s.notifications[i]
		// Check if input contains app name and summary
		if containsNotification(input, &n) {
			if bestMatch == nil || n.Timestamp > bestMatch.Timestamp {
				nCopy := n
				bestMatch = &nCopy
			}
		}
	}

	return bestMatch
}

// GetByID returns a notification by its ULID.
func (s *Store) GetByID(id string) *model.Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if idx, exists := s.index[id]; exists {
		n := s.notifications[idx]
		return &n
	}
	return nil
}

// Delete removes a notification by its ULID.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	idx, exists := s.index[id]
	if !exists {
		return nil // Not found, nothing to do
	}

	// Remove from slice
	s.notifications = append(s.notifications[:idx], s.notifications[idx+1:]...)

	// Rebuild indices
	s.index = make(map[string]int, len(s.notifications))
	s.hashIndex = make(map[string]int, len(s.notifications))
	for i, n := range s.notifications {
		s.index[n.HistuiID] = i
		if n.ContentHash != "" {
			s.hashIndex[n.ContentHash] = i
		}
	}

	// Rewrite persistence file if enabled
	if s.persistence != nil {
		if err := s.persistence.Rewrite(s.notifications); err != nil {
			return err
		}
	}

	s.notifyChange(ChangeEvent{
		Type:  ChangeTypeDelete,
		Count: 1,
	})

	return nil
}

// Count returns the total number of notifications.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.notifications)
}

// Update modifies a notification in the store.
func (s *Store) Update(n model.Notification) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	idx, exists := s.index[n.HistuiID]
	if !exists {
		return nil // Not found
	}

	// Update in slice
	s.notifications[idx] = n

	// Persist by rewriting (could be optimized later)
	if s.persistence != nil {
		if err := s.persistence.Rewrite(s.notifications); err != nil {
			return err
		}
	}

	return nil
}

// Dismiss marks a notification as dismissed.
func (s *Store) Dismiss(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	idx, exists := s.index[id]
	if !exists {
		return nil // Not found
	}

	// Mark as dismissed
	s.notifications[idx].MarkDismissed()

	// Persist
	if s.persistence != nil {
		if err := s.persistence.Rewrite(s.notifications); err != nil {
			return err
		}
	}

	return nil
}

// DeleteWithTombstone removes a notification and remembers its hash to prevent reimport.
func (s *Store) DeleteWithTombstone(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	idx, exists := s.index[id]
	if !exists {
		return nil // Not found, nothing to do
	}

	// Get the content hash before deleting
	hash := s.notifications[idx].ContentHash
	if hash == "" {
		// Compute it if not set
		s.notifications[idx].EnsureContentHash()
		hash = s.notifications[idx].ContentHash
	}

	// Add to tombstones
	s.tombstones[hash] = true

	// Remove from slice
	s.notifications = append(s.notifications[:idx], s.notifications[idx+1:]...)

	// Rebuild indices
	s.index = make(map[string]int, len(s.notifications))
	s.hashIndex = make(map[string]int, len(s.notifications))
	for i, n := range s.notifications {
		s.index[n.HistuiID] = i
		if n.ContentHash != "" {
			s.hashIndex[n.ContentHash] = i
		}
	}

	// Rewrite persistence file if enabled
	if s.persistence != nil {
		if err := s.persistence.Rewrite(s.notifications); err != nil {
			return err
		}
	}

	s.notifyChange(ChangeEvent{
		Type:  ChangeTypeDelete,
		Count: 1,
	})

	return nil
}

// AddTombstone adds a content hash to the tombstone set.
func (s *Store) AddTombstone(hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tombstones[hash] = true
}

// GetTombstones returns all tombstone hashes.
func (s *Store) GetTombstones() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hashes := make([]string, 0, len(s.tombstones))
	for h := range s.tombstones {
		hashes = append(hashes, h)
	}
	return hashes
}

// LoadTombstones adds tombstones from a slice of hashes.
func (s *Store) LoadTombstones(hashes []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, h := range hashes {
		s.tombstones[h] = true
	}
}

// Subscribe returns a channel that receives change events.
func (s *Store) Subscribe() <-chan ChangeEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan ChangeEvent, 10)
	s.subscribers = append(s.subscribers, ch)
	return ch
}

// Unsubscribe removes a subscription.
func (s *Store) Unsubscribe(ch <-chan ChangeEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, sub := range s.subscribers {
		// Compare by checking if it's the same channel
		if sub == ch {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			close(sub)
			return
		}
	}
}

// Close releases resources and closes all subscriber channels.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	// Close all subscriber channels
	for _, ch := range s.subscribers {
		close(ch)
	}
	s.subscribers = nil

	// Close persistence
	if s.persistence != nil {
		return s.persistence.Close()
	}

	return nil
}

// Hydrate loads notifications from persistence into the store.
func (s *Store) Hydrate() error {
	if s.persistence == nil {
		return nil
	}

	notifications, err := s.persistence.Load()
	if err != nil {
		return err
	}

	s.mu.Lock()
	added := 0
	for i := range notifications {
		n := &notifications[i]

		// Ensure content hash exists (for older records without it)
		n.EnsureContentHash()

		// Skip duplicates by content hash
		if _, exists := s.hashIndex[n.ContentHash]; exists {
			continue
		}

		// Skip duplicates by ULID
		if _, exists := s.index[n.HistuiID]; exists {
			continue
		}

		idx := len(s.notifications)
		s.notifications = append(s.notifications, *n)
		s.index[n.HistuiID] = idx
		s.hashIndex[n.ContentHash] = idx
		added++
	}
	s.mu.Unlock()

	// Notify subscribers if any new notifications were added
	if added > 0 {
		s.notifyChange(ChangeEvent{
			Type:   ChangeTypeAdd,
			Count:  added,
			Source: "persistence",
		})
	}

	return nil
}

// Clear removes all notifications from the store.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	count := len(s.notifications)
	s.notifications = make([]model.Notification, 0)
	s.index = make(map[string]int)
	s.hashIndex = make(map[string]int)

	if s.persistence != nil {
		if err := s.persistence.Clear(); err != nil {
			return err
		}
	}

	s.notifyChange(ChangeEvent{
		Type:  ChangeTypeClear,
		Count: count,
	})

	return nil
}

// notifyChange sends a change event to all subscribers (non-blocking).
func (s *Store) notifyChange(event ChangeEvent) {
	for _, ch := range s.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
}

// sortNotifications sorts notifications in-place.
func sortNotifications(ns []model.Notification, field, order string) {
	sort.Slice(ns, func(i, j int) bool {
		var less bool
		switch field {
		case "app":
			less = ns[i].AppName < ns[j].AppName
		case "urgency":
			less = ns[i].Urgency < ns[j].Urgency
		default: // timestamp
			less = ns[i].Timestamp < ns[j].Timestamp
		}
		if order == "desc" {
			return !less
		}
		return less
	})
}

// containsNotification checks if input contains notification content.
func containsNotification(input string, n *model.Notification) bool {
	// Simple containment check - input should contain app name and summary
	return len(input) > 0 &&
		(contains(input, n.AppName) && contains(input, n.Summary))
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Errors
var (
	ErrStoreClosed = storeError("store is closed")
)

type storeError string

func (e storeError) Error() string {
	return string(e)
}
