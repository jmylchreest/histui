package output

import (
	"encoding/json"
	"io"

	"github.com/jmylchreest/histui/internal/model"
)

// JSONFormatter formats notifications as JSON.
type JSONFormatter struct {
	opts FormatterOptions
}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter(opts FormatterOptions) *JSONFormatter {
	return &JSONFormatter{opts: opts}
}

// Format writes notifications as a JSON array.
func (f *JSONFormatter) Format(w io.Writer, notifications []model.Notification) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(notifications)
}

// FormatSingle writes a single notification as JSON.
func (f *JSONFormatter) FormatSingle(w io.Writer, n *model.Notification) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(n)
}
