package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	// Clear removes all stored notifications.
	Clear() error

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
	mu     sync.RWMutex
	path   string
	file   *os.File
	closed bool
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
		file.Close()
		return nil, err
	}

	if info.Size() == 0 {
		if err := p.writeHeader(); err != nil {
			file.Close()
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
			// Log and skip malformed lines
			continue
		}

		if n.HistuiID != "" {
			notifications = append(notifications, n)
		}
	}

	if err := scanner.Err(); err != nil {
		return notifications, fmt.Errorf("error reading file: %w", err)
	}

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
		os.Rename(backupPath, p.path)
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
	os.Remove(backupPath)

	return nil
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
		os.Rename(backupPath, p.path)
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

// RecoverFromCorruption attempts to recover from a corrupted file.
// It creates a backup and rewrites only valid notifications.
func RecoverFromCorruption(path string) error {
	// Read file and collect valid notifications
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	var valid []model.Notification
	scanner := bufio.NewScanner(file)
	const maxLineSize = 1024 * 1024
	scanner.Buffer(make([]byte, 64*1024), maxLineSize)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Skip header lines
		var header schemaHeader
		if json.Unmarshal(line, &header) == nil && header.HistuiSchemaVersion > 0 {
			continue
		}

		var n model.Notification
		if err := json.Unmarshal(line, &n); err == nil && n.HistuiID != "" {
			valid = append(valid, n)
		}
	}
	file.Close()

	if scanner.Err() != nil && !errors.Is(scanner.Err(), io.EOF) {
		// Continue with what we have
	}

	// Create backup
	backupPath := path + ".corrupted." + time.Now().Format("20060102-150405")
	if err := os.Rename(path, backupPath); err != nil {
		return fmt.Errorf("failed to backup corrupted file: %w", err)
	}

	// Create new persistence and write valid notifications
	p, err := NewJSONLPersistence(path)
	if err != nil {
		return err
	}
	defer p.Close()

	return p.AppendBatch(valid)
}
