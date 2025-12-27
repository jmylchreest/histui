package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DataDir returns the path to the histui data directory.
// Uses XDG_DATA_HOME or defaults to ~/.local/share/histui.
func DataDir() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "histui"), nil
}

// HistoryPath returns the path to the notification history file.
func HistoryPath() (string, error) {
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "history.jsonl"), nil
}

// DnDTrigger represents what triggered the DnD state change.
type DnDTrigger string

const (
	// DnDTriggerUser indicates a user-initiated DnD change (CLI, TUI, etc.)
	DnDTriggerUser DnDTrigger = "user"
	// DnDTriggerRule indicates a DnD rule triggered the change
	DnDTriggerRule DnDTrigger = "rule"
	// DnDTriggerSchedule indicates a scheduled DnD change
	DnDTriggerSchedule DnDTrigger = "schedule"
	// DnDTriggerSystem indicates a system event triggered the change (e.g., fullscreen app)
	DnDTriggerSystem DnDTrigger = "system"
)

// DnDTransition records details about a DnD state change.
type DnDTransition struct {
	Trigger   DnDTrigger `json:"trigger"`             // What type of event triggered the change
	Reason    string     `json:"reason"`              // Human-readable reason (e.g., "dnd on", "rule: meeting-hours")
	Source    string     `json:"source,omitempty"`    // Source identifier (e.g., "cli", "waybar", "histuid")
	Timestamp int64      `json:"timestamp"`           // When the transition occurred
	RuleName  string     `json:"rule_name,omitempty"` // Name of the rule if trigger is DnDTriggerRule
}

// SharedState contains state that is shared between histui and histuid.
// This is persisted to ~/.local/share/histui/state.json
type SharedState struct {
	// Do Not Disturb
	DnDEnabled   bool   `json:"dnd_enabled"`
	DnDEnabledAt int64  `json:"dnd_enabled_at,omitempty"` // Unix timestamp (legacy, kept for compatibility)
	DnDEnabledBy string `json:"dnd_enabled_by,omitempty"` // Legacy field, kept for compatibility

	// Enhanced DnD tracking
	DnDLastTransition *DnDTransition `json:"dnd_last_transition,omitempty"` // Details of the last DnD state change

	// Statistics (optional, for waybar)
	LastNotificationAt int64 `json:"last_notification_at,omitempty"`

	// Version for compatibility
	SchemaVersion int `json:"schema_version"` // Currently 2
}

const (
	// CurrentSchemaVersion is the current version of the state schema.
	CurrentSchemaVersion = 2
)

// stateFileMutex protects concurrent access to the state file.
var stateFileMutex sync.RWMutex

// DefaultSharedState returns a new SharedState with default values.
func DefaultSharedState() *SharedState {
	return &SharedState{
		DnDEnabled:    false,
		SchemaVersion: CurrentSchemaVersion,
	}
}

// StateFilePath returns the path to the state file.
func StateFilePath() (string, error) {
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "state.json"), nil
}

// LoadSharedState loads the shared state from disk.
// If the file doesn't exist, returns a default state.
func LoadSharedState() (*SharedState, error) {
	stateFileMutex.RLock()
	defer stateFileMutex.RUnlock()

	path, err := StateFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSharedState(), nil
		}
		return nil, err
	}

	var state SharedState
	if err := json.Unmarshal(data, &state); err != nil {
		// If the file is corrupted, return default state
		return DefaultSharedState(), nil
	}

	// Ensure schema version is set
	if state.SchemaVersion == 0 {
		state.SchemaVersion = CurrentSchemaVersion
	}

	return &state, nil
}

// SaveSharedState saves the shared state to disk.
func SaveSharedState(state *SharedState) error {
	stateFileMutex.Lock()
	defer stateFileMutex.Unlock()

	path, err := StateFilePath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Ensure schema version is set
	if state.SchemaVersion == 0 {
		state.SchemaVersion = CurrentSchemaVersion
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	// Write atomically via temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// SetDnD updates the Do Not Disturb state with full transition tracking.
// Parameters:
//   - enabled: whether DnD should be enabled
//   - trigger: what type of event triggered this change (user, rule, schedule, system)
//   - reason: human-readable reason (e.g., "dnd on", "meeting-hours rule")
//   - source: source identifier (e.g., "cli", "tui", "waybar", "histuid")
//   - ruleName: name of the rule if trigger is DnDTriggerRule, empty otherwise
func (s *SharedState) SetDnD(enabled bool, trigger DnDTrigger, reason, source, ruleName string) {
	s.DnDEnabled = enabled
	now := time.Now().Unix()

	// Update compatibility fields
	if enabled {
		s.DnDEnabledAt = now
		s.DnDEnabledBy = source
	} else {
		s.DnDEnabledAt = 0
		s.DnDEnabledBy = ""
	}

	// Update transition tracking
	s.DnDLastTransition = &DnDTransition{
		Trigger:   trigger,
		Reason:    reason,
		Source:    source,
		Timestamp: now,
		RuleName:  ruleName,
	}
}

// ToggleDnD toggles the Do Not Disturb state with full transition tracking.
// Parameters:
//   - trigger: what type of event triggered this change
//   - reason: human-readable reason
//   - source: source identifier
//   - ruleName: rule name if applicable
//
// Returns the new DnD state (true = enabled).
func (s *SharedState) ToggleDnD(trigger DnDTrigger, reason, source, ruleName string) bool {
	s.SetDnD(!s.DnDEnabled, trigger, reason, source, ruleName)
	return s.DnDEnabled
}

// UpdateLastNotification updates the last notification timestamp.
func (s *SharedState) UpdateLastNotification() {
	s.LastNotificationAt = time.Now().Unix()
}
