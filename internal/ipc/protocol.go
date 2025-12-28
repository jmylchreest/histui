// Package ipc provides inter-process communication between histui CLI/TUI and histuid daemon.
package ipc

import (
	"os"
	"path/filepath"
)

// Command types for IPC messages.
const (
	// CmdSetDnD sets the Do Not Disturb state.
	CmdSetDnD = "set_dnd"
	// CmdGetDnD gets the current DnD state.
	CmdGetDnD = "get_dnd"
	// CmdStopAudio stops any currently playing notification sound.
	CmdStopAudio = "stop_audio"
	// CmdClosePopup closes a specific popup by histui ID.
	CmdClosePopup = "close_popup"
	// CmdCloseAllPopups closes all active popups.
	CmdCloseAllPopups = "close_all_popups"
	// CmdPing checks if the daemon is alive.
	CmdPing = "ping"
)

// Request is the IPC request message format.
type Request struct {
	Command string `json:"command"`
	// Command-specific data
	DnDEnabled bool     `json:"dnd_enabled,omitempty"` // For CmdSetDnD
	HistuiID   string   `json:"histui_id,omitempty"`   // For CmdClosePopup
	HistuiIDs  []string `json:"histui_ids,omitempty"`  // For batch operations
}

// Response is the IPC response message format.
type Response struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	// Response data
	DnDEnabled bool `json:"dnd_enabled,omitempty"` // For CmdGetDnD
	Closed     int  `json:"closed,omitempty"`      // Number of popups closed
}

// SocketPath returns the path to the IPC socket.
func SocketPath() (string, error) {
	// Use XDG_RUNTIME_DIR if available (preferred for sockets)
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "histui", "histuid.sock"), nil
	}

	// Fall back to data directory
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataHome = filepath.Join(homeDir, ".local", "share")
	}
	return filepath.Join(dataHome, "histui", "histuid.sock"), nil
}
