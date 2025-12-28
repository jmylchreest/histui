package daemon

import (
	"github.com/jmylchreest/histui/internal/audio"
	"github.com/jmylchreest/histui/internal/db"
	"github.com/jmylchreest/histui/internal/dbus"
	"github.com/jmylchreest/histui/internal/display"
)

// IPCHandler implements ipc.Handler for histuid.
type IPCHandler struct {
	dndManager     *db.DnDManager
	audioManager   *audio.Manager
	displayManager *display.Manager
}

// NewIPCHandler creates a new IPC handler.
func NewIPCHandler(
	dndManager *db.DnDManager,
	audioManager *audio.Manager,
	displayManager *display.Manager,
) *IPCHandler {
	return &IPCHandler{
		dndManager:     dndManager,
		audioManager:   audioManager,
		displayManager: displayManager,
	}
}

// SetDnD sets the Do Not Disturb state.
func (h *IPCHandler) SetDnD(enabled bool) error {
	if h.dndManager == nil {
		return nil
	}
	return h.dndManager.SetEnabled(enabled)
}

// GetDnD returns the current DnD state.
func (h *IPCHandler) GetDnD() bool {
	if h.dndManager == nil {
		return false
	}
	return h.dndManager.Enabled()
}

// StopAudio stops any currently playing notification sound.
func (h *IPCHandler) StopAudio() {
	if h.audioManager != nil {
		h.audioManager.StopPlayback()
	}
}

// ClosePopup closes a popup by histui ID. Returns true if closed.
func (h *IPCHandler) ClosePopup(histuiID string) bool {
	if h.displayManager == nil {
		return false
	}
	return h.displayManager.CloseByHistuiID(histuiID, dbus.CloseReasonDismissed)
}

// CloseAllPopups closes all active popups. Returns count closed.
func (h *IPCHandler) CloseAllPopups() int {
	if h.displayManager == nil {
		return 0
	}
	return h.displayManager.CloseAll(dbus.CloseReasonDismissed)
}
