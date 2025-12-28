package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/jmylchreest/histui/internal/model"
)

// SchemaVersion is the current persistence schema version.
const SchemaVersion = 1

// Persistence defines the interface for history storage.
type Persistence interface {
	// Load reads all notifications from storage.
	Load() ([]model.Notification, error)

	// Append adds a notification to storage.
	Append(n model.Notification) error

	// AppendBatch adds multiple notifications efficiently.
	AppendBatch(ns []model.Notification) error

	// Rewrite replaces the entire storage file (used after prune).
	Rewrite(ns []model.Notification) error

	// Prune removes the oldest notifications, keeping at most maxKeep.
	// Returns the IDs of pruned notifications for cascade cleanup (e.g., images).
	Prune(maxKeep int) (prunedIDs []string, err error)

	// Clear removes all stored notifications.
	Clear() error

	// ReopenFile closes and reopens the file handle.
	// Use this after detecting external file replacement (e.g., after prune by another process).
	ReopenFile() error

	// Close releases file handles and resources.
	Close() error
}

// schemaHeader is the first line of the JSONL file.
type schemaHeader struct {
	HistuiSchemaVersion int   `json:"histui_schema_version"`
	CreatedAt           int64 `json:"created_at"`
}

// JSONLPersistence implements Persistence using JSONL files.
type JSONLPersistence struct {
	mu             sync.RWMutex
	path           string
	file           *os.File
	closed         bool
	needsRepair    bool                 // true if corruption was detected during Load
	malformedCount int                  // number of malformed lines found
	lastValid      []model.Notification // cached valid notifications from last Load
}

// NewJSONLPersistence creates a new JSONLPersistence.
// Creates the file if it doesn't exist.
func NewJSONLPersistence(path string) (*JSONLPersistence, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Open file for appending (create if needed)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}

	p := &JSONLPersistence{
		path: path,
		file: file,
	}

	// Check if file is empty and write header
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}

	if info.Size() == 0 {
		if err := p.writeHeader(); err != nil {
			_ = file.Close()
			return nil, err
		}
	}

	return p, nil
}

// writeHeader writes the schema version header to the file.
func (p *JSONLPersistence) writeHeader() error {
	header := schemaHeader{
		HistuiSchemaVersion: SchemaVersion,
		CreatedAt:           time.Now().Unix(),
	}

	data, err := json.Marshal(header)
	if err != nil {
		return err
	}

	_, err = p.file.Write(append(data, '\n'))
	return err
}

// ErrPersistenceClosed is returned when operations are attempted on a closed persistence.
var ErrPersistenceClosed = errors.New("persistence is closed")

// Load reads all notifications from storage.
func (p *JSONLPersistence) Load() ([]model.Notification, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed || p.file == nil {
		return nil, ErrPersistenceClosed
	}

	// Seek to beginning
	if _, err := p.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek %s: %w", p.path, err)
	}

	var notifications []model.Notification
	scanner := bufio.NewScanner(p.file)

	// Increase buffer size for potentially long lines
	const maxLineSize = 1024 * 1024 // 1MB
	scanner.Buffer(make([]byte, 64*1024), maxLineSize)

	// Reset corruption tracking
	malformedCount := 0

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		if len(line) == 0 {
			continue
		}

		// First line is the header
		if lineNum == 1 {
			var header schemaHeader
			if err := json.Unmarshal(line, &header); err != nil {
				// Not a valid header, might be legacy format
				// Try parsing as notification
				var n model.Notification
				if err := json.Unmarshal(line, &n); err == nil && n.HistuiID != "" {
					notifications = append(notifications, n)
				} else {
					malformedCount++
				}
				continue
			}

			if header.HistuiSchemaVersion > SchemaVersion {
				return nil, fmt.Errorf("unsupported schema version %d (max: %d)",
					header.HistuiSchemaVersion, SchemaVersion)
			}
			continue
		}

		// Parse notification
		var n model.Notification
		if err := json.Unmarshal(line, &n); err != nil {
			// Track malformed lines for lazy repair
			malformedCount++
			continue
		}

		if n.HistuiID != "" {
			notifications = append(notifications, n)
		}
	}

	if err := scanner.Err(); err != nil {
		return notifications, fmt.Errorf("error reading file: %w", err)
	}

	// Track corruption state for lazy repair
	p.malformedCount = malformedCount
	p.needsRepair = malformedCount > 0
	p.lastValid = notifications

	// Seek back to end for appending
	if _, err := p.file.Seek(0, io.SeekEnd); err != nil {
		return notifications, err
	}

	return notifications, nil
}

// Append adds a notification to storage.
func (p *JSONLPersistence) Append(n model.Notification) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed || p.file == nil {
		return ErrPersistenceClosed
	}

	// Lazy repair: if corruption was detected, rewrite the entire file
	if p.needsRepair {
		slog.Warn("repairing corrupted history file",
			"path", p.path,
			"malformed_lines", p.malformedCount,
			"valid_notifications", len(p.lastValid),
		)

		// Combine existing valid notifications with the new one
		allNotifications := append(p.lastValid, n)

		if err := p.rewriteUnlocked(allNotifications); err != nil {
			return fmt.Errorf("lazy repair failed: %w", err)
		}

		// Clear repair state
		p.needsRepair = false
		p.malformedCount = 0
		p.lastValid = allNotifications

		return nil
	}

	data, err := json.Marshal(n)
	if err != nil {
		return err
	}

	_, err = p.file.Write(append(data, '\n'))
	if err != nil {
		return err
	}

	return p.file.Sync()
}

// AppendBatch adds multiple notifications efficiently.
func (p *JSONLPersistence) AppendBatch(ns []model.Notification) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed || p.file == nil {
		return ErrPersistenceClosed
	}

	for _, n := range ns {
		data, err := json.Marshal(n)
		if err != nil {
			return err
		}
		if _, err := p.file.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return p.file.Sync()
}

// Rewrite replaces the entire storage file (used after prune/delete).
func (p *JSONLPersistence) Rewrite(ns []model.Notification) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrPersistenceClosed
	}

	return p.rewriteUnlocked(ns)
}

// rewriteUnlocked performs the actual rewrite without acquiring the lock.
// Caller must hold p.mu.
func (p *JSONLPersistence) rewriteUnlocked(ns []model.Notification) error {
	// Close current file
	if p.file != nil {
		if err := p.file.Close(); err != nil {
			return err
		}
		p.file = nil
	}

	// Create backup
	backupPath := p.path + ".bak"
	if err := os.Rename(p.path, backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Create new file
	file, err := os.OpenFile(p.path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		// Try to restore backup
		_ = os.Rename(backupPath, p.path)
		return fmt.Errorf("failed to create new file: %w", err)
	}
	p.file = file

	// Write header
	if err := p.writeHeader(); err != nil {
		return err
	}

	// Write all notifications
	for _, n := range ns {
		data, err := json.Marshal(n)
		if err != nil {
			return err
		}
		if _, err := p.file.Write(append(data, '\n')); err != nil {
			return err
		}
	}

	if err := p.file.Sync(); err != nil {
		return err
	}

	// Remove backup on success
	_ = os.Remove(backupPath)

	return nil
}

// Prune removes the oldest notifications, keeping at most maxKeep.
// Returns the IDs of pruned notifications for cascade cleanup (e.g., images).
// If maxKeep <= 0, no pruning is performed.
func (p *JSONLPersistence) Prune(maxKeep int) ([]string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, ErrPersistenceClosed
	}

	if maxKeep <= 0 {
		return nil, nil // Unlimited, no pruning
	}

	// Load all notifications
	notifications, err := p.loadAllLocked()
	if err != nil {
		return nil, fmt.Errorf("load for prune: %w", err)
	}

	if len(notifications) <= maxKeep {
		return nil, nil // Nothing to prune
	}

	// Sort by timestamp ascending (oldest first)
	sort.Slice(notifications, func(i, j int) bool {
		return notifications[i].Timestamp < notifications[j].Timestamp
	})

	// Calculate how many to prune
	pruneCount := len(notifications) - maxKeep
	toPrune := notifications[:pruneCount]
	toKeep := notifications[pruneCount:]

	// Collect IDs for cascade cleanup
	prunedIDs := make([]string, len(toPrune))
	for i, n := range toPrune {
		prunedIDs[i] = n.HistuiID
	}

	// Use atomic rewrite
	if err := p.rewriteUnlocked(toKeep); err != nil {
		return nil, fmt.Errorf("prune rewrite: %w", err)
	}

	return prunedIDs, nil
}

// loadAllLocked reads all notifications without taking the lock.
// Caller must hold p.mu.
func (p *JSONLPersistence) loadAllLocked() ([]model.Notification, error) {
	if p.file == nil {
		return nil, ErrPersistenceClosed
	}

	// Seek to beginning
	if _, err := p.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	var notifications []model.Notification
	scanner := bufio.NewScanner(p.file)
	const maxLineSize = 1024 * 1024
	scanner.Buffer(make([]byte, 64*1024), maxLineSize)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		if len(line) == 0 {
			continue
		}

		// Skip header line
		if lineNum == 1 {
			var header schemaHeader
			if json.Unmarshal(line, &header) == nil && header.HistuiSchemaVersion > 0 {
				continue
			}
			// Not a header, try as notification
		}

		var n model.Notification
		if err := json.Unmarshal(line, &n); err != nil {
			continue // Skip malformed
		}

		if n.HistuiID != "" {
			notifications = append(notifications, n)
		}
	}

	if err := scanner.Err(); err != nil {
		return notifications, fmt.Errorf("scan: %w", err)
	}

	// Seek back to end for appending
	if _, err := p.file.Seek(0, io.SeekEnd); err != nil {
		return notifications, err
	}

	return notifications, nil
}

// Clear removes all stored notifications.
func (p *JSONLPersistence) Clear() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrPersistenceClosed
	}

	// Create backup
	backupPath := p.path + ".bak"
	if p.file != nil {
		if err := p.file.Close(); err != nil {
			return err
		}
		p.file = nil
	}

	if err := os.Rename(p.path, backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Create new empty file with header
	file, err := os.OpenFile(p.path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		_ = os.Rename(backupPath, p.path)
		return err
	}
	p.file = file

	if err := p.writeHeader(); err != nil {
		return err
	}

	return p.file.Sync()
}

// Close releases file handles and resources.
func (p *JSONLPersistence) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	if p.file != nil {
		err := p.file.Close()
		p.file = nil
		return err
	}
	return nil
}

// ReopenFile closes the current file handle and opens a fresh one.
// Use this when the file has been replaced by an external process (e.g., prune by CLI).
// This ensures we're reading/writing to the current file, not a stale inode.
func (p *JSONLPersistence) ReopenFile() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrPersistenceClosed
	}

	// Close current handle
	if p.file != nil {
		if err := p.file.Close(); err != nil {
			return fmt.Errorf("close old handle: %w", err)
		}
		p.file = nil
	}

	// Open fresh handle to current path
	file, err := os.OpenFile(p.path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("reopen file: %w", err)
	}
	p.file = file

	// Seek to end for appending
	if _, err := p.file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek to end: %w", err)
	}

	return nil
}
