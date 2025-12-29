package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const cacheBaseDir = ".cache"

// CacheEntry represents a cached API response.
type CacheEntry struct {
	Hash    string `json:"hash"`
	Model   string `json:"model"`
	Content string `json:"content"`
}

// sanitizeModelForPath converts a model string to a safe directory name.
// e.g., "anthropic/claude-sonnet-4:online" -> "anthropic_claude-sonnet-4_online"
func sanitizeModelForPath(model string) string {
	s := strings.ReplaceAll(model, "/", "_")
	s = strings.ReplaceAll(s, ":", "_")
	return s
}

// getCacheDir returns the cache directory for a specific model.
func getCacheDir(model string) string {
	return filepath.Join(cacheBaseDir, sanitizeModelForPath(model))
}

// ensureCacheDir creates the cache directory for a model if it doesn't exist.
func ensureCacheDir(model string) error {
	return os.MkdirAll(getCacheDir(model), 0755)
}

// hashStrings creates a deterministic hash from a list of strings.
func hashStrings(items []string) string {
	// Sort for deterministic hash
	sorted := make([]string, len(items))
	copy(sorted, items)
	sort.Strings(sorted)

	h := sha256.New()
	for _, item := range sorted {
		h.Write([]byte(item))
		h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil))[:16] // First 16 chars is enough
}

// getCachePath returns the cache file path for a given type, hash, and model.
func getCachePath(cacheType, hash, model string) string {
	return filepath.Join(getCacheDir(model), fmt.Sprintf("%s-%s.json", cacheType, hash))
}

// loadCache loads a cached response if it exists for the given model.
func loadCache(cacheType, hash, model string) (string, bool) {
	path := getCachePath(cacheType, hash, model)

	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return "", false
	}

	// Verify hash matches
	if entry.Hash != hash {
		return "", false
	}

	return entry.Content, true
}

// saveCache saves a response to cache for the given model.
func saveCache(cacheType, hash, model, content string) error {
	if err := ensureCacheDir(model); err != nil {
		return err
	}

	entry := CacheEntry{
		Hash:    hash,
		Model:   model,
		Content: content,
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(getCachePath(cacheType, hash, model), data, 0644)
}

// ClassifyCacheKey generates a cache key for classification requests.
func ClassifyCacheKey(glyphNames []string) string {
	return hashStrings(glyphNames)
}

// AppGenCacheKey generates a cache key for app generation requests.
func AppGenCacheKey(icons []struct{ Name, Type string }) string {
	var items []string
	for _, icon := range icons {
		items = append(items, fmt.Sprintf("%s:%s", icon.Name, icon.Type))
	}
	return hashStrings(items)
}

// ClearCache removes all cached files for all models.
func ClearCache() error {
	return os.RemoveAll(cacheBaseDir)
}

// ClearCacheForModel removes all cached files for a specific model.
func ClearCacheForModel(model string) error {
	return os.RemoveAll(getCacheDir(model))
}

// CacheStats returns cache statistics across all models.
func CacheStats() (classifyCount, appGenCount int, totalSize int64) {
	// Walk all model subdirectories
	_ = filepath.Walk(cacheBaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // ignore errors
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		if strings.HasPrefix(info.Name(), "classify-") {
			classifyCount++
		} else if strings.HasPrefix(info.Name(), "appgen-") {
			appGenCount++
		}
		totalSize += info.Size()
		return nil
	})
	return
}

// ListCachedModels returns a list of models that have cached data.
func ListCachedModels() []string {
	var models []string
	entries, err := os.ReadDir(cacheBaseDir)
	if err != nil {
		return models
	}
	for _, entry := range entries {
		if entry.IsDir() {
			models = append(models, entry.Name())
		}
	}
	return models
}
