// Package icon provides icon name resolution and font loading.
package icon

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
)

var (
	fontInitOnce sync.Once
	fontDir      string
	fontErr      error
)

// InitFonts initializes all embedded fonts by writing them to a temp directory
// and registering them with fontconfig. This should be called once at startup
// before any GTK operations.
//
// Fonts are written to a private temporary directory in $XDG_RUNTIME_DIR or /tmp.
// Returns the fonts directory path and any error encountered.
func InitFonts() (string, error) {
	fontInitOnce.Do(func() {
		fontDir, fontErr = initFontsOnce()
	})
	return fontDir, fontErr
}

func initFontsOnce() (string, error) {
	// Get runtime directory
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = "/tmp"
	}

	// Create histui fonts directory
	fontsDir, err := os.MkdirTemp(runtimeDir, "histui-fonts-")
	if err != nil {
		return "", err
	}

	// Walk embedded fonts and write each to the temp directory
	fontCount := 0
	err = fs.WalkDir(EmbeddedFonts, "fonts", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Only process font files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".ttf" && ext != ".otf" {
			return nil
		}

		// Read embedded font
		data, err := EmbeddedFonts.ReadFile(path)
		if err != nil {
			return err
		}

		// Write to temp directory
		fontFile := filepath.Join(fontsDir, d.Name())
		if err := os.WriteFile(fontFile, data, 0600); err != nil {
			return err
		}

		fontCount++
		return nil
	})
	if err != nil {
		return "", err
	}

	if fontCount == 0 {
		return fontsDir, nil // No fonts to register
	}

	// Load lazily so CLI/TUI operations do not require fontconfig. GTK shares
	// this process-owned library handle; keep it loaded for the process lifetime.
	handle, err := purego.Dlopen("libfontconfig.so.1", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		_ = os.RemoveAll(fontsDir)
		return "", fmt.Errorf("load fontconfig: %w", err)
	}
	var addDir func(uintptr, string) int32
	purego.RegisterLibFunc(&addDir, handle, "FcConfigAppFontAddDir")
	if addDir(0, fontsDir) == 0 {
		_ = os.RemoveAll(fontsDir)
		return "", fmt.Errorf("register embedded fonts")
	}

	return fontsDir, nil
}

// CleanupFonts removes the temporary fonts directory.
// This is optional - the OS will clean up $XDG_RUNTIME_DIR on logout.
func CleanupFonts() {
	if fontDir != "" {
		_ = os.RemoveAll(fontDir)
	}
}

// ListEmbeddedFonts returns the names of all embedded font files.
func ListEmbeddedFonts() []string {
	var fonts []string
	_ = fs.WalkDir(EmbeddedFonts, "fonts", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".ttf" || ext == ".otf" {
			fonts = append(fonts, d.Name())
		}
		return nil
	})
	return fonts
}
