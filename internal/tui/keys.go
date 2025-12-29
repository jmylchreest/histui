package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines the key bindings for the TUI.
type KeyMap struct {
	// Navigation
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding

	// Stack navigation
	NextStack key.Binding
	PrevStack key.Binding

	// Actions
	Enter     key.Binding
	Back      key.Binding
	Yank      key.Binding // Copy body
	YankAll   key.Binding // Copy all as JSON
	YankImage key.Binding // Copy image to clipboard
	Dismiss   key.Binding
	Delete    key.Binding
	Search    key.Binding
	Refresh   key.Binding
	Filter    key.Binding // Open filter submenu
	Preview   key.Binding
	Replay    key.Binding

	// Quick filters (1-4)
	FilterAll         key.Binding
	FilterActive      key.Binding
	FilterUndismissed key.Binding
	FilterDismissed   key.Binding

	// Global
	Quit key.Binding
	Help key.Binding
}

// ShortHelp returns a short help message.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns a full help message.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown},
		{k.Enter, k.Back, k.Yank, k.YankAll},
		{k.Search, k.Filter, k.Refresh, k.Dismiss, k.Delete},
		{k.FilterAll, k.FilterActive, k.FilterUndismissed, k.FilterDismissed},
		{k.Preview, k.Replay, k.Help, k.Quit},
	}
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to bottom"),
		),
		NextStack: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next in stack"),
		),
		PrevStack: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("S-tab", "prev in stack"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "view details"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back"),
		),
		Yank: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy body"),
		),
		YankAll: key.NewBinding(
			key.WithKeys("Y"),
			key.WithHelp("Y", "copy all as JSON"),
		),
		YankImage: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "copy image"),
		),
		Dismiss: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "dismiss/restore"),
		),
		Delete: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "delete permanently"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter menu"),
		),
		Preview: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "preview"),
		),
		Replay: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "replay"),
		),
		FilterAll: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "all"),
		),
		FilterActive: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "active"),
		),
		FilterUndismissed: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "undismissed"),
		),
		FilterDismissed: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "dismissed"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit/back"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}
