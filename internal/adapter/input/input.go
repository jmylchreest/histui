// Package input provides input adapters for notification sources.
package input

import (
	"context"
	"os/exec"

	"github.com/jmylchreest/histui/internal/model"
)

// InputAdapter fetches notifications from a source.
type InputAdapter interface {
	// Name returns the adapter identifier (e.g., "dunst", "stdin").
	Name() string

	// Import fetches notifications from the source.
	// Returns the notifications and any error encountered.
	Import(ctx context.Context) ([]model.Notification, error)
}

// DetectDaemon returns the name of the first available notification daemon.
// Returns empty string if none found.
func DetectDaemon() string {
	// Check for dunst
	if _, err := exec.LookPath("dunstctl"); err == nil {
		return "dunst"
	}

	// Future: Add detection for other daemons
	// - mako (makoctl)
	// - swaync (swaync-client)

	return ""
}

// NewAdapter creates an InputAdapter for the specified source.
// If source is empty, attempts to auto-detect.
func NewAdapter(source string) (InputAdapter, error) {
	if source == "" {
		source = DetectDaemon()
	}

	switch source {
	case "dunst":
		return NewDunstAdapter(), nil
	case "stdin":
		return NewStdinAdapter(), nil
	default:
		return nil, &AdapterError{
			Source:  source,
			Message: "unknown or unavailable adapter",
		}
	}
}

// AdapterError represents an adapter-related error.
type AdapterError struct {
	Source  string
	Message string
	Err     error
}

func (e *AdapterError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AdapterError) Unwrap() error {
	return e.Err
}
