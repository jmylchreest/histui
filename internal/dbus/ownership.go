// Package dbus provides D-Bus interfaces for histui.
package dbus

import (
	"fmt"
	"os"
	"strings"

	"github.com/godbus/dbus/v5"
)

// DaemonOwnerInfo contains information about the notification daemon owner.
type DaemonOwnerInfo struct {
	ProcessName string // e.g., "histuid", "dunst", "mako", "swaync"
	PID         uint32
	IsHistuid   bool // true if histuid owns the notification bus name
}

// GetNotificationDaemonOwner returns information about which process
// owns the org.freedesktop.Notifications bus name.
// Returns nil if no daemon is running or D-Bus is unavailable.
func GetNotificationDaemonOwner() (*DaemonOwnerInfo, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect to D-Bus: %w", err)
	}
	defer func() { _ = conn.Close() }()

	return GetNotificationDaemonOwnerWithConn(conn)
}

// GetNotificationDaemonOwnerWithConn returns information about which process
// owns the org.freedesktop.Notifications bus name using an existing connection.
func GetNotificationDaemonOwnerWithConn(conn *dbus.Conn) (*DaemonOwnerInfo, error) {
	if conn == nil {
		return nil, fmt.Errorf("nil D-Bus connection")
	}

	// Get the unique connection name of the owner
	var owner string
	err := conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, DBusBusName).Store(&owner)
	if err != nil {
		// No owner means no notification daemon is running
		return nil, nil
	}

	// Get the PID of the connection
	var pid uint32
	err = conn.BusObject().Call("org.freedesktop.DBus.GetConnectionUnixProcessID", 0, owner).Store(&pid)
	if err != nil {
		return nil, fmt.Errorf("get owner PID: %w", err)
	}

	// Get the process name from /proc
	procName := getProcessNameByPID(pid)

	info := &DaemonOwnerInfo{
		ProcessName: procName,
		PID:         pid,
		IsHistuid:   procName == "histuid",
	}

	return info, nil
}

// getProcessNameByPID reads the process name from /proc/<pid>/comm.
func getProcessNameByPID(pid uint32) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// GetDaemonStatusLabel returns a status label for the TUI status bar.
// Returns strings like "[histuid]", "[dunst] STALE", "[--]", etc.
func GetDaemonStatusLabel() string {
	info, err := GetNotificationDaemonOwner()
	if err != nil {
		return "[offline]"
	}
	if info == nil {
		return "[--] STALE"
	}

	if info.IsHistuid {
		return "[histuid]"
	}

	// Another daemon is running - history is stale
	name := info.ProcessName
	if name == "" {
		name = fmt.Sprintf("pid:%d", info.PID)
	}
	return fmt.Sprintf("[%s] STALE", name)
}
