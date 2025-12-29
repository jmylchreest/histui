package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jmylchreest/histui/internal/adapter/input"
	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/db"
	"github.com/jmylchreest/histui/internal/model"
)

// copyText copies text to the system clipboard.
func copyText(text string, cfg *config.Config) error {
	// Get clipboard command
	cmd := detectClipboardCommand(cfg)
	if cmd == "" {
		return fmt.Errorf("no clipboard command available")
	}

	// Parse command
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return fmt.Errorf("invalid clipboard command")
	}

	// Execute with text as stdin
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, parts[0], parts[1:]...)
	c.Stdin = strings.NewReader(text)

	return c.Run()
}

// copyImage copies image data to the system clipboard.
// mimeType should be "image/png", "image/jpeg", etc.
func copyImage(data []byte, mimeType string) error {
	if len(data) == 0 {
		return fmt.Errorf("no image data")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check for Wayland (wl-copy with -t for mime type)
	if _, err := exec.LookPath("wl-copy"); err == nil {
		c := exec.CommandContext(ctx, "wl-copy", "-t", mimeType)
		c.Stdin = bytes.NewReader(data)
		return c.Run()
	}

	// Check for X11 (xclip with -t for target/mime type)
	if _, err := exec.LookPath("xclip"); err == nil {
		c := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-t", mimeType)
		c.Stdin = bytes.NewReader(data)
		return c.Run()
	}

	return fmt.Errorf("no image clipboard command available (need wl-copy or xclip)")
}

// copyImageFromPath copies an image file to the clipboard.
func copyImageFromPath(path string) error {
	if path == "" {
		return fmt.Errorf("no image path")
	}

	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("image file not found: %s", path)
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read image: %w", err)
	}

	// Detect mime type from file content
	mimeType := "image/png"
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		mimeType = "image/jpeg"
	} else if len(data) >= 4 && string(data[:4]) == "GIF8" {
		mimeType = "image/gif"
	}

	return copyImage(data, mimeType)
}

// hasImage checks if a notification has an image available for copying.
// This only checks local sources (IconPath, Extensions.ImageData).
// Use hasImageWithDB for database-stored images.
func hasImage(n model.Notification) bool {
	// Check IconPath
	if n.IconPath != "" {
		if _, err := os.Stat(n.IconPath); err == nil {
			return true
		}
	}

	// Check embedded ImageData
	if n.Extensions != nil && len(n.Extensions.ImageData) > 0 {
		return true
	}

	return false
}

// hasImageWithDB checks if a notification has an image, including database-stored images.
func hasImageWithDB(n model.Notification, database *db.DB) bool {
	// Check local sources first
	if hasImage(n) {
		return true
	}

	// Check database for stored images
	if database != nil {
		if has, _ := database.HasImage(n.HistuiID, db.ImageRefImage); has {
			return true
		}
	}

	return false
}

// detectClipboardCommand returns the clipboard command to use.
func detectClipboardCommand(cfg *config.Config) string {
	// Use configured command if specified
	if cfg != nil && cfg.Clipboard.Command != "" {
		return cfg.Clipboard.Command
	}

	// Auto-detect based on environment
	// Check for Wayland
	if _, err := exec.LookPath("wl-copy"); err == nil {
		return "wl-copy"
	}

	// Check for X11
	if _, err := exec.LookPath("xclip"); err == nil {
		return "xclip -selection clipboard"
	}

	if _, err := exec.LookPath("xsel"); err == nil {
		return "xsel --clipboard --input"
	}

	return ""
}

// importFromAdapter imports notifications from an input adapter into the database.
func importFromAdapter(ctx context.Context, adapter input.InputAdapter, database *db.DB) error {
	if adapter == nil {
		return fmt.Errorf("no input adapter provided")
	}

	notifications, err := adapter.Import(ctx)
	if err != nil {
		return err
	}

	if len(notifications) > 0 {
		_ = database.AddBatch(notifications)
	}

	return nil
}
