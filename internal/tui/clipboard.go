package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jmylchreest/histui/internal/adapter/input"
	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/store"
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

// importFromAdapter imports notifications from an input adapter into the store.
func importFromAdapter(ctx context.Context, adapter input.InputAdapter, s *store.Store) error {
	if adapter == nil {
		return fmt.Errorf("no input adapter provided")
	}

	notifications, err := adapter.Import(ctx)
	if err != nil {
		return err
	}

	if len(notifications) > 0 {
		_ = s.AddBatch(notifications)
	}

	return nil
}
