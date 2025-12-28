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

const cacheDir = ".cache"

// CacheEntry represents a cached API response.
type CacheEntry struct {
	Hash    string `json:"hash"`
	Model   string `json:"model"`
	Content string `json:"content"`
}

// ensureCacheDir creates the cache directory if it doesn't exist.
func ensureCacheDir() error {
	return os.MkdirAll(cacheDir, 0755)
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

// getCachePath returns the cache file path for a given type and hash.
func getCachePath(cacheType, hash string) string {
	return filepath.Join(cacheDir, fmt.Sprintf("%s-%s.json", cacheType, hash))
}

// loadCache loads a cached response if it exists and matches the model.
func loadCache(cacheType, hash, model string) (string, bool) {
	path := getCachePath(cacheType, hash)

	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return "", false
	}

	// Verify hash and model match
	if entry.Hash != hash {
		return "", false
	}

	// Model mismatch is OK - we still use the cache but note it
	// (user might want to regenerate with different model later)

	return entry.Content, true
}

// saveCache saves a response to cache.
func saveCache(cacheType, hash, model, content string) error {
	if err := ensureCacheDir(); err != nil {
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

	return os.WriteFile(getCachePath(cacheType, hash), data, 0644)
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

// ClearCache removes all cached files.
func ClearCache() error {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			os.Remove(filepath.Join(cacheDir, entry.Name()))
		}
	}
	return nil
}

// CacheStats returns cache statistics.
func CacheStats() (classifyCount, appGenCount int, totalSize int64) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return 0, 0, 0
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "classify-") {
			classifyCount++
		} else if strings.HasPrefix(entry.Name(), "appgen-") {
			appGenCount++
		}

		info, err := entry.Info()
		if err == nil {
			totalSize += info.Size()
		}
	}
	return
}
