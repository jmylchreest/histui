package output

import (
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/jmylchreest/histui/internal/model"
)

// DmenuFormatter formats notifications for dmenu/rofi/fuzzel.
type DmenuFormatter struct {
	opts     FormatterOptions
	template *template.Template
}

// NewDmenuFormatter creates a new dmenu formatter.
func NewDmenuFormatter(opts FormatterOptions) *DmenuFormatter {
	f := &DmenuFormatter{opts: opts}

	// Parse custom template if provided
	if opts.Template != "" {
		tmpl, err := template.New("dmenu").Funcs(templateFuncs()).Parse(opts.Template)
		if err == nil {
			f.template = tmpl
		}
	}

	return f
}

// Format writes notifications in dmenu format (one per line).
func (f *DmenuFormatter) Format(w io.Writer, notifications []model.Notification) error {
	for i, n := range notifications {
		line := f.formatLine(i+1, &n)
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

// formatLine formats a single notification line.
func (f *DmenuFormatter) formatLine(index int, n *model.Notification) string {
	// Use custom template if available
	if f.template != nil {
		var buf strings.Builder
		data := templateData{
			Index:        index,
			Notification: n,
			RelativeTime: relativeTime(n.Timestamp),
		}
		if err := f.template.Execute(&buf, data); err == nil {
			return buf.String()
		}
	}

	// Default format: [index] [time] [app] summary: body
	var parts []string
	sep := f.opts.Separator
	if sep == "" {
		sep = " | "
	}

	if f.opts.ShowIndex {
		parts = append(parts, fmt.Sprintf("%d", index))
	}

	if f.opts.ShowTime {
		parts = append(parts, relativeTime(n.Timestamp))
	}

	if f.opts.ShowApp && n.AppName != "" {
		parts = append(parts, n.AppName)
	}

	// Summary and body
	content := n.Summary
	if n.Body != "" {
		body := sanitizeBody(n.Body, f.opts.BodyMaxLen, f.opts.IncludeNewline)
		if body != "" {
			content += ": " + body
		}
	}
	parts = append(parts, content)

	return strings.Join(parts, sep)
}

// templateData provides data for custom templates.
type templateData struct {
	Index        int
	Notification *model.Notification
	RelativeTime string
}

// templateFuncs returns template helper functions.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"truncate": func(s string, maxLen int) string {
			if maxLen <= 0 || len(s) <= maxLen {
				return s
			}
			if maxLen <= 3 {
				return s[:maxLen]
			}
			return s[:maxLen-3] + "..."
		},
		"reltime": func(ts int64) string {
			return relativeTime(ts)
		},
		"urgencyIcon": func(urgency int) string {
			switch urgency {
			case model.UrgencyLow:
				return "L"
			case model.UrgencyCritical:
				return "!"
			default:
				return "-"
			}
		},
	}
}

// relativeTime returns a human-readable relative time string.
func relativeTime(timestamp int64) string {
	if timestamp == 0 {
		return "unknown"
	}

	t := time.Unix(timestamp, 0)
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		mins := int(d.Minutes())
		return fmt.Sprintf("%dm", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		return fmt.Sprintf("%dh", hours)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	default:
		weeks := int(d.Hours() / 24 / 7)
		return fmt.Sprintf("%dw", weeks)
	}
}

// sanitizeBody cleans up body text for single-line display.
func sanitizeBody(body string, maxLen int, includeNewline bool) string {
	// Replace newlines with spaces unless explicitly included
	if !includeNewline {
		body = strings.ReplaceAll(body, "\n", " ")
		body = strings.ReplaceAll(body, "\r", "")
	}

	// Collapse multiple spaces
	for strings.Contains(body, "  ") {
		body = strings.ReplaceAll(body, "  ", " ")
	}

	body = strings.TrimSpace(body)

	// Truncate if needed
	if maxLen > 0 && len(body) > maxLen {
		if maxLen <= 3 {
			return body[:maxLen]
		}
		return body[:maxLen-3] + "..."
	}

	return body
}
