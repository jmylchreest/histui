package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func waitForInvalidation(t *testing.T, w *fileWatcher) {
	t.Helper()
	select {
	case <-w.changes:
	case <-time.After(3 * time.Second):
		t.Fatal("file change was not delivered")
	}
}

func TestFileWatcherKeepsWatchingBetweenCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".lastupdated")
	w, err := newFileWatcher(path)
	require.NoError(t, err)
	defer w.close()
	require.NoError(t, os.WriteFile(path, []byte("first"), 0600))
	waitForInvalidation(t, w)
	// No next wait command has been started. The filesystem watch must still
	// be active and retain invalidation while the TUI is processing its read.
	require.NoError(t, os.WriteFile(path, []byte("second"), 0600))
	waitForInvalidation(t, w)
	// Atomic replacement must be observed through the parent-directory watch.
	tmp := filepath.Join(filepath.Dir(path), "replacement")
	require.NoError(t, os.WriteFile(tmp, []byte("third"), 0600))
	require.NoError(t, os.Rename(tmp, path))
	waitForInvalidation(t, w)
}

func TestFileWatcherCoalescesPendingEvents(t *testing.T) {
	w := &fileWatcher{changes: make(chan struct{}, 1)}
	for range 1000 {
		w.invalidate()
	}
	require.Len(t, w.changes, 1)
}

func TestFileWatcherCloseUnblocksWait(t *testing.T) {
	w, err := newFileWatcher(filepath.Join(t.TempDir(), ".lastupdated"))
	require.NoError(t, err)
	returned := make(chan struct{})
	go func() { w.wait(); close(returned) }()
	w.close()
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		t.Fatal("wait did not stop on close")
	}
}
