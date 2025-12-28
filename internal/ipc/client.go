package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Client provides IPC communication with the histuid daemon.
// Operations gracefully fall back when daemon is not running.
type Client struct {
	socketPath string
	timeout    time.Duration
}

// NewClient creates a new IPC client.
func NewClient() (*Client, error) {
	sockPath, err := SocketPath()
	if err != nil {
		return nil, err
	}

	return &Client{
		socketPath: sockPath,
		timeout:    2 * time.Second,
	}, nil
}

// IsRunning checks if the daemon is running and accepting connections.
func (c *Client) IsRunning() bool {
	conn, err := c.connect()
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Ping checks if the daemon is alive.
func (c *Client) Ping() error {
	resp, err := c.send(Request{Command: CmdPing})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("ping failed: %s", resp.Error)
	}
	return nil
}

// SetDnD sets the Do Not Disturb state.
// No-op if daemon is not running (DnD only matters when daemon is active).
func (c *Client) SetDnD(enabled bool) error {
	resp, err := c.send(Request{
		Command:    CmdSetDnD,
		DnDEnabled: enabled,
	})
	if err != nil {
		// Daemon not running - no-op
		return nil
	}
	if !resp.Success {
		return fmt.Errorf("set dnd failed: %s", resp.Error)
	}
	return nil
}

// GetDnD gets the current DnD state.
// Returns false if daemon is not running.
func (c *Client) GetDnD() (bool, error) {
	resp, err := c.send(Request{Command: CmdGetDnD})
	if err != nil {
		// Daemon not running - DnD is effectively disabled
		return false, nil
	}
	if !resp.Success {
		return false, fmt.Errorf("get dnd failed: %s", resp.Error)
	}
	return resp.DnDEnabled, nil
}

// ToggleDnD toggles the DnD state and returns the new state.
func (c *Client) ToggleDnD() (bool, error) {
	current, err := c.GetDnD()
	if err != nil {
		return false, err
	}
	newState := !current
	if err := c.SetDnD(newState); err != nil {
		return false, err
	}
	return newState, nil
}

// StopAudio stops any currently playing notification sound.
// No-op if daemon is not running.
func (c *Client) StopAudio() error {
	resp, err := c.send(Request{Command: CmdStopAudio})
	if err != nil {
		// Daemon not running - nothing to stop
		return nil
	}
	if !resp.Success {
		return fmt.Errorf("stop audio failed: %s", resp.Error)
	}
	return nil
}

// ClosePopup closes a specific popup by histui ID.
// No-op if daemon is not running.
func (c *Client) ClosePopup(histuiID string) error {
	resp, err := c.send(Request{
		Command:  CmdClosePopup,
		HistuiID: histuiID,
	})
	if err != nil {
		// Daemon not running - no popup to close
		return nil
	}
	if !resp.Success {
		return fmt.Errorf("close popup failed: %s", resp.Error)
	}
	return nil
}

// ClosePopups closes multiple popups by histui IDs.
// No-op if daemon is not running.
func (c *Client) ClosePopups(histuiIDs []string) error {
	for _, id := range histuiIDs {
		if err := c.ClosePopup(id); err != nil {
			return err
		}
	}
	return nil
}

// CloseAllPopups closes all active popups.
// No-op if daemon is not running. Returns number closed.
func (c *Client) CloseAllPopups() (int, error) {
	resp, err := c.send(Request{Command: CmdCloseAllPopups})
	if err != nil {
		// Daemon not running - no popups to close
		return 0, nil
	}
	if !resp.Success {
		return 0, fmt.Errorf("close all popups failed: %s", resp.Error)
	}
	return resp.Closed, nil
}

// connect establishes a connection to the daemon socket.
func (c *Client) connect() (net.Conn, error) {
	return net.DialTimeout("unix", c.socketPath, c.timeout)
}

// send sends a request and receives a response.
func (c *Client) send(req Request) (*Response, error) {
	conn, err := c.connect()
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	// Set deadline for the entire operation
	if err := conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}

	// Send request
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	// Read response
	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &resp, nil
}

// EnsureSocketDir ensures the socket directory exists.
func EnsureSocketDir() error {
	sockPath, err := SocketPath()
	if err != nil {
		return err
	}
	return os.MkdirAll(filepath.Dir(sockPath), 0700)
}
