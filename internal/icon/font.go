// Package icon provides icon name resolution and font loading.
package icon

/*
#cgo pkg-config: fontconfig
#include <fontconfig/fontconfig.h>
#include <stdlib.h>
*/
import "C"

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"
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
// Fonts are written to $XDG_RUNTIME_DIR/histui/fonts/ or /tmp/histui/fonts/.
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
	fontsDir := filepath.Join(runtimeDir, "histui", "fonts")
	if err := os.MkdirAll(fontsDir, 0700); err != nil {
		return "", err
	}

	// Walk embedded fonts and write each to the temp directory
	fontCount := 0
	err := fs.WalkDir(EmbeddedFonts, "fonts", func(path string, d fs.DirEntry, err error) error {
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

	// Register the fonts directory with fontconfig
	cDir := C.CString(fontsDir)
	defer C.free(unsafe.Pointer(cDir))
	C.FcConfigAppFontAddDir(nil, (*C.FcChar8)(unsafe.Pointer(cDir)))

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
