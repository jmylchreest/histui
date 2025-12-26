// Package tui provides the BubbleTea-based terminal user interface.
package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmylchreest/histui/internal/adapter/input"
	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/model"
	"github.com/jmylchreest/histui/internal/store"
	"gopkg.in/yaml.v3"
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
	list       list.Model
	viewport   viewport.Model
	searchInput textinput.Model
	help       help.Model

	// State
	notifications  []model.Notification
	selected       *model.Notification
	searchQuery    string
	showDismissed  bool
	width          int
	height         int
	ready          bool

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
	itemWidth := m.Width() - d.DefaultDelegate.Styles.NormalTitle.GetHorizontalPadding()

	// Styles
	var titleStyle, descStyle lipgloss.Style

	if isDismissed {
		// Dismissed: dimmed/gray color
		if isSelected {
			titleStyle = d.DefaultDelegate.Styles.SelectedTitle.
				Foreground(lipgloss.Color("8"))
			descStyle = d.DefaultDelegate.Styles.SelectedDesc.
				Foreground(lipgloss.Color("8"))
		} else {
			titleStyle = d.DefaultDelegate.Styles.NormalTitle.
				Foreground(lipgloss.Color("8"))
			descStyle = d.DefaultDelegate.Styles.NormalDesc.
				Foreground(lipgloss.Color("8"))
		}
	} else {
		// Normal: use default delegate styles
		if isSelected {
			titleStyle = d.DefaultDelegate.Styles.SelectedTitle
			descStyle = d.DefaultDelegate.Styles.SelectedDesc
		} else {
			titleStyle = d.DefaultDelegate.Styles.NormalTitle
			descStyle = d.DefaultDelegate.Styles.NormalDesc
		}
	}

	// Build title with optional prefix
	title := ni.Title()
	if isDismissed {
		title = "[d] " + title
	}

	// Truncate if needed
	if itemWidth > 0 && len(title) > itemWidth {
		title = title[:itemWidth-1] + "…"
	}

	desc := ni.Description()
	if itemWidth > 0 && len(desc) > itemWidth {
		desc = desc[:itemWidth-1] + "…"
	}

	// Render using the same structure as DefaultDelegate
	fmt.Fprint(w, titleStyle.Render(title))
	fmt.Fprint(w, "\n")
	fmt.Fprint(w, descStyle.Render(desc))
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

	searchInput := textinput.New()
	searchInput.Placeholder = "Search..."
	searchInput.CharLimit = 100

	h := help.New()

	keys := DefaultKeyMap()

	m := Model{
		cfg:    cfg,
		store:  s,
		mode:   ModeList,
		list:   l,
		searchInput: searchInput,
		help:   h,
		keys:   keys,
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
		}
		return m, nil
	}

	return m, nil
}

// handleListKey handles keys in list mode.
func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
					m.store.Update(n)
					m.notifications = m.fetchNotifications()
					m.list.SetItems(m.buildListItems())
					return m, func() tea.Msg {
						return statusMsg{text: "Notification restored", isErr: false}
					}
				}
				// Dismiss
				m.store.Dismiss(item.notification.HistuiID)
				m.notifications = m.fetchNotifications()
				m.list.SetItems(m.buildListItems())
				return m, func() tea.Msg {
					return statusMsg{text: "Notification dismissed", isErr: false}
				}
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.HardDelete):
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			if m.store != nil {
				m.store.DeleteWithTombstone(item.notification.HistuiID)
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
		var filtered []model.Notification
		query := m.searchQuery
		for _, n := range notifications {
			if containsIgnoreCase(n.Summary, query) ||
				containsIgnoreCase(n.Body, query) ||
				containsIgnoreCase(n.AppName, query) {
				filtered = append(filtered, n)
			}
		}
		notifications = filtered
	}

	items := make([]list.Item, len(notifications))
	for i, n := range notifications {
		items[i] = notificationItem{notification: n, index: i}
	}
	return items
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

	// Extensions
	if n.Extensions != nil {
		s += "\n" + labelStyle.Render("Extensions:") + "\n"
		if n.Extensions.URLs != "" {
			s += "  URLs: " + n.Extensions.URLs + "\n"
		}
		if n.Extensions.Progress > 0 {
			s += fmt.Sprintf("  Progress: %d%%\n", n.Extensions.Progress)
		}
	}

	return s
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

	return s
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

	return searchBar + "\n" + m.list.View() + "\n" + m.buildKeybindBar(m.width, "search")
}

func (m Model) viewHelp() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10"))

	s := titleStyle.Render("Keyboard Shortcuts") + "\n\n"

	s += sectionStyle.Render("Navigation") + "\n"
	s += keyStyle.Render("  j/k, ↑/↓") + "     Move up/down\n"
	s += keyStyle.Render("  g/G") + "          Go to top/bottom\n"
	s += keyStyle.Render("  pgup/pgdn") + "    Page up/down\n"
	s += "\n"

	s += sectionStyle.Render("Actions") + "\n"
	s += keyStyle.Render("  enter") + "        View notification details\n"
	s += keyStyle.Render("  c") + "            Copy body to clipboard\n"
	s += keyStyle.Render("  s") + "            Copy summary to clipboard\n"
	s += keyStyle.Render("  C") + "            Copy all visible as JSON\n"
	s += keyStyle.Render("  alt+c") + "        Copy all visible as YAML\n"
	s += keyStyle.Render("  d") + "            Dismiss notification (hide)\n"
	s += keyStyle.Render("  D") + "            Delete permanently (won't reimport)\n"
	s += keyStyle.Render("  a") + "            Toggle showing dismissed\n"
	s += keyStyle.Render("  /") + "            Search\n"
	s += keyStyle.Render("  r") + "            Refresh from source\n"
	s += "\n"

	s += sectionStyle.Render("General") + "\n"
	s += keyStyle.Render("  ?") + "            Toggle this help\n"
	s += keyStyle.Render("  esc") + "          Back / Cancel\n"
	s += keyStyle.Render("  q") + "            Quit\n"

	s += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(
		"Press ? or esc to return")

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
			{"?", "help", 3},
			{"/", "search", 4},
			{"d", "dismiss", 5},
			{"a", "all", 6},
			{"c", "copy", 7},
			{"s", "summary", 8},
			{"D", "delete", 9},
			{"r", "refresh", 10},
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
	Config        *config.Config
	Store         *store.Store
	Adapter       input.InputAdapter
	PersistPath   string // Path to watch for changes (empty = no watching)
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
		watcher.Stop()
	}

	return err
}
