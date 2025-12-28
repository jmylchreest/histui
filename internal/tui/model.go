// Package tui provides the BubbleTea-based terminal user interface.
package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/blacktop/go-termimg"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"

	"github.com/jmylchreest/histui/internal/adapter/input"
	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/core"
	"github.com/jmylchreest/histui/internal/model"
	"github.com/jmylchreest/histui/internal/store"
)

// Mode represents the current UI mode.
type Mode int

const (
	ModeList Mode = iota
	ModeDetail
	ModeSearch
	ModeHelp
)

// Model is the main TUI model.
type Model struct {
	// Configuration
	cfg   *config.Config
	store *store.Store

	// Current mode
	mode Mode

	// Components
	list        list.Model
	viewport    viewport.Model
	searchInput textinput.Model
	help        help.Model

	// State
	notifications []model.Notification
	selected      *model.Notification
	searchQuery   string
	showDismissed bool
	width         int
	height        int
	ready         bool
	helpPage      int  // 0 = keybindings, 1 = filter reference
	previewActive bool // Show preview panel overlay

	// Key bindings
	keys KeyMap

	// Status message
	statusMsg string
	statusErr bool

	// Refresh channel subscription
	refreshCh <-chan store.ChangeEvent
}

// notificationItem wraps a notification for the list component.
type notificationItem struct {
	notification model.Notification
	index        int
}

func (i notificationItem) Title() string {
	return i.notification.Summary
}

func (i notificationItem) Description() string {
	return fmt.Sprintf("[%s] %s - %s",
		i.notification.AppName,
		i.notification.RelativeTime(),
		i.notification.BodyTruncated(50))
}

func (i notificationItem) FilterValue() string {
	return i.notification.Summary + " " + i.notification.Body + " " + i.notification.AppName
}

// notificationDelegate is a custom list delegate for styling notifications.
type notificationDelegate struct {
	list.DefaultDelegate
}

// newNotificationDelegate creates a new notification delegate.
func newNotificationDelegate() notificationDelegate {
	d := list.NewDefaultDelegate()
	return notificationDelegate{DefaultDelegate: d}
}

// Render renders a list item with custom styling for dismissed notifications.
// All items are rendered consistently to avoid visual glitches.
func (d notificationDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ni, ok := item.(notificationItem)
	if !ok {
		d.DefaultDelegate.Render(w, m, index, item)
		return
	}

	// Check if this item is selected
	isSelected := index == m.Index()
	isDismissed := ni.notification.IsDismissed()

	// Get item width from the list
	itemWidth := m.Width() - d.Styles.NormalTitle.GetHorizontalPadding()

	// Styles
	var titleStyle, descStyle lipgloss.Style

	if isDismissed {
		// Dismissed: dimmed/gray color
		if isSelected {
			titleStyle = d.Styles.SelectedTitle.
				Foreground(lipgloss.Color("8"))
			descStyle = d.Styles.SelectedDesc.
				Foreground(lipgloss.Color("8"))
		} else {
			titleStyle = d.Styles.NormalTitle.
				Foreground(lipgloss.Color("8"))
			descStyle = d.Styles.NormalDesc.
				Foreground(lipgloss.Color("8"))
		}
	} else {
		// Normal: use default delegate styles
		if isSelected {
			titleStyle = d.Styles.SelectedTitle
			descStyle = d.Styles.SelectedDesc
		} else {
			titleStyle = d.Styles.NormalTitle
			descStyle = d.Styles.NormalDesc
		}
	}

	// Status indicator in left margin (2 chars: status + space)
	statusIndicator := "  " // default: empty margin
	if isDismissed {
		statusIndicator = "D "
	}
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// Build title
	title := ni.Title()

	// Truncate if needed (account for status margin)
	effectiveWidth := itemWidth - 2 // subtract margin width
	if effectiveWidth > 0 && len(title) > effectiveWidth {
		title = title[:effectiveWidth-1] + "…"
	}

	desc := ni.Description()
	if effectiveWidth > 0 && len(desc) > effectiveWidth {
		desc = desc[:effectiveWidth-1] + "…"
	}

	// Render with status margin on left
	_, _ = fmt.Fprint(w, statusStyle.Render(statusIndicator)+titleStyle.Render(title))
	_, _ = fmt.Fprint(w, "\n")
	_, _ = fmt.Fprint(w, "  "+descStyle.Render(desc)) // align desc with title
}

// New creates a new TUI model.
func New(cfg *config.Config, s *store.Store) Model {
	// Initialize components with custom delegate for styling
	delegate := newNotificationDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Notification History"
	l.SetShowStatusBar(true)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()

	// Hide the empty status bar message to prevent duplicate "no items" display
	// The list shows both StatusEmpty in the status bar and NoItems in the content area
	// We keep NoItems (centered in content) and hide StatusEmpty (in status bar)
	l.Styles.StatusEmpty = lipgloss.NewStyle()

	searchInput := textinput.New()
	searchInput.Placeholder = "Search or filter (e.g., app=discord)..."
	searchInput.CharLimit = 100

	h := help.New()

	keys := DefaultKeyMap()

	m := Model{
		cfg:           cfg,
		store:         s,
		mode:          ModeList,
		list:          l,
		searchInput:   searchInput,
		help:          h,
		keys:          keys,
		previewActive: true, // Enable preview by default
	}

	// Subscribe to store changes if available
	if s != nil {
		m.refreshCh = s.Subscribe()
	}

	return m
}

// Init initializes the TUI.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadNotifications,
		m.watchForChanges,
	)
}

// loadNotifications fetches notifications from the store.
func (m Model) loadNotifications() tea.Msg {
	return loadNotificationsMsg{}
}

type loadNotificationsMsg struct{}

// watchForChanges watches for store changes.
func (m Model) watchForChanges() tea.Msg {
	if m.refreshCh == nil {
		return nil
	}
	// Wait for a change event
	<-m.refreshCh
	return refreshMsg{}
}

type refreshMsg struct{}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Update component sizes
		m.list.SetSize(msg.Width, msg.Height-2)
		m.viewport = viewport.New(msg.Width, msg.Height-4)
		m.viewport.YPosition = 2

		return m, nil

	case loadNotificationsMsg:
		m.notifications = m.fetchNotifications()
		m.list.SetItems(m.buildListItems())
		return m, nil

	case refreshMsg:
		m.notifications = m.fetchNotifications()
		m.list.SetItems(m.buildListItems())
		return m, m.watchForChanges

	case statusMsg:
		m.statusMsg = msg.text
		m.statusErr = msg.isErr
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{}
		})

	case clearStatusMsg:
		m.statusMsg = ""
		m.statusErr = false
		return m, nil

	case copyResultMsg:
		if msg.err != nil {
			return m, func() tea.Msg {
				return statusMsg{text: "Copy failed: " + msg.err.Error(), isErr: true}
			}
		}
		return m, func() tea.Msg {
			return statusMsg{text: "Copied to clipboard", isErr: false}
		}
	}

	// Update child components
	switch m.mode {
	case ModeList:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	case ModeDetail:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	case ModeSearch:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

type statusMsg struct {
	text  string
	isErr bool
}

type clearStatusMsg struct{}

type copyResultMsg struct {
	err error
}

// handleKey handles key presses.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		if m.mode == ModeHelp {
			m.mode = ModeList
		} else {
			m.mode = ModeHelp
		}
		return m, nil
	}

	// Mode-specific keys
	switch m.mode {
	case ModeList:
		return m.handleListKey(msg)
	case ModeDetail:
		return m.handleDetailKey(msg)
	case ModeSearch:
		return m.handleSearchKey(msg)
	case ModeHelp:
		if key.Matches(msg, m.keys.Back) {
			m.mode = ModeList
			m.helpPage = 0
		}
		// Navigate help pages with left/right or h/l
		switch msg.String() {
		case "left", "h":
			if m.helpPage > 0 {
				m.helpPage--
			}
		case "right", "l", "tab":
			if m.helpPage < 1 {
				m.helpPage++
			}
		}
		return m, nil
	}

	return m, nil
}

// handleListKey handles keys in list mode.
func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Close preview panel with esc
	if m.previewActive && key.Matches(msg, m.keys.Back) {
		m.previewActive = false
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Enter):
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			m.selected = &item.notification
			m.mode = ModeDetail
			m.viewport.SetContent(m.renderDetail(item.notification))
			m.viewport.GotoTop()
		}
		return m, nil

	case key.Matches(msg, m.keys.Copy):
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			return m, m.copyToClipboard(item.notification.Body)
		}
		return m, nil

	case key.Matches(msg, m.keys.CopySummary):
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			return m, m.copyToClipboard(item.notification.Summary)
		}
		return m, nil

	case key.Matches(msg, m.keys.CopyAllJSON):
		// Get currently visible notifications
		items := m.list.Items()
		notifications := make([]model.Notification, 0, len(items))
		for _, item := range items {
			if ni, ok := item.(notificationItem); ok {
				notifications = append(notifications, ni.notification)
			}
		}
		data, err := json.MarshalIndent(notifications, "", "  ")
		if err != nil {
			return m, func() tea.Msg {
				return statusMsg{text: "Failed to marshal JSON: " + err.Error(), isErr: true}
			}
		}
		return m, m.copyToClipboard(string(data))

	case key.Matches(msg, m.keys.CopyAllYAML):
		// Get currently visible notifications
		items := m.list.Items()
		notifications := make([]model.Notification, 0, len(items))
		for _, item := range items {
			if ni, ok := item.(notificationItem); ok {
				notifications = append(notifications, ni.notification)
			}
		}
		data, err := yaml.Marshal(notifications)
		if err != nil {
			return m, func() tea.Msg {
				return statusMsg{text: "Failed to marshal YAML: " + err.Error(), isErr: true}
			}
		}
		return m, m.copyToClipboard(string(data))

	case key.Matches(msg, m.keys.Dismiss):
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			if m.store != nil {
				n := item.notification
				if n.IsDismissed() {
					// Undismiss
					n.Undismiss()
					_ = m.store.Update(n)
					m.notifications = m.fetchNotifications()
					m.list.SetItems(m.buildListItems())
					return m, func() tea.Msg {
						return statusMsg{text: "Notification restored", isErr: false}
					}
				}
				// Dismiss
				_ = m.store.Dismiss(item.notification.HistuiID)
				m.notifications = m.fetchNotifications()
				m.list.SetItems(m.buildListItems())
				return m, func() tea.Msg {
					return statusMsg{text: "Notification dismissed", isErr: false}
				}
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.DismissAll):
		if m.store != nil {
			// Get all currently visible (filtered) items
			items := m.list.Items()
			dismissedCount := 0
			for _, item := range items {
				if ni, ok := item.(notificationItem); ok {
					if !ni.notification.IsDismissed() {
						_ = m.store.Dismiss(ni.notification.HistuiID)
						dismissedCount++
					}
				}
			}
			if dismissedCount > 0 {
				m.notifications = m.fetchNotifications()
				m.list.SetItems(m.buildListItems())
				return m, func() tea.Msg {
					return statusMsg{text: fmt.Sprintf("Dismissed %d notification(s)", dismissedCount), isErr: false}
				}
			}
			return m, func() tea.Msg {
				return statusMsg{text: "No notifications to dismiss", isErr: false}
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.HardDelete):
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			if m.store != nil {
				_ = m.store.DeleteWithTombstone(item.notification.HistuiID)
				m.notifications = m.fetchNotifications()
				m.list.SetItems(m.buildListItems())
			}
		}
		return m, func() tea.Msg {
			return statusMsg{text: "Notification deleted permanently", isErr: false}
		}

	case key.Matches(msg, m.keys.ToggleDismissed):
		m.showDismissed = !m.showDismissed
		m.list.SetItems(m.buildListItems())
		if m.showDismissed {
			return m, func() tea.Msg {
				return statusMsg{text: "Showing all notifications", isErr: false}
			}
		}
		return m, func() tea.Msg {
			return statusMsg{text: "Hiding dismissed notifications", isErr: false}
		}

	case key.Matches(msg, m.keys.Preview):
		m.previewActive = !m.previewActive
		return m, nil

	case key.Matches(msg, m.keys.Search):
		// Reset search when entering search mode
		m.searchInput.SetValue("")
		m.searchQuery = ""
		m.list.SetItems(m.buildListItems())
		m.mode = ModeSearch
		m.searchInput.Focus()
		return m, textinput.Blink

	case key.Matches(msg, m.keys.Refresh):
		return m, m.loadNotifications
	}

	// Pass to list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// handleDetailKey handles keys in detail mode.
func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.mode = ModeList
		m.selected = nil
		return m, nil

	case key.Matches(msg, m.keys.Copy):
		if m.selected != nil {
			return m, m.copyToClipboard(m.selected.Body)
		}
		return m, nil

	case key.Matches(msg, m.keys.CopySummary):
		if m.selected != nil {
			return m, m.copyToClipboard(m.selected.Summary)
		}
		return m, nil

	case key.Matches(msg, m.keys.Search):
		// Go to search mode, reset search and show full list
		m.selected = nil
		m.searchInput.SetValue("")
		m.searchQuery = ""
		m.list.SetItems(m.buildListItems())
		m.mode = ModeSearch
		m.searchInput.Focus()
		return m, textinput.Blink
	}

	// Pass to viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// handleSearchKey handles keys in search mode.
func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// Esc exits search mode and clears search
		m.mode = ModeList
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		m.searchQuery = ""
		m.list.SetItems(m.buildListItems())
		return m, nil

	case tea.KeyEnter:
		// Enter opens the selected notification (like in list mode)
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			m.selected = &item.notification
			m.mode = ModeDetail
			m.searchInput.Blur()
			m.viewport.SetContent(m.renderDetail(item.notification))
			m.viewport.GotoTop()
		}
		return m, nil

	case tea.KeyUp, tea.KeyDown:
		// Allow navigating the list while searching
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	// Pass to text input
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)

	// Live filtering: update search query and rebuild list on each keystroke
	m.searchQuery = m.searchInput.Value()
	m.list.SetItems(m.buildListItems())

	return m, cmd
}

// fetchNotifications gets notifications from the store or directly from dunst.
func (m Model) fetchNotifications() []model.Notification {
	if m.store != nil {
		return m.store.All()
	}
	return nil
}

// buildListItems creates list items from current notifications.
func (m Model) buildListItems() []list.Item {
	notifications := m.notifications

	// Filter out dismissed unless showDismissed is true
	if !m.showDismissed {
		var visible []model.Notification
		for _, n := range notifications {
			if !n.IsDismissed() {
				visible = append(visible, n)
			}
		}
		notifications = visible
	}

	// Apply search filter if active
	if m.searchQuery != "" {
		query := m.searchQuery

		// Detect if this is a filter expression (contains operators like =, ~, <, >)
		if isFilterExpression(query) {
			// Parse and apply filter expression
			if expr, err := core.ParseFilter(query); err == nil {
				notifications = core.FilterWithExpr(notifications, expr)
			}
		} else {
			// Regular text search
			var filtered []model.Notification
			for _, n := range notifications {
				if containsIgnoreCase(n.Summary, query) ||
					containsIgnoreCase(n.Body, query) ||
					containsIgnoreCase(n.AppName, query) {
					filtered = append(filtered, n)
				}
			}
			notifications = filtered
		}
	}

	items := make([]list.Item, len(notifications))
	for i, n := range notifications {
		items[i] = notificationItem{notification: n, index: i}
	}
	return items
}

// isFilterExpression checks if a query looks like a filter expression.
// Filter expressions contain operators like =, !=, ~, ~=, <, >, <=, >=
// and field names like app, summary, body, urgency, etc.
func isFilterExpression(query string) bool {
	// Check for filter operators
	operators := []string{"!=", "~=", ">=", "<=", "=", "~", ">", "<"}
	for _, op := range operators {
		if strings.Contains(query, op) {
			// Verify it looks like a valid filter (field<op>value pattern)
			parts := strings.SplitN(query, op, 2)
			if len(parts) == 2 && len(strings.TrimSpace(parts[0])) > 0 {
				field := strings.ToLower(strings.TrimSpace(parts[0]))
				// Check if the field looks like a valid filter field
				validFields := []string{"app", "summary", "body", "urgency", "category", "dismissed", "seen", "timestamp"}
				for _, vf := range validFields {
					if field == vf || strings.HasPrefix(field, vf+",") || strings.Contains(field, ","+vf) {
						return true
					}
				}
				// Also check if comma-separated multiple conditions
				if strings.Contains(query, ",") {
					return true
				}
			}
		}
	}
	return false
}

// renderDetail renders the detail view for a notification.
func (m Model) renderDetail(n model.Notification) string {
	var s string

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	s += headerStyle.Render(n.Summary) + "\n\n"

	// Metadata
	s += labelStyle.Render("App: ") + n.AppName + "\n"
	s += labelStyle.Render("Time: ") + n.RelativeTime() + "\n"
	s += labelStyle.Render("Urgency: ") + n.UrgencyName + "\n"
	if n.Category != "" {
		s += labelStyle.Render("Category: ") + n.Category + "\n"
	}

	// Body
	s += "\n" + labelStyle.Render("Body:") + "\n"
	s += n.Body + "\n"

	// Extensions - only show if there's something useful
	if n.Extensions != nil {
		var extLines []string

		if n.Extensions.Progress >= 0 {
			extLines = append(extLines, fmt.Sprintf("  Progress: %d%%", n.Extensions.Progress))
		}
		if n.Extensions.URLs != "" {
			extLines = append(extLines, "  URLs: "+n.Extensions.URLs)
		}
		if n.Extensions.StackTag != "" {
			extLines = append(extLines, "  Stack Tag: "+n.Extensions.StackTag)
		}
		if len(n.Extensions.Actions) > 0 {
			var actionLabels []string
			for _, a := range n.Extensions.Actions {
				actionLabels = append(actionLabels, a.Label)
			}
			extLines = append(extLines, "  Actions: "+strings.Join(actionLabels, ", "))
		}
		if n.Extensions.DesktopEntry != "" {
			extLines = append(extLines, "  Desktop: "+n.Extensions.DesktopEntry)
		}

		// Only show Extensions header if we have something to display
		if len(extLines) > 0 {
			s += "\n" + labelStyle.Render("Extensions:") + "\n"
			s += strings.Join(extLines, "\n") + "\n"
		}
	}

	return s
}

// renderPreviewPanel renders the floating preview panel for the selected notification.
func (m Model) renderPreviewPanel(n model.Notification) string {
	// Panel dimensions
	const imgCols = 10
	const imgRows = 5
	const spacing = 2
	const headerTextWidth = 30                                  // Text width next to image
	const panelContentWidth = imgCols + spacing + headerTextWidth // Total content width
	const panelWidth = panelContentWidth + 4                    // +4 for border and padding

	// Styles
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("10")).
		Padding(0, 1).
		Width(panelWidth)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Width(headerTextWidth).
		MaxWidth(headerTextWidth)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Width(headerTextWidth).
		MaxWidth(headerTextWidth)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	// Build image column (left side)
	imageStr := m.renderPreviewImage(n)

	// Build text column (right side) - start 1 row down to align with image
	var textLines []string
	textLines = append(textLines, "") // empty first row

	// Title (truncated to fit)
	title := n.Summary
	if len(title) > headerTextWidth {
		title = title[:headerTextWidth-3] + "..."
	}
	textLines = append(textLines, headerStyle.Render(title))

	// App and time
	meta := n.AppName + " " + dimStyle.Render("|") + " " + n.RelativeTime()
	if len(meta) > headerTextWidth {
		meta = meta[:headerTextWidth-3] + "..."
	}
	textLines = append(textLines, labelStyle.Render(meta))

	// Pad text to match image height if needed
	for len(textLines) < imgRows {
		textLines = append(textLines, strings.Repeat(" ", headerTextWidth))
	}

	// Extract URLs from pango markup before stripping
	urls := extractURLsFromMarkup(n.Body)

	// Body (wrapped) - strip pango markup for plain text display
	// Body spans full panel width
	body := stripPangoMarkup(n.Body)
	body = strings.Join(strings.Fields(body), " ")
	bodyLines := wrapText(body, panelContentWidth)
	maxBodyLines := 3
	if len(bodyLines) > maxBodyLines {
		bodyLines = bodyLines[:maxBodyLines]
		if len(bodyLines[maxBodyLines-1]) > 3 {
			bodyLines[maxBodyLines-1] = bodyLines[maxBodyLines-1][:len(bodyLines[maxBodyLines-1])-3] + "..."
		}
	}

	textContent := strings.Join(textLines, "\n")

	// Join image and text side by side
	content := lipgloss.JoinHorizontal(lipgloss.Top, imageStr, "  ", textContent)

	// Add body below if present
	if len(bodyLines) > 0 {
		bodyContent := strings.Join(bodyLines, "\n")
		content = content + "\n" + bodyContent
	}

	// Add URLs if found (truncated to panel width)
	if len(urls) > 0 {
		urlStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("4")).
			Underline(true).
			MaxWidth(panelContentWidth)
		content = content + "\n"
		for _, url := range urls {
			// Truncate long URLs
			displayURL := url
			if len(displayURL) > panelContentWidth {
				displayURL = displayURL[:panelContentWidth-3] + "..."
			}
			content = content + "\n" + urlStyle.Render(displayURL)
		}
	}

	return borderStyle.Render(content)
}

// renderPreviewImage renders the notification image or a placeholder.
// Uses Halfblocks protocol which renders as ANSI text characters that
// BubbleTea can properly manage during redraws.
func (m Model) renderPreviewImage(n model.Notification) string {
	// Terminal image dimensions: 10 cols x 5 rows
	const imgCols = 10
	const imgRows = 5

	var img *termimg.Image
	var err error

	// Try IconPath first
	if n.IconPath != "" {
		img, err = termimg.Open(n.IconPath)
	}

	// Fall back to ImageData if available
	if (err != nil || img == nil) && n.Extensions != nil && len(n.Extensions.ImageData) > 0 {
		img, err = termimg.From(bytes.NewReader(n.Extensions.ImageData))
	}

	if err == nil && img != nil {
		// Use ScaleFill to ensure image fills the target size (scales up small images)
		rendered, renderErr := img.
			Protocol(termimg.Halfblocks).
			Width(imgCols).
			Height(imgRows).
			Scale(termimg.ScaleFill).
			Render()
		if renderErr == nil && rendered != "" {
			return rendered
		}
	}

	// Fallback: render placeholder
	return renderImagePlaceholder()
}

// renderImagePlaceholder renders a square placeholder using halfblock-style characters.
// Matches the 10 cols x 5 rows dimensions of rendered images.
func renderImagePlaceholder() string {
	// Use dim block characters to create a placeholder that matches halfblock rendering
	// 10 cols x 5 rows with an X pattern
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	lines := []string{
		"▄▄▄▄▄▄▄▄▄▄",
		"█▀      ▀█",
		"█  ▀▄▄▀  █",
		"█▄      ▄█",
		"▀▀▀▀▀▀▀▀▀▀",
	}

	var result []string
	for _, line := range lines {
		result = append(result, dimStyle.Render(line))
	}
	return strings.Join(result, "\n")
}

// wrapText wraps text to the specified width.
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return lines
	}

	currentLine := words[0]
	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

// Regex patterns for pango/HTML markup parsing
var (
	// Match href URLs in anchor tags: <a href="...">
	hrefRegex = regexp.MustCompile(`<a\s+[^>]*href=["']([^"']+)["'][^>]*>`)
	// Match all XML/HTML tags
	tagRegex = regexp.MustCompile(`<[^>]+>`)
)

// stripPangoMarkup removes pango/HTML markup tags and returns plain text.
func stripPangoMarkup(markup string) string {
	// Remove all tags
	text := tagRegex.ReplaceAllString(markup, "")
	// Decode HTML entities using stdlib
	text = html.UnescapeString(text)
	return text
}

// extractURLsFromMarkup extracts URLs from pango/HTML anchor tags.
func extractURLsFromMarkup(markup string) []string {
	matches := hrefRegex.FindAllStringSubmatch(markup, -1)
	var urls []string
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			urls = append(urls, match[1])
			seen[match[1]] = true
		}
	}
	return urls
}

// copyToClipboard copies text to the system clipboard.
func (m Model) copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		err := copyText(text, m.cfg)
		return copyResultMsg{err: err}
	}
}

// View renders the TUI.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	switch m.mode {
	case ModeList:
		return m.viewList()
	case ModeDetail:
		return m.viewDetail()
	case ModeSearch:
		return m.viewSearch()
	case ModeHelp:
		return m.viewHelp()
	default:
		return ""
	}
}

func (m Model) viewList() string {
	var s string
	s += m.list.View()

	// Status bar
	if m.statusMsg != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))
		if m.statusErr {
			statusStyle = statusStyle.Foreground(lipgloss.Color("9"))
		}
		s += "\n" + statusStyle.Render(m.statusMsg)
	} else {
		s += "\n" + m.buildKeybindBar(m.width, "list")
	}

	// Overlay preview panel if active
	if m.previewActive {
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			panel := m.renderPreviewPanel(item.notification)
			// Place panel in top-right corner
			s = m.overlayPanel(s, panel)
		}
	}

	return s
}

// overlayPanel overlays the panel on top of the base content, positioned at top-right.
func (m Model) overlayPanel(base, panel string) string {
	baseLines := strings.Split(base, "\n")
	panelLines := strings.Split(panel, "\n")

	// Calculate panel width (max line length in panel)
	panelWidth := 0
	for _, line := range panelLines {
		lineLen := lipgloss.Width(line)
		if lineLen > panelWidth {
			panelWidth = lineLen
		}
	}

	// Position: top-right with 1 char padding from right edge
	startCol := m.width - panelWidth - 1
	if startCol < 0 {
		startCol = 0
	}
	startRow := 1 // Start 1 row from top

	// Overlay panel onto base
	for i, panelLine := range panelLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}

		baseLine := baseLines[row]
		baseLineWidth := lipgloss.Width(baseLine)

		// Build new line: base content up to panel start, then panel, then rest of base
		var newLine string

		if startCol <= baseLineWidth {
			// Truncate base line at panel start position
			newLine = truncateToWidth(baseLine, startCol)
		} else {
			// Pad base line to reach panel start position
			newLine = baseLine + strings.Repeat(" ", startCol-baseLineWidth)
		}

		newLine += panelLine

		baseLines[row] = newLine
	}

	return strings.Join(baseLines, "\n")
}

// truncateToWidth truncates a string (with ANSI codes) to the specified display width.
func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}

	// Use lipgloss to handle ANSI-aware truncation
	style := lipgloss.NewStyle().MaxWidth(width)
	return style.Render(s)
}

func (m Model) viewDetail() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1)

	header := headerStyle.Render("Notification Detail")

	return header + "\n" + m.viewport.View() + "\n" + m.buildKeybindBar(m.width, "detail")
}

func (m Model) viewSearch() string {
	matchCount := len(m.list.Items())
	countStr := fmt.Sprintf("(%d matches)", matchCount)

	// Show search bar at top, then the filtered list, then keybinds
	searchBar := "Search: " + m.searchInput.View() + " " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(countStr)

	// Show quick filter suggestions when search is empty or typing "app="
	suggestions := ""
	query := m.searchInput.Value()
	if query == "" || strings.HasPrefix(strings.ToLower(query), "app=") {
		apps := core.UniqueApps(m.notifications)
		if len(apps) > 0 {
			dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
			appStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

			// Show up to 8 apps
			maxApps := 8
			if len(apps) < maxApps {
				maxApps = len(apps)
			}

			var appList []string
			for i := 0; i < maxApps; i++ {
				appList = append(appList, appStyle.Render(apps[i]))
			}

			hint := "Apps: "
			if len(apps) > maxApps {
				hint = fmt.Sprintf("Apps (%d): ", len(apps))
			}
			suggestions = dimStyle.Render(hint) + strings.Join(appList, dimStyle.Render(", "))
		}
	}

	result := searchBar
	if suggestions != "" {
		result += "\n" + suggestions
	}

	return result + "\n" + m.list.View() + "\n" + m.buildKeybindBar(m.width, "search")
}

func (m Model) viewHelp() string {
	if m.helpPage == 0 {
		return m.viewHelpKeybindings()
	}
	return m.viewHelpFilters()
}

func (m Model) viewHelpKeybindings() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	s := titleStyle.Render("Keyboard Shortcuts") + dimStyle.Render(" (1/2)") + "\n\n"

	s += sectionStyle.Render("Navigation") + "\n"
	s += keyStyle.Render("  j/k, ↑/↓") + "     Move up/down\n"
	s += keyStyle.Render("  g/G") + "          Go to top/bottom\n"
	s += keyStyle.Render("  pgup/pgdn") + "    Page up/down\n"
	s += "\n"

	s += sectionStyle.Render("Actions") + "\n"
	s += keyStyle.Render("  enter") + "        View details\n"
	s += keyStyle.Render("  p") + "            Preview panel\n"
	s += keyStyle.Render("  c") + "            Copy body\n"
	s += keyStyle.Render("  s") + "            Copy summary\n"
	s += keyStyle.Render("  C") + "            Copy all as JSON\n"
	s += keyStyle.Render("  alt+c") + "        Copy all as YAML\n"
	s += keyStyle.Render("  d") + "            Dismiss/undismiss\n"
	s += keyStyle.Render("  alt+d") + "        Dismiss all visible\n"
	s += keyStyle.Render("  D") + "            Delete permanently\n"
	s += keyStyle.Render("  a") + "            Toggle dismissed\n"
	s += keyStyle.Render("  /") + "            Search/filter\n"
	s += keyStyle.Render("  r") + "            Refresh\n"
	s += "\n"

	s += sectionStyle.Render("General") + "\n"
	s += keyStyle.Render("  ?") + "            This help\n"
	s += keyStyle.Render("  esc") + "          Back\n"
	s += keyStyle.Render("  q") + "            Quit\n"

	s += "\n" + dimStyle.Render("←/→ or h/l: switch pages  ?/esc: close")

	return s
}

func (m Model) viewHelpFilters() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	fieldStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10"))

	opStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("11"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	s := titleStyle.Render("Filter Reference") + dimStyle.Render(" (2/2)") + "\n\n"

	s += sectionStyle.Render("Fields") + "\n"
	s += fieldStyle.Render("  app") + "        Application name\n"
	s += fieldStyle.Render("  summary") + "    Notification title\n"
	s += fieldStyle.Render("  body") + "       Notification body\n"
	s += fieldStyle.Render("  urgency") + "    low, normal, critical\n"
	s += fieldStyle.Render("  category") + "   Notification category\n"
	s += fieldStyle.Render("  dismissed") + "  true/false\n"
	s += fieldStyle.Render("  seen") + "       true/false\n"
	s += fieldStyle.Render("  timestamp") + "  Duration (1h, 7d, 2w)\n"
	s += "\n"

	s += sectionStyle.Render("Operators") + "\n"
	s += opStyle.Render("  =") + "   Equal          " + opStyle.Render("!=") + "  Not equal\n"
	s += opStyle.Render("  ~") + "   Contains       " + opStyle.Render("~=") + "  Regex match\n"
	s += opStyle.Render("  >") + "   Greater than   " + opStyle.Render("<") + "   Less than\n"
	s += opStyle.Render("  >=") + "  Greater/equal  " + opStyle.Render("<=") + "  Less/equal\n"
	s += "\n"

	s += sectionStyle.Render("Examples") + "\n"
	s += "  app=discord\n"
	s += "  body~meeting\n"
	s += "  urgency=critical\n"
	s += "  timestamp<1h          " + dimStyle.Render("(last hour)") + "\n"
	s += "  app=slack,seen=false  " + dimStyle.Render("(multiple)") + "\n"

	s += "\n" + dimStyle.Render("←/→ or h/l: switch pages  ?/esc: close")

	return s
}

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(substr) == 0 ||
			findIgnoreCase(s, substr))
}

func findIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldAt(s, i, substr) {
			return true
		}
	}
	return false
}

func equalFoldAt(s string, start int, substr string) bool {
	for j := 0; j < len(substr); j++ {
		c1 := s[start+j]
		c2 := substr[j]
		if c1 == c2 {
			continue
		}
		// Simple ASCII case folding
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 32
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 32
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

// keybind represents a single keybind with priority for the status bar.
type keybind struct {
	key      string
	desc     string
	priority int // lower = more important (shown first)
}

// buildKeybindBar builds a keybind bar that fits within the given width.
// mode determines which keybinds are shown: "list", "detail", "search"
func (m Model) buildKeybindBar(width int, mode string) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	var binds []keybind

	switch mode {
	case "list":
		// Priority order for list mode (most important first)
		binds = []keybind{
			{"q", "quit", 1},
			{"enter", "view", 2},
			{"p", "preview", 3},
			{"?", "help", 4},
			{"/", "search", 5},
			{"d", "dismiss", 6},
			{"alt+d", "dismiss all", 7},
			{"a", "all", 8},
			{"c", "copy", 9},
			{"s", "summary", 10},
			{"D", "delete", 11},
			{"r", "refresh", 12},
		}
	case "detail":
		binds = []keybind{
			{"q", "quit", 1},
			{"esc", "back", 2},
			{"/", "search", 3},
			{"c", "copy body", 4},
			{"s", "copy summary", 5},
			{"j/k", "scroll", 6},
		}
	case "search":
		binds = []keybind{
			{"enter", "view", 1},
			{"esc", "close", 2},
			{"↑/↓", "navigate", 3},
		}
	}

	// Build the bar, adding keybinds until we run out of space
	const separator = "  "
	result := ""
	for _, b := range binds {
		item := keyStyle.Render(b.key) + " " + b.desc
		plainItem := b.key + " " + b.desc
		testLen := len(result) + len(separator) + len(plainItem)
		if result != "" {
			testLen = len(stripANSI(result)) + len(separator) + len(plainItem)
		}

		if width > 0 && testLen > width {
			break
		}
		if result != "" {
			result += separator
		}
		result += item
	}

	return style.Render(result)
}

// stripANSI removes ANSI escape codes for length calculation.
func stripANSI(s string) string {
	result := make([]byte, 0, len(s))
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if s[i] == 'm' {
				inEscape = false
			}
			continue
		}
		result = append(result, s[i])
	}
	return string(result)
}

// RunOptions configures the TUI.
type RunOptions struct {
	Config      *config.Config
	Store       *store.Store
	Adapter     input.InputAdapter
	PersistPath string // Path to watch for changes (empty = no watching)
}

// Run starts the TUI with the given options.
func Run(opts RunOptions) error {
	s := opts.Store

	// If no store provided, create one
	if s == nil {
		s = store.NewStore(nil)
	}

	// Import from adapter on startup to ensure we have notifications
	if opts.Adapter != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := importFromAdapter(ctx, opts.Adapter, s)
		cancel()
		if err != nil {
			// Log but continue - store might have persisted notifications
			fmt.Fprintf(os.Stderr, "Warning: failed to import notifications: %v\n", err)
		}
	}

	// Start file watcher if persistence path provided
	var watcher *store.FileWatcher
	if opts.PersistPath != "" {
		var err error
		watcher, err = store.NewFileWatcher(s, opts.PersistPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create file watcher: %v\n", err)
		} else {
			if err := watcher.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to start file watcher: %v\n", err)
			}
		}
	}

	m := New(opts.Config, s)
	p := tea.NewProgram(m, tea.WithAltScreen())

	_, err := p.Run()

	// Stop watcher on exit
	if watcher != nil {
		_ = watcher.Stop()
	}

	return err
}
