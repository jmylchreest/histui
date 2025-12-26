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

	// Actions
	Enter           key.Binding
	Back            key.Binding
	Copy            key.Binding
	CopySummary     key.Binding
	CopyAllJSON     key.Binding
	CopyAllYAML     key.Binding
	Dismiss         key.Binding
	HardDelete      key.Binding
	Search          key.Binding
	Refresh         key.Binding
	ToggleDismissed key.Binding

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
		{k.Enter, k.Back, k.Copy, k.CopySummary},
		{k.Search, k.Refresh, k.Dismiss, k.HardDelete},
		{k.ToggleDismissed, k.Help, k.Quit},
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
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "view details"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back"),
		),
		Copy: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "copy body"),
		),
		CopySummary: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "copy summary"),
		),
		CopyAllJSON: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "copy all as JSON"),
		),
		CopyAllYAML: key.NewBinding(
			key.WithKeys("alt+c"),
			key.WithHelp("alt+c", "copy all as YAML"),
		),
		Dismiss: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "dismiss"),
		),
		HardDelete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete permanently"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		ToggleDismissed: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "toggle dismissed"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}
