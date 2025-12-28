// Package daemon provides the main orchestration for histuid.
package daemon

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jmylchreest/histui/internal/icon"
)

// CleanupTemp removes temporary files created by histuid.
// This should be called on both startup (in case of unclean shutdown) and shutdown.
// It only removes regular files and directories that histuid created, never symlinks or sockets.
func CleanupTemp(logger *slog.Logger) {
	cleanupFonts(logger)
	cleanupSoundsCache(logger)
}

// cleanupFonts removes the temporary fonts directory.
// Fonts are extracted to $XDG_RUNTIME_DIR/histui/fonts/ or /tmp/histui/fonts/.
func cleanupFonts(logger *slog.Logger) {
	icon.CleanupFonts()
	if logger != nil {
		logger.Debug("cleaned up temporary fonts")
	}
}

// cleanupSoundsCache removes extracted theme sounds from the cache directory.
// Sounds are cached at ~/.cache/histui/themes/*/sounds/.
func cleanupSoundsCache(logger *slog.Logger) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		if logger != nil {
			logger.Debug("failed to get cache dir for cleanup", "error", err)
		}
		return
	}

	themesDir := filepath.Join(cacheDir, "histui", "themes")
	if _, err := os.Stat(themesDir); os.IsNotExist(err) {
		return // Nothing to clean
	}

	// Walk the themes directory and clean up sounds subdirectories
	entries, err := os.ReadDir(themesDir)
	if err != nil {
		if logger != nil {
			logger.Debug("failed to read themes cache dir", "error", err)
		}
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		soundsDir := filepath.Join(themesDir, entry.Name(), "sounds")
		if _, err := os.Stat(soundsDir); os.IsNotExist(err) {
			continue
		}

		// Remove only regular files from the sounds directory
		cleanupDirectory(soundsDir, logger)

		// Try to remove the empty sounds directory
		_ = os.Remove(soundsDir)

		// Try to remove the empty theme directory
		themeDir := filepath.Join(themesDir, entry.Name())
		_ = os.Remove(themeDir)
	}

	// Try to remove the empty themes directory
	_ = os.Remove(themesDir)

	// Try to remove the empty histui cache directory
	histuiCacheDir := filepath.Join(cacheDir, "histui")
	_ = os.Remove(histuiCacheDir)

	if logger != nil {
		logger.Debug("cleaned up sounds cache")
	}
}

// cleanupDirectory removes only regular files from a directory.
// Symlinks, sockets, and other special files are never removed.
func cleanupDirectory(dir string, logger *slog.Logger) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		// Get file info to check the actual file type (not following symlinks)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Only remove regular files
		if !info.Mode().IsRegular() {
			if logger != nil {
				logger.Debug("skipping non-regular file during cleanup",
					"path", filepath.Join(dir, entry.Name()),
					"mode", info.Mode().String())
			}
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := os.Remove(path); err != nil {
			if logger != nil {
				logger.Debug("failed to remove file during cleanup", "path", path, "error", err)
			}
		}
	}
}
