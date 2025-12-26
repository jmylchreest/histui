// Package output provides output formatters for notifications.
package output

import (
	"io"

	"github.com/jmylchreest/histui/internal/model"
)

// Formatter formats notifications for output.
type Formatter interface {
	// Format writes formatted notifications to the writer.
	Format(w io.Writer, notifications []model.Notification) error
}

// FormatType represents an output format type.
type FormatType string

const (
	FormatDmenu FormatType = "dmenu"
	FormatJSON  FormatType = "json"
	FormatPlain FormatType = "plain"
)

// NewFormatter creates a formatter for the specified format type.
func NewFormatter(format FormatType, opts FormatterOptions) Formatter {
	switch format {
	case FormatJSON:
		return NewJSONFormatter(opts)
	case FormatPlain:
		return NewPlainFormatter(opts)
	case FormatDmenu:
		fallthrough
	default:
		return NewDmenuFormatter(opts)
	}
}

// FormatterOptions configures formatter behavior.
type FormatterOptions struct {
	Template       string // Custom template for dmenu/plain format
	ShowIndex      bool   // Show 1-based index prefix
	ShowTime       bool   // Show relative time
	ShowApp        bool   // Show app name
	BodyMaxLen     int    // Maximum body length (0 = unlimited)
	Separator      string // Field separator for dmenu format
	OutputField    string // Field to output (for single-notification mode)
	IncludeNewline bool   // Include newlines in body (default: replace with space)
}

// DefaultFormatterOptions returns sensible defaults for dmenu output.
func DefaultFormatterOptions() FormatterOptions {
	return FormatterOptions{
		ShowIndex:      true,
		ShowTime:       true,
		ShowApp:        true,
		BodyMaxLen:     80,
		Separator:      " | ",
		IncludeNewline: false,
	}
}
