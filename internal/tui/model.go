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

	"github.com/jmylchreest/histui/internal/adapter/input"
	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/core"
	"github.com/jmylchreest/histui/internal/db"
	"github.com/jmylchreest/histui/internal/dbus"
	"github.com/jmylchreest/histui/internal/icon"
	"github.com/jmylchreest/histui/internal/theme"
	"github.com/jmylchreest/histui/internal/model"
)

// Mode represents the current UI mode.
type Mode int

const (
	ModeList Mode = iota
	ModeDetail
	ModeSearch
	ModeHelp
	ModeFilter // Filter submenu
)

// FilterMode represents the current filter view.
type FilterMode int

const (
	FilterAll FilterMode = iota
	FilterActive
	FilterUndismissed
	FilterDismissed
)

// Model is the main TUI model.
type Model struct {
	// Configuration
	cfg          *config.Config
	db           *db.DB
	daemon       *dbus.DaemonClient // D-Bus client for histuid communication
	iconResolver *icon.Resolver     // Icon resolver for NerdFont symbols
	themeName    string             // Theme name for loading theme icons

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
	filterMode    FilterMode // Current filter view (All, Active, Undismissed, Dismissed)
	width         int
	height        int
	ready         bool
	helpPage      int  // 0 = keybindings, 1 = filter reference
	previewActive bool // Show preview panel overlay

	// Stack navigation (for Tab cycling through stacked notifications)
	stackedNotifications []model.Notification // Current stack being viewed
	stackIndex           int                  // Current position in stack

	// Detail view image navigation
	detailImages     []imageSource // All available images for current notification
	detailImageIndex int           // Current image being displayed (0-based)

	// Active popup tracking (histui IDs of notifications with active popups)
	activeIDs    map[string]bool
	daemonStatus string // e.g., "[histuid]", "[dunst] STALE", "[offline]"

	// Stack tag counts (stack_tag -> count of notifications with that tag)
	stackTagCounts map[string]int

	// Key bindings
	keys KeyMap

	// Status message
	statusMsg string
	statusErr bool

	// Toast overlay (brief popup notification)
	toastMsg     string
	toastVisible bool
}

// notificationItem wraps a notification for the list component.
type notificationItem struct {
	notification model.Notification
	index        int
	isActive     bool // true if notification has an active popup
	stackCount   int  // number of notifications with same stack_tag (0 if not stacked)
}

func (i notificationItem) Title() string {
	return i.notification.Summary
}

func (i notificationItem) Description() string {
	stackInfo := ""
	if i.stackCount > 1 {
		stackInfo = fmt.Sprintf(" (x%d)", i.stackCount)
	}
	return fmt.Sprintf("[%s] %s%s - %s",
		i.notification.AppName,
		i.notification.RelativeTime(),
		stackInfo,
		i.notification.BodyTruncated(50))
}

func (i notificationItem) FilterValue() string {
	return i.notification.Summary + " " + i.notification.Body + " " + i.notification.AppName
}

// imageSource represents an image that can be displayed in the detail view.
type imageSource struct {
	label    string // Display label (e.g., "image-path", "embedded", "icon")
	path     string // File path (if applicable)
	data     []byte // Raw image data (if applicable)
	hintKey  string // Original hint key (for image-path hints)
	fromDB   bool   // Whether loaded from database
	imageRef string // Database image ref (if fromDB)
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
	isCritical := ni.notification.Urgency == model.UrgencyCritical

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
	} else if isCritical {
		// Critical: red title
		if isSelected {
			titleStyle = d.Styles.SelectedTitle.
				Foreground(lipgloss.Color("9")) // bright red
			descStyle = d.Styles.SelectedDesc
		} else {
			titleStyle = d.Styles.NormalTitle.
				Foreground(lipgloss.Color("9")) // bright red
			descStyle = d.Styles.NormalDesc
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

	// Status indicator in left margin (3 chars: status + 2 spaces)
	// Priority: A (active popup) > D (dismissed) > R (replayed) > 3 spaces
	statusIndicator := "   " // default: empty 3-char margin
	if ni.isActive {
		statusIndicator = "A  "
	} else if isDismissed {
		statusIndicator = "D  "
	} else if ni.notification.IsReplayed() {
		statusIndicator = "R  "
	}
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// Build title - strip pango/HTML markup for plain text display
	title := stripPangoMarkup(ni.Title())

	// Truncate if needed (account for 3-char status margin)
	// Use visual width for proper Unicode handling
	effectiveWidth := itemWidth - 3 // subtract margin width
	if effectiveWidth > 0 && lipgloss.Width(title) > effectiveWidth {
		title = truncateString(title, effectiveWidth-1) + "…"
	}

	// Build description - strip pango/HTML markup for plain text display
	desc := stripPangoMarkup(ni.Description())
	if effectiveWidth > 0 && lipgloss.Width(desc) > effectiveWidth {
		desc = truncateString(desc, effectiveWidth-1) + "…"
	}

	// Render with status margin on left
	_, _ = fmt.Fprint(w, statusStyle.Render(statusIndicator)+titleStyle.Render(title))
	_, _ = fmt.Fprint(w, "\n")
	_, _ = fmt.Fprint(w, "   "+descStyle.Render(desc)) // align desc with title (3-char margin)
}

// New creates a new TUI model.
func New(cfg *config.Config, database *db.DB, daemonClient *dbus.DaemonClient, iconRes *icon.Resolver) Model {
	// Initialize components with custom delegate for styling
	delegate := newNotificationDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Notification History"
	l.SetShowStatusBar(true)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()

	// Hide empty state messages - the (0 matches) count in search mode is sufficient,
	// and in list mode an empty list is self-explanatory
	l.Styles.StatusEmpty = lipgloss.NewStyle()
	l.Styles.NoItems = lipgloss.NewStyle()

	searchInput := textinput.New()
	searchInput.Placeholder = "Search or filter (e.g., app=discord)..."
	searchInput.CharLimit = 100

	h := help.New()

	keys := DefaultKeyMap()

	// Determine theme name for urgency icons
	themeName := "default"
	if cfg != nil && cfg.TUI.Theme != "" {
		themeName = cfg.TUI.Theme
	}

	// Load theme aliases into the resolver
	if iconRes != nil {
		if aliasesData, found := theme.GetEmbeddedAliases(themeName); found {
			if themeAliases, err := icon.LoadThemeAliases(aliasesData); err == nil && len(themeAliases) > 0 {
				iconRes.SetThemeAliases(themeAliases)
			}
		}
	}

	m := Model{
		cfg:           cfg,
		db:            database,
		daemon:        daemonClient,
		iconResolver:  iconRes,
		themeName:     themeName,
		mode:          ModeList,
		list:          l,
		searchInput:   searchInput,
		help:          h,
		keys:          keys,
		filterMode:    FilterUndismissed, // Default to showing undismissed only
		previewActive: true,              // Enable preview by default
		ready:         true,              // Start ready - size will be updated when WindowSizeMsg arrives
		width:         80,                // Default width
		height:        24,                // Default height
	}

	// Set initial size so list is usable immediately
	l.SetSize(80, 22)

	return m
}

// Init initializes the TUI.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadNotifications,
		startFileWatcher(),
		startDBusWatcher(),
		fetchActiveIDs(),
		fetchDaemonStatus(),
	)
}

// loadNotifications fetches notifications from the database.
func (m Model) loadNotifications() tea.Msg {
	return loadNotificationsMsg{}
}

type loadNotificationsMsg struct{}

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
		return m, nil

	case fileWatcherMsg:
		// Preserve the currently selected item's ID
		var selectedID string
		if sel, ok := m.list.SelectedItem().(notificationItem); ok {
			selectedID = sel.notification.HistuiID
		}

		// Refresh notifications
		m.notifications = m.fetchNotifications()
		m.list.SetItems(m.buildListItems())

		// Restore selection by ID
		if selectedID != "" {
			for i, item := range m.list.Items() {
				if ni, ok := item.(notificationItem); ok {
					if ni.notification.HistuiID == selectedID {
						m.list.Select(i)
						break
					}
				}
			}
		}

		// Update selected detail if preview is active
		if m.previewActive {
			if sel, ok := m.list.SelectedItem().(notificationItem); ok {
				n := sel.notification
				m.selected = &n
			}
		}

		// Restart the watcher for the next change
		return m, startFileWatcher()

	case dbusSignalMsg:
		switch msg.signalType {
		case "displayed":
			// Add to active IDs
			if m.activeIDs == nil {
				m.activeIDs = make(map[string]bool)
			}
			m.activeIDs[msg.histuiID] = true
			m.list.SetItems(m.buildListItems())
		case "dismissed":
			// Remove from active IDs
			if m.activeIDs != nil {
				delete(m.activeIDs, msg.histuiID)
			}
			m.list.SetItems(m.buildListItems())
		case "dnd_changed":
			// Could update a DnD indicator in status bar
			// For now, just refresh the list to update styling if needed
			m.list.SetItems(m.buildListItems())
		case "config_changed":
			// Refresh the list
			m.notifications = m.fetchNotifications()
			m.list.SetItems(m.buildListItems())
		}
		// Restart the D-Bus watcher
		return m, startDBusWatcher()

	case dbusRetryMsg:
		// Retry D-Bus watcher connection and also refresh daemon status
		return m, tea.Batch(startDBusWatcher(), fetchDaemonStatus())

	case activeIDsMsg:
		// Update the active IDs map
		m.activeIDs = make(map[string]bool)
		for _, id := range msg.ids {
			m.activeIDs[id] = true
		}
		m.list.SetItems(m.buildListItems())
		return m, nil

	case daemonStatusMsg:
		m.daemonStatus = msg.status
		return m, nil

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

	case toastMsg:
		m.toastMsg = msg.text
		m.toastVisible = true
		return m, tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
			return clearToastMsg{}
		})

	case clearToastMsg:
		m.toastVisible = false
		m.toastMsg = ""
		return m, nil

	case copyResultMsg:
		if msg.err != nil {
			return m, func() tea.Msg {
				return statusMsg{text: "Copy failed: " + msg.err.Error(), isErr: true}
			}
		}
		return m, func() tea.Msg {
			return toastMsg{text: "Copied to clipboard"}
		}

	case replayResultMsg:
		if msg.err != nil {
			return m, func() tea.Msg {
				return statusMsg{text: "Replay failed: " + msg.err.Error(), isErr: true}
			}
		}
		// Refresh to show the R indicator
		m.notifications = m.fetchNotifications()
		m.list.SetItems(m.buildListItems())
		return m, func() tea.Msg {
			return statusMsg{text: "Notification replayed", isErr: false}
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

type toastMsg struct {
	text string
}

type clearToastMsg struct{}

type copyResultMsg struct {
	err error
}

type replayResultMsg struct {
	err error
}

// handleKey handles key presses.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// In search mode, only handle Esc/Enter/navigation - let text input handle everything else
	if m.mode == ModeSearch {
		return m.handleSearchKey(msg)
	}

	// Help toggle is global (except in search mode)
	if key.Matches(msg, m.keys.Help) {
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
		// Only quit from list mode
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		return m.handleListKey(msg)

	case ModeDetail:
		// q in detail mode goes back to list (handled in handleDetailKey)
		return m.handleDetailKey(msg)

	case ModeFilter:
		return m.handleFilterKey(msg)

	case ModeHelp:
		if key.Matches(msg, m.keys.Back) || key.Matches(msg, m.keys.Quit) {
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
	// Quick filters (1-4)
	case key.Matches(msg, m.keys.FilterAll):
		m.filterMode = FilterAll
		m.list.SetItems(m.buildListItems())
		return m, func() tea.Msg {
			return statusMsg{text: "Filter: All notifications", isErr: false}
		}

	case key.Matches(msg, m.keys.FilterActive):
		m.filterMode = FilterActive
		m.list.SetItems(m.buildListItems())
		return m, func() tea.Msg {
			return statusMsg{text: "Filter: Active only", isErr: false}
		}

	case key.Matches(msg, m.keys.FilterUndismissed):
		m.filterMode = FilterUndismissed
		m.list.SetItems(m.buildListItems())
		return m, func() tea.Msg {
			return statusMsg{text: "Filter: Undismissed", isErr: false}
		}

	case key.Matches(msg, m.keys.FilterDismissed):
		m.filterMode = FilterDismissed
		m.list.SetItems(m.buildListItems())
		return m, func() tea.Msg {
			return statusMsg{text: "Filter: Dismissed only", isErr: false}
		}

	case key.Matches(msg, m.keys.Filter):
		m.mode = ModeFilter
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			m.selected = &item.notification
			m.mode = ModeDetail
			m.viewport.SetContent(m.renderDetail(item.notification))
			m.viewport.GotoTop()
			// Load stacked notifications for Tab cycling
			m.loadStackedNotifications(item)
			// Load images for Left/Right cycling
			m.detailImages = m.collectImages(item.notification)
			m.detailImageIndex = 0
		}
		return m, nil

	case key.Matches(msg, m.keys.Yank):
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			return m, m.copyToClipboard(item.notification.Body)
		}
		return m, nil

	case key.Matches(msg, m.keys.YankAll):
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

	case key.Matches(msg, m.keys.YankImage):
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			if !hasImageWithDB(item.notification, m.db) {
				return m, func() tea.Msg {
					return statusMsg{text: "No image available", isErr: true}
				}
			}
			return m, m.copyImageToClipboard(item.notification)
		}
		return m, nil

	case key.Matches(msg, m.keys.Dismiss):
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			if m.db != nil {
				n := item.notification
				if n.IsDismissed() {
					// Undismiss
					n.Undismiss()
					_ = m.db.UpdateNotification(&n)
					m.notifications = m.fetchNotifications()
					m.list.SetItems(m.buildListItems())
					return m, func() tea.Msg {
						return statusMsg{text: "Notification restored", isErr: false}
					}
				}
				// Dismiss in database
				_ = m.db.DismissNotification(item.notification.HistuiID)
				m.notifications = m.fetchNotifications()
				m.list.SetItems(m.buildListItems())

				// Also close the popup if it's active
				if item.isActive && m.daemon != nil {
					_, _ = m.daemon.CloseNotification(item.notification.HistuiID)
				}

				return m, func() tea.Msg {
					return statusMsg{text: "Notification dismissed", isErr: false}
				}
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.Delete):
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			if m.db != nil {
				// Close the popup first if it's active
				if item.isActive && m.daemon != nil {
					_, _ = m.daemon.CloseNotification(item.notification.HistuiID)
				}

				_ = m.db.DeleteNotification(item.notification.HistuiID)
				m.notifications = m.fetchNotifications()
				m.list.SetItems(m.buildListItems())

				return m, func() tea.Msg {
					return statusMsg{text: "Notification deleted permanently", isErr: false}
				}
			}
		}
		return m, func() tea.Msg {
			return statusMsg{text: "Notification deleted permanently", isErr: false}
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

	case key.Matches(msg, m.keys.Replay):
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			return m, m.replayNotification(item.notification)
		}
		return m, nil

	case key.Matches(msg, m.keys.NextStack), key.Matches(msg, m.keys.PrevStack):
		// Tab cycling through stacked items in list view
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			if item.stackCount > 1 {
				return m.cycleStack(msg)
			}
		}
		return m, nil
	}

	// Pass to list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// loadStackedNotifications loads notifications for Tab cycling.
func (m *Model) loadStackedNotifications(item notificationItem) {
	m.stackedNotifications = nil
	m.stackIndex = 0

	if item.stackCount <= 1 || m.db == nil {
		return
	}

	n := item.notification
	if n.Extensions == nil || n.Extensions.StackTag == "" {
		return
	}

	stacked, err := m.db.GetByStackTag(n.Extensions.StackTag)
	if err != nil || len(stacked) <= 1 {
		return
	}

	m.stackedNotifications = stacked

	// Find current position in stack
	for i, sn := range stacked {
		if sn.HistuiID == n.HistuiID {
			m.stackIndex = i
			break
		}
	}
}

// cycleStack cycles through stacked notifications.
func (m Model) cycleStack(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.stackedNotifications) <= 1 {
		return m, nil
	}

	if key.Matches(msg, m.keys.NextStack) {
		m.stackIndex = (m.stackIndex + 1) % len(m.stackedNotifications)
	} else {
		m.stackIndex--
		if m.stackIndex < 0 {
			m.stackIndex = len(m.stackedNotifications) - 1
		}
	}

	// Find and select the notification in the list
	targetID := m.stackedNotifications[m.stackIndex].HistuiID
	for i, item := range m.list.Items() {
		if ni, ok := item.(notificationItem); ok {
			if ni.notification.HistuiID == targetID {
				m.list.Select(i)
				break
			}
		}
	}

	return m, func() tea.Msg {
		return statusMsg{
			text:  fmt.Sprintf("Stack %d/%d", m.stackIndex+1, len(m.stackedNotifications)),
			isErr: false,
		}
	}
}

// handleDetailKey handles keys in detail mode.
func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Quit):
		// Both esc and q go back to list from detail view
		m.mode = ModeList
		m.selected = nil
		return m, nil

	case key.Matches(msg, m.keys.Yank):
		if m.selected != nil {
			return m, m.copyToClipboard(m.selected.Body)
		}
		return m, nil

	case key.Matches(msg, m.keys.YankAll):
		if m.selected != nil {
			data, err := json.MarshalIndent(m.selected, "", "  ")
			if err != nil {
				return m, func() tea.Msg {
					return statusMsg{text: "Failed to marshal JSON: " + err.Error(), isErr: true}
				}
			}
			return m, m.copyToClipboard(string(data))
		}
		return m, nil

	case key.Matches(msg, m.keys.YankImage):
		if m.selected != nil {
			if !hasImageWithDB(*m.selected, m.db) {
				return m, func() tea.Msg {
					return statusMsg{text: "No image available", isErr: true}
				}
			}
			return m, m.copyImageToClipboard(*m.selected)
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

	case key.Matches(msg, m.keys.NextStack), key.Matches(msg, m.keys.PrevStack):
		// Tab cycling in detail view
		if len(m.stackedNotifications) > 1 {
			return m.cycleStackInDetail(msg)
		}
		return m, nil
	}

	// Handle Left/Right for image cycling (check raw key since not in KeyMap)
	switch msg.String() {
	case "left", "h":
		if len(m.detailImages) > 1 {
			m.detailImageIndex--
			if m.detailImageIndex < 0 {
				m.detailImageIndex = len(m.detailImages) - 1
			}
			return m, func() tea.Msg {
				return statusMsg{
					text:  fmt.Sprintf("Image %d/%d: %s", m.detailImageIndex+1, len(m.detailImages), m.detailImages[m.detailImageIndex].label),
					isErr: false,
				}
			}
		}
	case "right", "l":
		if len(m.detailImages) > 1 {
			m.detailImageIndex = (m.detailImageIndex + 1) % len(m.detailImages)
			return m, func() tea.Msg {
				return statusMsg{
					text:  fmt.Sprintf("Image %d/%d: %s", m.detailImageIndex+1, len(m.detailImages), m.detailImages[m.detailImageIndex].label),
					isErr: false,
				}
			}
		}
	}

	// Pass to viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// cycleStackInDetail cycles through stacked notifications while in detail view.
func (m Model) cycleStackInDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.stackedNotifications) <= 1 {
		return m, nil
	}

	if key.Matches(msg, m.keys.NextStack) {
		m.stackIndex = (m.stackIndex + 1) % len(m.stackedNotifications)
	} else {
		m.stackIndex--
		if m.stackIndex < 0 {
			m.stackIndex = len(m.stackedNotifications) - 1
		}
	}

	// Update the detail view with the new notification
	n := m.stackedNotifications[m.stackIndex]
	m.selected = &n
	m.viewport.SetContent(m.renderDetail(n))
	m.viewport.GotoTop()

	// Reload images for the new notification
	m.detailImages = m.collectImages(n)
	m.detailImageIndex = 0

	return m, func() tea.Msg {
		return statusMsg{
			text:  fmt.Sprintf("Stack %d/%d", m.stackIndex+1, len(m.stackedNotifications)),
			isErr: false,
		}
	}
}

// handleFilterKey handles keys in filter submenu mode.
func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Quit):
		m.mode = ModeList
		return m, nil

	case key.Matches(msg, m.keys.FilterAll):
		m.filterMode = FilterAll
		m.mode = ModeList
		m.list.SetItems(m.buildListItems())
		return m, func() tea.Msg {
			return statusMsg{text: "Filter: All notifications", isErr: false}
		}

	case key.Matches(msg, m.keys.FilterActive):
		m.filterMode = FilterActive
		m.mode = ModeList
		m.list.SetItems(m.buildListItems())
		return m, func() tea.Msg {
			return statusMsg{text: "Filter: Active only", isErr: false}
		}

	case key.Matches(msg, m.keys.FilterUndismissed):
		m.filterMode = FilterUndismissed
		m.mode = ModeList
		m.list.SetItems(m.buildListItems())
		return m, func() tea.Msg {
			return statusMsg{text: "Filter: Undismissed", isErr: false}
		}

	case key.Matches(msg, m.keys.FilterDismissed):
		m.filterMode = FilterDismissed
		m.mode = ModeList
		m.list.SetItems(m.buildListItems())
		return m, func() tea.Msg {
			return statusMsg{text: "Filter: Dismissed only", isErr: false}
		}

	// Also allow single letter shortcuts in filter mode
	default:
		switch msg.String() {
		case "a":
			m.filterMode = FilterAll
			m.mode = ModeList
			m.list.SetItems(m.buildListItems())
			return m, func() tea.Msg {
				return statusMsg{text: "Filter: All notifications", isErr: false}
			}
		case "v": // "visible" / active
			m.filterMode = FilterActive
			m.mode = ModeList
			m.list.SetItems(m.buildListItems())
			return m, func() tea.Msg {
				return statusMsg{text: "Filter: Active only", isErr: false}
			}
		case "u":
			m.filterMode = FilterUndismissed
			m.mode = ModeList
			m.list.SetItems(m.buildListItems())
			return m, func() tea.Msg {
				return statusMsg{text: "Filter: Undismissed", isErr: false}
			}
		case "d":
			m.filterMode = FilterDismissed
			m.mode = ModeList
			m.list.SetItems(m.buildListItems())
			return m, func() tea.Msg {
				return statusMsg{text: "Filter: Dismissed only", isErr: false}
			}
		}
	}

	return m, nil
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
			// Load images for Left/Right cycling
			m.detailImages = m.collectImages(item.notification)
			m.detailImageIndex = 0
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

// fetchNotifications gets notifications from the database.
func (m Model) fetchNotifications() []model.Notification {
	if m.db != nil {
		notifications, err := m.db.All()
		if err != nil {
			return nil
		}
		return notifications
	}
	return nil
}

// buildListItems creates list items from current notifications.
func (m *Model) buildListItems() []list.Item {
	notifications := m.notifications

	// Apply filter mode
	switch m.filterMode {
	case FilterAll:
		// Show everything
	case FilterActive:
		// Only show notifications with active popups
		var visible []model.Notification
		for _, n := range notifications {
			if m.activeIDs != nil && m.activeIDs[n.HistuiID] {
				visible = append(visible, n)
			}
		}
		notifications = visible
	case FilterUndismissed:
		// Only show non-dismissed
		var visible []model.Notification
		for _, n := range notifications {
			if !n.IsDismissed() {
				visible = append(visible, n)
			}
		}
		notifications = visible
	case FilterDismissed:
		// Only show dismissed
		var visible []model.Notification
		for _, n := range notifications {
			if n.IsDismissed() {
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

	// Refresh stack tag counts if we have a database
	if m.db != nil {
		if counts, err := m.db.GetStackTagCounts(); err == nil {
			m.stackTagCounts = counts
		}
	}

	items := make([]list.Item, len(notifications))
	for i, n := range notifications {
		isActive := m.activeIDs != nil && m.activeIDs[n.HistuiID]
		stackCount := 0
		if n.Extensions != nil && n.Extensions.StackTag != "" && m.stackTagCounts != nil {
			stackCount = m.stackTagCounts[n.Extensions.StackTag]
		}
		items[i] = notificationItem{notification: n, index: i, isActive: isActive, stackCount: stackCount}
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

	// Build header/metadata section
	s += headerStyle.Render(n.Summary) + "\n\n"
	s += labelStyle.Render("App: ") + n.AppName + "\n"
	s += labelStyle.Render("Time: ") + n.RelativeTime() + "\n"
	s += labelStyle.Render("Urgency: ") + n.UrgencyName + "\n"
	if n.Category != "" {
		s += labelStyle.Render("Category: ") + n.Category + "\n"
	}

	// Body - strip pango/HTML markup for plain text display
	s += "\n" + labelStyle.Render("Body:") + "\n"
	s += stripPangoMarkup(n.Body) + "\n"

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

	// Original Hints - show the raw hints that were received
	if len(n.OriginalHints) > 0 {
		s += "\n" + labelStyle.Render("Original Hints:") + "\n"
		for key, value := range n.OriginalHints {
			s += fmt.Sprintf("  %s: %v\n", key, value)
		}
	}

	// Stacked Notifications - show other notifications with the same stack_tag
	if n.Extensions != nil && n.Extensions.StackTag != "" && m.db != nil {
		if m.stackTagCounts != nil && m.stackTagCounts[n.Extensions.StackTag] > 1 {
			stackedNotifications, err := m.db.GetByStackTag(n.Extensions.StackTag)
			if err == nil && len(stackedNotifications) > 1 {
				stackStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("11"))
				s += "\n" + labelStyle.Render(fmt.Sprintf("Stacked Notifications (%d):", len(stackedNotifications))) + "\n"
				for _, sn := range stackedNotifications {
					marker := "  "
					if sn.HistuiID == n.HistuiID {
						marker = "> "
					}
					timestamp := sn.TimestampTime().Format("2006-01-02 15:04:05")
					s += fmt.Sprintf("%s%s  %s\n", marker, stackStyle.Render(sn.HistuiID[:8]+"..."), timestamp)
				}
			}
		}
	}

	return s
}

// renderPreviewPanel renders the floating preview panel for the selected notification.
func (m Model) renderPreviewPanel(item notificationItem) string {
	n := item.notification
	// Panel dimensions
	const imgCols = 10
	const imgRows = 5
	const spacing = 2
	const headerTextWidth = 30                                    // Text width next to image
	const panelContentWidth = imgCols + spacing + headerTextWidth // Total content width
	const panelWidth = panelContentWidth + 4                      // +4 for border and padding

	// Styles
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("10")).
		Padding(1, 2). // Vertical and horizontal padding for breathing room
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

	// Build image column (left side) - only if we have an icon
	imageStr := m.renderPreviewIcon(n)
	hasIcon := imageStr != ""

	// Adjust text width based on whether we have an icon
	textWidth := headerTextWidth
	if !hasIcon {
		textWidth = panelContentWidth
	}

	// Build text column - start 1 row down to align with image (if present)
	var textLines []string
	if hasIcon {
		textLines = append(textLines, "") // empty first row for icon alignment
	}

	// Title (truncated to fit)
	title := n.Summary
	if len(title) > textWidth {
		title = title[:textWidth-3] + "..."
	}
	textLines = append(textLines, headerStyle.Render(title))

	// App and time
	meta := n.AppName + " " + dimStyle.Render("|") + " " + n.RelativeTime()
	if len(meta) > textWidth {
		meta = meta[:textWidth-3] + "..."
	}
	textLines = append(textLines, labelStyle.Render(meta))

	// Pad text to match image height if icon present
	if hasIcon {
		for len(textLines) < imgRows {
			textLines = append(textLines, strings.Repeat(" ", textWidth))
		}
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

	// Join image and text side by side (or just text if no icon)
	var content string
	if hasIcon {
		content = lipgloss.JoinHorizontal(lipgloss.Top, imageStr, "  ", textContent)
	} else {
		content = textContent
	}

	// Add body below if present (with blank line separator)
	if len(bodyLines) > 0 {
		bodyContent := strings.Join(bodyLines, "\n")
		content = content + "\n\n" + bodyContent
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

	// Add stacked notifications section if this is part of a stack
	if item.stackCount > 1 && m.db != nil && n.Extensions != nil && n.Extensions.StackTag != "" {
		stackedNotifications, err := m.db.GetByStackTag(n.Extensions.StackTag)
		if err == nil && len(stackedNotifications) > 1 {
			stackHeaderStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("8"))
			stackItemStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("7")).
				MaxWidth(panelContentWidth)
			dimStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("8"))

			content = content + "\n\n" + stackHeaderStyle.Render(fmt.Sprintf("Stack (%d):", len(stackedNotifications)))
			for _, sn := range stackedNotifications {
				// Mark current notification
				prefix := "  "
				if sn.HistuiID == n.HistuiID {
					prefix = "> "
				}
				// Format: > 5m ago - summary truncated
				stackLine := prefix + dimStyle.Render(sn.RelativeTime()) + " " + sn.Summary
				if len(stackLine) > panelContentWidth {
					stackLine = stackLine[:panelContentWidth-3] + "..."
				}
				content = content + "\n" + stackItemStyle.Render(stackLine)
			}
		}
	}

	return borderStyle.Render(content)
}

// renderImageFromSource renders an image from an imageSource using halfblocks.
// Uses a larger size than preview (30x15 chars for more detail).
// Returns empty string if no image is available.
// Handles SVG conversion automatically via the imgconv module.
func (m Model) renderImageFromSource(src imageSource, histuiID string) string {
	// Terminal image dimensions: 30 cols x 15 rows for detail view
	const imgCols = 30
	const imgRows = 15
	const imgPixelWidth = imgCols * 2
	const imgPixelHeight = imgRows * 2
	// Size for SVG rasterization (higher for better quality)
	const svgRasterSize = 128

	var img *termimg.Image
	var err error

	// Load image based on source type
	if src.path != "" {
		// Check if path is SVG and convert first
		if isSVGPath(src.path) {
			pngData, rasterErr := RasterizeSVGFile(src.path, svgRasterSize, DefaultIconColor())
			if rasterErr == nil {
				img, err = termimg.From(bytes.NewReader(pngData))
			}
		} else {
			img, err = termimg.Open(src.path)
		}
	} else if len(src.data) > 0 {
		// Check if data is SVG and convert first
		if isSVGData(src.data) {
			pngData, rasterErr := RasterizeSVG(src.data, svgRasterSize, DefaultIconColor())
			if rasterErr == nil {
				img, err = termimg.From(bytes.NewReader(pngData))
			}
		} else {
			img, err = termimg.From(bytes.NewReader(src.data))
		}
	} else if src.fromDB && m.db != nil {
		data, loadErr := m.db.LoadImage(histuiID, src.imageRef)
		if loadErr == nil && len(data) > 0 {
			// Check if DB data is SVG and convert
			if isSVGData(data) {
				pngData, rasterErr := RasterizeSVG(data, svgRasterSize, DefaultIconColor())
				if rasterErr == nil {
					img, err = termimg.From(bytes.NewReader(pngData))
				}
			} else {
				img, err = termimg.From(bytes.NewReader(data))
			}
		}
	}

	if err == nil && img != nil {
		rendered, renderErr := img.
			Protocol(termimg.Halfblocks).
			Width(imgPixelWidth).
			Height(imgPixelHeight).
			Scale(termimg.ScaleFit). // Preserve aspect ratio, fit within bounds
			Render()
		if renderErr == nil && rendered != "" {
			lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
			if len(lines) > imgRows {
				lines = lines[:imgRows]
			}
			return strings.Join(lines, "\n")
		}
	}

	return ""
}

// renderDetailImagePanel renders a floating panel with the current image for the detail view.
// Shows the image in a bordered box similar to the preview panel.
// Displays navigation info when multiple images are available.
func (m Model) renderDetailImagePanel(n model.Notification) string {
	// Use collected images from model
	if len(m.detailImages) == 0 {
		return ""
	}

	// Get current image
	idx := m.detailImageIndex
	if idx < 0 || idx >= len(m.detailImages) {
		idx = 0
	}
	src := m.detailImages[idx]

	// Render the image
	imageStr := m.renderImageFromSource(src, n.HistuiID)
	if imageStr == "" {
		return ""
	}

	// Panel styles
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("10")).
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	// Build title with navigation info
	title := src.label
	if len(m.detailImages) > 1 {
		title = fmt.Sprintf("%s (%d/%d)", src.label, idx+1, len(m.detailImages))
	}

	// Build content
	content := titleStyle.Render(title)
	if len(m.detailImages) > 1 {
		content += " " + dimStyle.Render("←/→ cycle")
	}
	content += "\n\n" + imageStr

	// Add image list if multiple images
	if len(m.detailImages) > 1 {
		content += "\n"
		for i, img := range m.detailImages {
			marker := "  "
			style := dimStyle
			if i == idx {
				marker = "> "
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
			}
			label := img.label
			if img.path != "" {
				// Show shortened path
				parts := strings.Split(img.path, "/")
				if len(parts) > 0 {
					label = parts[len(parts)-1]
					if len(label) > 20 {
						label = label[:17] + "..."
					}
				}
			}
			content += "\n" + marker + style.Render(label)
		}
	}

	return borderStyle.Render(content)
}

// collectImages gathers all available image sources for a notification.
// Returns images in priority order: image-path hints, embedded ImageData, IconPath, database images.
func (m Model) collectImages(n model.Notification) []imageSource {
	var images []imageSource

	// 1. Check OriginalHints for image-path hints (these are file paths)
	if n.OriginalHints != nil {
		// Look for image-path hints (can be multiple with different keys)
		imageHintKeys := []string{"image-path", "image_path"}
		for _, key := range imageHintKeys {
			if val, ok := n.OriginalHints[key]; ok {
				if path, isStr := val.(string); isStr && path != "" {
					// Verify file exists
					if _, err := os.Stat(path); err == nil {
						images = append(images, imageSource{
							label:   "image-path",
							path:    path,
							hintKey: key,
						})
					}
				}
			}
		}
	}

	// 2. Check embedded ImageData in Extensions
	if n.Extensions != nil && len(n.Extensions.ImageData) > 0 {
		images = append(images, imageSource{
			label: "embedded",
			data:  n.Extensions.ImageData,
		})
	}

	// 3. Check IconPath (file path to icon)
	if n.IconPath != "" {
		if _, err := os.Stat(n.IconPath); err == nil {
			images = append(images, imageSource{
				label: "icon",
				path:  n.IconPath,
			})
		}
	}

	// 4. Check database for stored images
	if m.db != nil {
		// Check for "image" ref
		if has, _ := m.db.HasImage(n.HistuiID, db.ImageRefImage); has {
			images = append(images, imageSource{
				label:    "stored-image",
				fromDB:   true,
				imageRef: db.ImageRefImage,
			})
		}
		// Check for "icon" ref (if not already added from IconPath)
		if has, _ := m.db.HasImage(n.HistuiID, db.ImageRefIcon); has {
			// Only add if we don't already have an icon path
			hasIconPath := false
			for _, img := range images {
				if img.label == "icon" {
					hasIconPath = true
					break
				}
			}
			if !hasIconPath {
				images = append(images, imageSource{
					label:    "stored-icon",
					fromDB:   true,
					imageRef: db.ImageRefIcon,
				})
			}
		}
	}

	return images
}

// renderPreviewIcon renders the notification icon as a NerdFont symbol using halfblocks.
// This shows the app icon that matches what histuid displays in popups.
// Falls back to theme urgency icons if no NerdFont symbol is available.
// Returns empty string if no icon can be rendered.
func (m Model) renderPreviewIcon(n model.Notification) string {
	// Dimensions for the icon (matches image placeholder)
	const iconCols = 10
	const iconRows = 5

	// Get the NerdFont symbol for this notification
	symbol := ""

	if m.iconResolver != nil {
		// Try app name first
		appName := strings.ToLower(n.AppName)
		symbol = m.iconResolver.GetNerdSymbol(appName)

		// Try resolved icon name
		if symbol == "" {
			resolved := m.iconResolver.Resolve(appName)
			if resolved != appName {
				symbol = m.iconResolver.GetNerdSymbol(resolved)
			}
		}

		// Try category
		if symbol == "" && n.Category != "" {
			symbol = m.iconResolver.GetNerdSymbolForCategory(n.Category)
		}
	}

	// If we have a symbol, try to render it
	if symbol != "" {
		rendered := renderNerdSymbolToHalfblocks(symbol, iconCols, iconRows)
		if rendered != "" {
			return rendered
		}
	}

	// Try theme urgency icon as fallback
	if m.iconResolver != nil {
		urgencyIconName := m.iconResolver.GetUrgencyIconName(n.Urgency)
		if urgencyIconName != "" {
			// Try to load the icon from embedded themes
			rendered := m.renderThemeIcon(urgencyIconName, iconCols, iconRows)
			if rendered != "" {
				return rendered
			}
		}
	}

	// Fall back to urgency-based NerdFont symbol
	symbol = icon.FallbackNerdSymbolForUrgency(n.Urgency)
	if symbol != "" {
		rendered := renderNerdSymbolToHalfblocks(symbol, iconCols, iconRows)
		if rendered != "" {
			return rendered
		}
	}

	// Final fallback: render symbol as text in a simple box
	if symbol == "" {
		return ""
	}

	iconStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	lines := make([]string, iconRows)
	border := strings.Repeat("─", iconCols-2)

	lines[0] = dimStyle.Render("╭" + border + "╮")
	lines[1] = dimStyle.Render("│") + strings.Repeat(" ", (iconCols-4)/2) + iconStyle.Render(symbol) + strings.Repeat(" ", (iconCols-4)/2) + dimStyle.Render("│")
	lines[2] = dimStyle.Render("│") + strings.Repeat(" ", iconCols-2) + dimStyle.Render("│")
	lines[3] = dimStyle.Render("│") + strings.Repeat(" ", iconCols-2) + dimStyle.Render("│")
	lines[4] = dimStyle.Render("╰" + border + "╯")

	return strings.Join(lines, "\n")
}

// renderThemeIcon renders a theme icon (SVG) as halfblocks.
// Returns empty string if the icon cannot be loaded or rendered.
func (m Model) renderThemeIcon(iconName string, cols, rows int) string {
	// Size for rasterization (higher for better quality)
	const rasterSize = 64

	// Try to load icon from embedded themes
	iconData, ext, found := theme.GetEmbeddedIcon(m.themeName, iconName)
	if !found {
		return ""
	}

	// Convert SVG to PNG if needed
	var imgData []byte
	if ext == ".svg" {
		pngData, err := RasterizeSVG(iconData, rasterSize, DefaultIconColor())
		if err != nil {
			return ""
		}
		imgData = pngData
	} else {
		// Already a raster format
		imgData = iconData
	}

	// Render using termimg
	img, err := termimg.From(bytes.NewReader(imgData))
	if err != nil {
		return ""
	}

	rendered, err := img.
		Protocol(termimg.Halfblocks).
		Width(cols * 2).
		Height(rows * 2).
		Scale(termimg.ScaleFit).
		Render()
	if err != nil {
		return ""
	}

	// Trim and limit to expected rows
	lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	if len(lines) > rows {
		lines = lines[:rows]
	}

	return strings.Join(lines, "\n")
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

// copyImageToClipboard copies the notification's image to the system clipboard.
func (m Model) copyImageToClipboard(n model.Notification) tea.Cmd {
	// Capture database reference for the closure
	database := m.db

	return func() tea.Msg {
		// Try IconPath first
		if n.IconPath != "" {
			err := copyImageFromPath(n.IconPath)
			if err == nil {
				return copyResultMsg{err: nil}
			}
		}

		// Try embedded ImageData in Extensions
		if n.Extensions != nil && len(n.Extensions.ImageData) > 0 {
			mimeType := "image/png"
			if len(n.Extensions.ImageData) >= 3 &&
				n.Extensions.ImageData[0] == 0xFF &&
				n.Extensions.ImageData[1] == 0xD8 &&
				n.Extensions.ImageData[2] == 0xFF {
				mimeType = "image/jpeg"
			}
			err := copyImage(n.Extensions.ImageData, mimeType)
			return copyResultMsg{err: err}
		}

		// Try loading from database
		if database != nil {
			if data, err := database.LoadImage(n.HistuiID, db.ImageRefImage); err == nil && len(data) > 0 {
				mimeType := "image/png"
				if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
					mimeType = "image/jpeg"
				}
				err := copyImage(data, mimeType)
				return copyResultMsg{err: err}
			}
		}

		return copyResultMsg{err: fmt.Errorf("no image available")}
	}
}

// replayNotification sends the notification to the D-Bus daemon.
func (m Model) replayNotification(n model.Notification) tea.Cmd {
	return func() tea.Msg {
		// Create a replayer using the database for images
		replayer, err := dbus.NewReplayer(m.db)
		if err != nil {
			return replayResultMsg{err: err}
		}
		defer func() { _ = replayer.Close() }()

		_, err = replayer.Replay(&n)
		return replayResultMsg{err: err}
	}
}

// View renders the TUI.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var view string
	switch m.mode {
	case ModeList:
		view = m.viewList()
	case ModeDetail:
		view = m.viewDetail()
	case ModeSearch:
		view = m.viewSearch()
	case ModeHelp:
		view = m.viewHelp()
	case ModeFilter:
		view = m.viewFilter()
	default:
		view = ""
	}

	// Apply toast overlay if visible
	if m.toastVisible {
		view = m.renderToast(view)
	}

	return view
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
			panel := m.renderPreviewPanel(item)
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

// renderToast renders a centered toast overlay on top of the base content.
func (m Model) renderToast(base string) string {
	if !m.toastVisible || m.toastMsg == "" {
		return base
	}

	// Toast styling - rounded border, green background hint
	toastStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("10")).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("22")).
		Padding(0, 2).
		Bold(true)

	toast := toastStyle.Render(m.toastMsg)
	toastWidth := lipgloss.Width(toast)
	toastHeight := lipgloss.Height(toast)

	// Calculate center position
	baseLines := strings.Split(base, "\n")
	centerRow := len(baseLines)/2 - toastHeight/2
	centerCol := (m.width - toastWidth) / 2

	if centerRow < 0 {
		centerRow = 0
	}
	if centerCol < 0 {
		centerCol = 0
	}

	// Overlay toast onto base
	toastLines := strings.Split(toast, "\n")
	for i, toastLine := range toastLines {
		row := centerRow + i
		if row >= len(baseLines) {
			break
		}

		baseLine := baseLines[row]
		baseLineWidth := lipgloss.Width(baseLine)

		var newLine string
		if centerCol <= baseLineWidth {
			newLine = truncateToWidth(baseLine, centerCol)
		} else {
			newLine = baseLine + strings.Repeat(" ", centerCol-baseLineWidth)
		}
		newLine += toastLine

		baseLines[row] = newLine
	}

	return strings.Join(baseLines, "\n")
}

func (m Model) viewDetail() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1)

	header := headerStyle.Render("Notification Detail")

	s := header + "\n" + m.viewport.View() + "\n" + m.buildKeybindBar(m.width, "detail")

	// Overlay image panel if we have collected images
	if m.selected != nil && len(m.detailImages) > 0 {
		imagePanel := m.renderDetailImagePanel(*m.selected)
		if imagePanel != "" {
			s = m.overlayPanel(s, imagePanel)
		}
	}

	return s
}

func (m Model) viewSearch() string {
	matchCount := len(m.list.Items())
	countStr := fmt.Sprintf("(%d matches)", matchCount)

	// Show search bar at top, then the filtered list, then keybinds
	searchBar := "Search: " + m.searchInput.View() + " " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(countStr)

	s := searchBar + "\n" + m.list.View() + "\n" + m.buildKeybindBar(m.width, "search")

	// Overlay preview panel if active
	if m.previewActive {
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			panel := m.renderPreviewPanel(item)
			s = m.overlayPanel(s, panel)
		}
	}

	return s
}

func (m Model) viewFilter() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	// Current filter indicator
	currentLabel := ""
	switch m.filterMode {
	case FilterAll:
		currentLabel = "All"
	case FilterActive:
		currentLabel = "Active"
	case FilterUndismissed:
		currentLabel = "Undismissed"
	case FilterDismissed:
		currentLabel = "Dismissed"
	}

	s := titleStyle.Render("Filter Menu") + dimStyle.Render(fmt.Sprintf(" (current: %s)", currentLabel)) + "\n\n"

	s += keyStyle.Render("  1") + "/" + keyStyle.Render("a") + "  " + descStyle.Render("All notifications") + "\n"
	s += keyStyle.Render("  2") + "/" + keyStyle.Render("v") + "  " + descStyle.Render("Active (visible popups)") + "\n"
	s += keyStyle.Render("  3") + "/" + keyStyle.Render("u") + "  " + descStyle.Render("Undismissed") + "\n"
	s += keyStyle.Render("  4") + "/" + keyStyle.Render("d") + "  " + descStyle.Render("Dismissed") + "\n"

	s += "\n" + dimStyle.Render("Press a key to select, esc to cancel")

	return s
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
	s += keyStyle.Render("  Tab/S-Tab") + "    Cycle stacked items\n"
	s += "\n"

	s += sectionStyle.Render("Actions") + "\n"
	s += keyStyle.Render("  enter") + "        View details\n"
	s += keyStyle.Render("  p") + "            Preview panel\n"
	s += keyStyle.Render("  y") + "            Copy (yank) body\n"
	s += keyStyle.Render("  Y") + "            Copy all as JSON\n"
	s += keyStyle.Render("  i") + "            Copy image\n"
	s += keyStyle.Render("  d") + "            Dismiss/undismiss\n"
	s += keyStyle.Render("  x") + "            Delete permanently\n"
	s += keyStyle.Render("  R") + "            Replay notification\n"
	s += "\n"

	s += sectionStyle.Render("Filters") + "\n"
	s += keyStyle.Render("  f") + "            Filter menu\n"
	s += keyStyle.Render("  1") + "            All notifications\n"
	s += keyStyle.Render("  2") + "            Active (visible)\n"
	s += keyStyle.Render("  3") + "            Undismissed\n"
	s += keyStyle.Render("  4") + "            Dismissed\n"
	s += keyStyle.Render("  /") + "            Search/filter\n"
	s += "\n"

	s += sectionStyle.Render("General") + "\n"
	s += keyStyle.Render("  ?") + "            This help\n"
	s += keyStyle.Render("  r") + "            Refresh\n"
	s += keyStyle.Render("  esc") + "          Back\n"
	s += keyStyle.Render("  q") + "            Quit (back in detail)\n"

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

// truncateString truncates a string to the specified visual width.
// Properly handles Unicode characters by iterating runes.
func truncateString(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	width := 0
	for i, r := range s {
		runeWidth := 1
		// East Asian wide characters take 2 columns
		if r > 0x1100 {
			runeWidth = 2
		}
		if width+runeWidth > maxWidth {
			return s[:i]
		}
		width += runeWidth
	}
	return s
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
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))     // cyan for status
	connectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green for connected
	disconnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))    // dim for disconnected

	// Build daemon status prefix with connection indicator
	var prefix string
	if m.daemonStatus != "" {
		// Show connection status: ● (connected) or ○ (disconnected)
		connIndicator := disconnStyle.Render("○")
		if m.daemon != nil && m.daemon.IsAvailable() {
			connIndicator = connectedStyle.Render("●")
		}
		prefix = connIndicator + " " + statusStyle.Render(m.daemonStatus) + "  "
	}

	// Check if current selection has an image (including database-stored images)
	var currentHasImage bool
	if mode == "list" {
		if item, ok := m.list.SelectedItem().(notificationItem); ok {
			currentHasImage = hasImageWithDB(item.notification, m.db)
		}
	} else if mode == "detail" && m.selected != nil {
		currentHasImage = hasImageWithDB(*m.selected, m.db)
	}

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
			{"f", "filter", 6},
			{"1-4", "quick filter", 7},
			{"d", "dismiss", 8},
			{"y", "copy", 9},
		}
		if currentHasImage {
			binds = append(binds, keybind{"i", "image", 10})
		}
		binds = append(binds,
			keybind{"R", "replay", 11},
			keybind{"x", "delete", 12},
			keybind{"r", "refresh", 13},
		)
	case "detail":
		binds = []keybind{
			{"q/esc", "back", 1},
			{"/", "search", 2},
			{"y", "copy body", 3},
			{"Y", "copy JSON", 4},
		}
		if currentHasImage {
			binds = append(binds, keybind{"i", "copy image", 5})
		}
		// Show image navigation if multiple images
		if len(m.detailImages) > 1 {
			binds = append(binds, keybind{"←/→", "images", 6})
		}
		binds = append(binds,
			keybind{"Tab", "stack", 7},
			keybind{"j/k", "scroll", 8},
		)
	case "search":
		binds = []keybind{
			{"enter", "view", 1},
			{"esc", "close", 2},
			{"↑/↓", "navigate", 3},
		}
	}

	// Build the bar, adding keybinds until we run out of space
	const separator = "  "
	prefixLen := len(stripANSI(prefix))
	result := ""
	for _, b := range binds {
		item := keyStyle.Render(b.key) + " " + b.desc
		plainItem := b.key + " " + b.desc
		testLen := prefixLen + len(result) + len(separator) + len(plainItem)
		if result != "" {
			testLen = prefixLen + len(stripANSI(result)) + len(separator) + len(plainItem)
		}

		if width > 0 && testLen > width {
			break
		}
		if result != "" {
			result += separator
		}
		result += item
	}

	return prefix + style.Render(result)
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
	Config  *config.Config
	DB      *db.DB
	Adapter input.InputAdapter
}

// Run starts the TUI with the given options.
func Run(opts RunOptions) error {
	database := opts.DB

	// Import from adapter on startup to ensure we have notifications
	if opts.Adapter != nil && database != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := importFromAdapter(ctx, opts.Adapter, database)
		cancel()
		if err != nil {
			// Log but continue - database might have persisted notifications
			fmt.Fprintf(os.Stderr, "Warning: failed to import notifications: %v\n", err)
		}
	}

	// Create D-Bus client for daemon communication (gracefully handles unavailable daemon)
	daemonClient := dbus.NewDaemonClient(nil)

	// Create icon resolver for NerdFont symbols (gracefully handles errors)
	iconResolver, _ := icon.NewResolverWithAliases()

	m := New(opts.Config, database, daemonClient, iconResolver)
	p := tea.NewProgram(m, tea.WithAltScreen())

	_, err := p.Run()

	// Clean up D-Bus client
	if daemonClient != nil {
		_ = daemonClient.Close()
	}

	return err
}
