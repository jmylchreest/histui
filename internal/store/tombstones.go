package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
)

// TombstoneFile manages persistence of tombstone hashes.
type TombstoneFile struct {
	path string
}

// tombstoneData is the JSON structure for tombstones.
type tombstoneData struct {
	Hashes []string `json:"hashes"`
}

// NewTombstoneFile creates a new TombstoneFile.
func NewTombstoneFile(path string) *TombstoneFile {
	return &TombstoneFile{path: path}
}

// Load reads tombstone hashes from the file.
func (t *TombstoneFile) Load() ([]string, error) {
	data, err := os.ReadFile(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No file yet
		}
		return nil, err
	}

	var td tombstoneData
	if err := json.Unmarshal(data, &td); err != nil {
		return nil, err
	}

	return td.Hashes, nil
}

// Save writes tombstone hashes to the file.
func (t *TombstoneFile) Save(hashes []string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(t.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	td := tombstoneData{Hashes: hashes}
	data, err := json.MarshalIndent(td, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(t.path, data, 0600)
}

// Append adds a hash to the file.
func (t *TombstoneFile) Append(hash string) error {
	hashes, err := t.Load()
	if err != nil {
		return err
	}

	// Check if already exists
	if slices.Contains(hashes, hash) {
		return nil
	}

	hashes = append(hashes, hash)
	return t.Save(hashes)
}
