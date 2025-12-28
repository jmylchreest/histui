package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ImageRefType identifies the type of image stored.
type ImageRefType string

const (
	// ImageRefIcon is the notification icon (typically small).
	ImageRefIcon ImageRefType = "icon"
	// ImageRefImage is the notification image (typically larger, inline).
	ImageRefImage ImageRefType = "image"
)

// ImageStore manages notification images on disk.
// Images are stored at ~/.local/share/histui/images/[histui_id]_[ref].png
type ImageStore struct {
	mu   sync.RWMutex
	dir  string
	open bool
}

// NewImageStore creates a new ImageStore at the specified directory.
// Creates the directory if it doesn't exist.
func NewImageStore(dir string) (*ImageStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create images directory %s: %w", dir, err)
	}

	return &ImageStore{
		dir:  dir,
		open: true,
	}, nil
}

// DefaultImageStorePath returns the default path for the image store.
func DefaultImageStorePath() (string, error) {
	dataDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, ".local", "share", "histui", "images"), nil
}

// Save stores image data for a notification.
// Returns the ref string to be stored in the notification's HistuiImageRefs.
func (s *ImageStore) Save(histuiID string, refType ImageRefType, data []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.open {
		return "", fmt.Errorf("image store is closed")
	}

	if len(data) == 0 {
		return "", nil
	}

	ref := string(refType)
	filename := fmt.Sprintf("%s_%s.png", histuiID, ref)
	path := filepath.Join(s.dir, filename)

	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write image %s: %w", path, err)
	}

	return ref, nil
}

// Load retrieves image data for a notification by ref.
func (s *ImageStore) Load(histuiID string, ref string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.open {
		return nil, fmt.Errorf("image store is closed")
	}

	filename := fmt.Sprintf("%s_%s.png", histuiID, ref)
	path := filepath.Join(s.dir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not found is not an error
		}
		return nil, fmt.Errorf("failed to read image %s: %w", path, err)
	}

	return data, nil
}

// LoadAll retrieves all images for a notification.
// Returns a map of ref -> data.
func (s *ImageStore) LoadAll(histuiID string, refs []string) (map[string][]byte, error) {
	result := make(map[string][]byte, len(refs))

	for _, ref := range refs {
		data, err := s.Load(histuiID, ref)
		if err != nil {
			return nil, err
		}
		if data != nil {
			result[ref] = data
		}
	}

	return result, nil
}

// Delete removes all images for a notification.
func (s *ImageStore) Delete(histuiID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.open {
		return fmt.Errorf("image store is closed")
	}

	// Delete all possible image refs
	for _, refType := range []ImageRefType{ImageRefIcon, ImageRefImage} {
		filename := fmt.Sprintf("%s_%s.png", histuiID, refType)
		path := filepath.Join(s.dir, filename)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete image %s: %w", path, err)
		}
	}

	return nil
}

// DeleteByIDs removes images for multiple notifications.
// Returns the count of notifications cleaned up.
func (s *ImageStore) DeleteByIDs(histuiIDs []string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.open {
		return 0, fmt.Errorf("image store is closed")
	}

	deleted := 0
	for _, id := range histuiIDs {
		for _, refType := range []ImageRefType{ImageRefIcon, ImageRefImage} {
			filename := fmt.Sprintf("%s_%s.png", id, refType)
			path := filepath.Join(s.dir, filename)
			if err := os.Remove(path); err != nil {
				if !os.IsNotExist(err) {
					return deleted, fmt.Errorf("failed to delete image %s: %w", path, err)
				}
			}
		}
		deleted++
	}

	return deleted, nil
}

// Path returns the full path to an image file.
func (s *ImageStore) Path(histuiID string, ref string) string {
	filename := fmt.Sprintf("%s_%s.png", histuiID, ref)
	return filepath.Join(s.dir, filename)
}

// Exists checks if an image exists for the given notification and ref.
func (s *ImageStore) Exists(histuiID string, ref string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filename := fmt.Sprintf("%s_%s.png", histuiID, ref)
	path := filepath.Join(s.dir, filename)
	_, err := os.Stat(path)
	return err == nil
}

// Dir returns the image store directory.
func (s *ImageStore) Dir() string {
	return s.dir
}

// Close marks the store as closed.
func (s *ImageStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.open = false
	return nil
}

// CleanOrphaned removes images that don't have corresponding notification IDs.
// The validIDs parameter is a set of all valid notification histui_ids.
func (s *ImageStore) CleanOrphaned(validIDs map[string]bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.open {
		return 0, fmt.Errorf("image store is closed")
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read images directory: %w", err)
	}

	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Parse filename: [histui_id]_[ref].png
		// ULID is 26 chars, so id ends at index 26
		if len(name) < 28 { // 26 (ULID) + 1 (_) + 1 (min ref) = 28 min
			continue
		}

		histuiID := name[:26]
		if !validIDs[histuiID] {
			path := filepath.Join(s.dir, name)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return removed, fmt.Errorf("failed to remove orphaned image %s: %w", path, err)
			}
			removed++
		}
	}

	return removed, nil
}
