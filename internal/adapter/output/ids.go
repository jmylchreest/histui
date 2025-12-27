package output

import (
	"fmt"
	"io"

	"github.com/jmylchreest/histui/internal/model"
)

// IDsFormatter outputs just the histui IDs, one per line.
// Useful for piping to other commands (e.g., histui set --stdin).
type IDsFormatter struct{}

// NewIDsFormatter creates a new IDs formatter.
func NewIDsFormatter() *IDsFormatter {
	return &IDsFormatter{}
}

// Format writes histui IDs to the writer, one per line.
func (f *IDsFormatter) Format(w io.Writer, notifications []model.Notification) error {
	for _, n := range notifications {
		if _, err := fmt.Fprintln(w, n.HistuiID); err != nil {
			return err
		}
	}
	return nil
}
