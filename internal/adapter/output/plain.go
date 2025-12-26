package output

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/jmylchreest/histui/internal/model"
)

// PlainFormatter formats notifications as plain text.
type PlainFormatter struct {
	opts     FormatterOptions
	template *template.Template
}

// NewPlainFormatter creates a new plain text formatter.
func NewPlainFormatter(opts FormatterOptions) *PlainFormatter {
	f := &PlainFormatter{opts: opts}

	// Parse custom template if provided
	if opts.Template != "" {
		tmpl, err := template.New("plain").Funcs(templateFuncs()).Parse(opts.Template)
		if err == nil {
			f.template = tmpl
		}
	}

	return f
}

// Format writes notifications as plain text.
func (f *PlainFormatter) Format(w io.Writer, notifications []model.Notification) error {
	for i, n := range notifications {
		if err := f.formatNotification(w, i+1, &n); err != nil {
			return err
		}
	}
	return nil
}

// formatNotification formats a single notification.
func (f *PlainFormatter) formatNotification(w io.Writer, index int, n *model.Notification) error {
	// Use custom template if available
	if f.template != nil {
		data := templateData{
			Index:        index,
			Notification: n,
			RelativeTime: relativeTime(n.Timestamp),
		}
		return f.template.Execute(w, data)
	}

	// Default format
	var sb strings.Builder

	if f.opts.ShowIndex {
		sb.WriteString(fmt.Sprintf("[%d] ", index))
	}

	if f.opts.ShowApp && n.AppName != "" {
		sb.WriteString(fmt.Sprintf("<%s> ", n.AppName))
	}

	sb.WriteString(n.Summary)

	if f.opts.ShowTime {
		sb.WriteString(fmt.Sprintf(" (%s)", relativeTime(n.Timestamp)))
	}

	sb.WriteString("\n")

	if n.Body != "" {
		body := n.Body
		if !f.opts.IncludeNewline {
			body = strings.ReplaceAll(body, "\n", " ")
		}
		if f.opts.BodyMaxLen > 0 && len(body) > f.opts.BodyMaxLen {
			body = body[:f.opts.BodyMaxLen-3] + "..."
		}
		sb.WriteString("    " + body + "\n")
	}

	_, err := w.Write([]byte(sb.String()))
	return err
}

// FormatField outputs a specific field from a notification.
func FormatField(n *model.Notification, field string) string {
	switch strings.ToLower(field) {
	case "id", "histui_id":
		return n.HistuiID
	case "app", "app_name", "appname":
		return n.AppName
	case "summary":
		return n.Summary
	case "body":
		return n.Body
	case "category":
		return n.Category
	case "icon", "icon_path":
		return n.IconPath
	case "urgency":
		return n.UrgencyName
	case "all", "full":
		return fmt.Sprintf("%s\n%s", n.Summary, n.Body)
	default:
		return n.Summary
	}
}
