package daemon

import (
	"github.com/jmylchreest/histui/internal/audio"
	"github.com/jmylchreest/histui/internal/db"
	"github.com/jmylchreest/histui/internal/dbus"
	"github.com/jmylchreest/histui/internal/display"
)

// DaemonHandler implements dbus.DaemonHandler for histuid.
// It handles D-Bus daemon control requests.
type DaemonHandler struct {
	dndManager     *db.DnDManager
	audioManager   *audio.Manager
	displayManager *display.Manager
}

// NewDaemonHandler creates a new daemon handler.
func NewDaemonHandler(
	dndManager *db.DnDManager,
	audioManager *audio.Manager,
	displayManager *display.Manager,
) *DaemonHandler {
	return &DaemonHandler{
		dndManager:     dndManager,
		audioManager:   audioManager,
		displayManager: displayManager,
	}
}

// SetDnD sets the Do Not Disturb state.
func (h *DaemonHandler) SetDnD(enabled bool) error {
	if h.dndManager == nil {
		return nil
	}
	return h.dndManager.SetEnabled(enabled)
}

// GetDnD returns the current DnD state.
func (h *DaemonHandler) GetDnD() bool {
	if h.dndManager == nil {
		return false
	}
	return h.dndManager.Enabled()
}

// StopAudio stops any currently playing notification sound.
func (h *DaemonHandler) StopAudio() {
	if h.audioManager != nil {
		h.audioManager.StopPlayback()
	}
}

// CloseNotification closes a notification popup by histui ID. Returns true if closed.
func (h *DaemonHandler) CloseNotification(histuiID string) bool {
	if h.displayManager == nil {
		return false
	}
	return h.displayManager.CloseByHistuiID(histuiID, dbus.CloseReasonDismissed)
}

// CloseAllNotifications closes all active notification popups. Returns count closed.
func (h *DaemonHandler) CloseAllNotifications() int {
	if h.displayManager == nil {
		return 0
	}
	return h.displayManager.CloseAll(dbus.CloseReasonDismissed)
}

// GetActiveNotifications returns histui IDs of all active/queued notification popups.
func (h *DaemonHandler) GetActiveNotifications() []string {
	if h.displayManager == nil {
		return []string{}
	}
	return h.displayManager.GetActiveHistuiIDs()
}

// IsNotificationActive checks if a notification has an active or queued popup.
func (h *DaemonHandler) IsNotificationActive(histuiID string) bool {
	if h.displayManager == nil {
		return false
	}
	return h.displayManager.IsNotificationActive(histuiID)
}
