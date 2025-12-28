package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// Handler defines the interface for handling IPC commands.
type Handler interface {
	// SetDnD sets the Do Not Disturb state.
	SetDnD(enabled bool) error
	// GetDnD returns the current DnD state.
	GetDnD() bool
	// StopAudio stops any currently playing notification sound.
	StopAudio()
	// ClosePopup closes a popup by histui ID. Returns true if closed.
	ClosePopup(histuiID string) bool
	// CloseAllPopups closes all active popups. Returns count closed.
	CloseAllPopups() int
}

// Server provides the IPC server for histuid.
type Server struct {
	socketPath string
	listener   net.Listener
	handler    Handler
	logger     *slog.Logger

	mu      sync.Mutex
	running bool
	wg      sync.WaitGroup
}

// NewServer creates a new IPC server.
func NewServer(handler Handler, logger *slog.Logger) (*Server, error) {
	sockPath, err := SocketPath()
	if err != nil {
		return nil, err
	}

	return &Server{
		socketPath: sockPath,
		handler:    handler,
		logger:     logger,
	}, nil
}

// Start starts the IPC server.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}

	// Ensure socket directory exists
	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0700); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("create socket dir: %w", err)
	}

	// Remove any stale socket
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		s.mu.Unlock()
		return fmt.Errorf("remove stale socket: %w", err)
	}

	// Create listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = listener
	s.running = true
	s.mu.Unlock()

	// Set socket permissions
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		s.logger.Warn("failed to set socket permissions", "error", err)
	}

	s.logger.Info("IPC server started", "socket", s.socketPath)

	// Accept connections
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.acceptLoop()
	}()

	return nil
}

// Stop stops the IPC server.
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	// Close listener to unblock accept
	if s.listener != nil {
		_ = s.listener.Close()
	}

	// Wait for goroutines
	s.wg.Wait()

	// Remove socket file
	_ = os.Remove(s.socketPath)

	s.logger.Info("IPC server stopped")
	return nil
}

// acceptLoop accepts incoming connections.
func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			running := s.running
			s.mu.Unlock()
			if !running {
				return // Server stopped
			}
			s.logger.Warn("accept error", "error", err)
			continue
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConnection(conn)
		}()
	}
}

// handleConnection handles a single client connection.
func (s *Server) handleConnection(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var req Request
	if err := decoder.Decode(&req); err != nil {
		s.logger.Debug("failed to decode request", "error", err)
		return
	}

	resp := s.handleRequest(&req)

	if err := encoder.Encode(resp); err != nil {
		s.logger.Debug("failed to encode response", "error", err)
	}
}

// handleRequest processes a request and returns a response.
func (s *Server) handleRequest(req *Request) *Response {
	switch req.Command {
	case CmdPing:
		return &Response{Success: true}

	case CmdSetDnD:
		if err := s.handler.SetDnD(req.DnDEnabled); err != nil {
			return &Response{Success: false, Error: err.Error()}
		}
		return &Response{Success: true, DnDEnabled: req.DnDEnabled}

	case CmdGetDnD:
		enabled := s.handler.GetDnD()
		return &Response{Success: true, DnDEnabled: enabled}

	case CmdStopAudio:
		s.handler.StopAudio()
		return &Response{Success: true}

	case CmdClosePopup:
		closed := s.handler.ClosePopup(req.HistuiID)
		count := 0
		if closed {
			count = 1
		}
		return &Response{Success: true, Closed: count}

	case CmdCloseAllPopups:
		count := s.handler.CloseAllPopups()
		return &Response{Success: true, Closed: count}

	default:
		return &Response{Success: false, Error: fmt.Sprintf("unknown command: %s", req.Command)}
	}
}

// SocketPath returns the socket path for this server.
func (s *Server) SocketPath() string {
	return s.socketPath
}
