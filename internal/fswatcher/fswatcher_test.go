package fswatcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	handler := func(string, fsnotify.Op) {}
	w := New("test", nil, handler)
	assert.NotNil(t, w)
	assert.Equal(t, "test", w.name)
	assert.NotNil(t, w.watchedPaths)
	assert.NotNil(t, w.watchedDirs)
	w.Stop()
}

func TestNew_WithOptions(t *testing.T) {
	var gotErr error
	errHandler := func(err error) { gotErr = err }

	w := New("test", nil, nil,
		WithEnsureDirs(true),
		WithEventFilter(fsnotify.Write),
		WithErrorHandler(errHandler),
	)
	assert.True(t, w.ensureDirs)
	assert.Equal(t, fsnotify.Write, w.eventFilter)

	// Verify the error handler was set by invoking it
	w.errorHandler(assert.AnError)
	assert.Equal(t, assert.AnError, gotErr)

	w.Stop()
}

func TestWatch_And_Unwatch(t *testing.T) {
	handler := func(string, fsnotify.Op) {}
	w := New("test", nil, handler)
	defer w.Stop()

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	require.NoError(t, os.WriteFile(file1, []byte("hello"), 0644))

	// Watch a file
	w.Watch(file1)

	w.mu.RLock()
	assert.True(t, w.watchedPaths[file1])
	assert.Equal(t, 1, w.watchedDirs[tmpDir])
	w.mu.RUnlock()

	// Unwatch it
	w.Unwatch(file1)

	w.mu.RLock()
	assert.False(t, w.watchedPaths[file1])
	_, hasDirWatch := w.watchedDirs[tmpDir]
	w.mu.RUnlock()
	assert.False(t, hasDirWatch)
}

func TestWatch_DirectoryRefcounting(t *testing.T) {
	handler := func(string, fsnotify.Op) {}
	w := New("test", nil, handler)
	defer w.Stop()

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	require.NoError(t, os.WriteFile(file1, []byte("a"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("b"), 0644))

	w.Watch(file1)
	w.Watch(file2)

	w.mu.RLock()
	assert.Equal(t, 2, w.watchedDirs[tmpDir])
	w.mu.RUnlock()

	// Remove one file — dir should still be watched
	w.Unwatch(file1)

	w.mu.RLock()
	assert.Equal(t, 1, w.watchedDirs[tmpDir])
	w.mu.RUnlock()

	// Remove second file — dir watch should be removed
	w.Unwatch(file2)

	w.mu.RLock()
	_, hasDirWatch := w.watchedDirs[tmpDir]
	w.mu.RUnlock()
	assert.False(t, hasDirWatch)
}

func TestWatch_EmptyPath(t *testing.T) {
	handler := func(string, fsnotify.Op) {}
	w := New("test", nil, handler)
	defer w.Stop()

	// Should not panic
	w.Watch("")
	w.mu.RLock()
	assert.Empty(t, w.watchedPaths)
	w.mu.RUnlock()
}

func TestWatch_Duplicate(t *testing.T) {
	handler := func(string, fsnotify.Op) {}
	w := New("test", nil, handler)
	defer w.Stop()

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "file.txt")
	require.NoError(t, os.WriteFile(file, []byte("a"), 0644))

	w.Watch(file)
	w.Watch(file) // duplicate

	w.mu.RLock()
	assert.Equal(t, 1, w.watchedDirs[tmpDir])
	w.mu.RUnlock()
}

func TestStart_Idempotent(t *testing.T) {
	handler := func(string, fsnotify.Op) {}
	w := New("test", nil, handler)
	defer w.Stop()

	ctx := context.Background()
	require.NoError(t, w.Start(ctx))
	require.NoError(t, w.Start(ctx)) // second call should be no-op
}

func TestStop_NotRunning(t *testing.T) {
	handler := func(string, fsnotify.Op) {}
	w := New("test", nil, handler)

	// Should not panic or block
	w.Stop()
}

func TestStart_ContextCancellation(t *testing.T) {
	handler := func(string, fsnotify.Op) {}
	w := New("test", nil, handler)

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, w.Start(ctx))

	cancel()

	// Event loop should exit; give it a moment
	time.Sleep(50 * time.Millisecond)
	// Stop should not block since event loop already exited
	w.Stop()
}

func TestWithEnsureDirs(t *testing.T) {
	handler := func(string, fsnotify.Op) {}
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "subdir", "nested")
	file := filepath.Join(newDir, "config.toml")

	w := New("test", nil, handler, WithEnsureDirs(true))
	defer w.Stop()

	// Directory doesn't exist yet
	_, err := os.Stat(newDir)
	assert.True(t, os.IsNotExist(err))

	// Watch should create it
	w.Watch(file)

	_, err = os.Stat(newDir)
	assert.NoError(t, err)
}

func TestFileChange_HandlerCalled(t *testing.T) {
	var mu sync.Mutex
	var gotPath string
	var gotOp fsnotify.Op

	handler := func(absPath string, op fsnotify.Op) {
		mu.Lock()
		gotPath = absPath
		gotOp = op
		mu.Unlock()
	}

	w := New("test", nil, handler)

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "watched.txt")
	require.NoError(t, os.WriteFile(file, []byte("initial"), 0644))

	w.Watch(file)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, w.Start(ctx))

	// Modify the file
	time.Sleep(50 * time.Millisecond) // let watcher settle
	require.NoError(t, os.WriteFile(file, []byte("modified"), 0644))

	// Wait for the event
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return gotPath != ""
	}, 2*time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, file, gotPath)
	assert.True(t, gotOp&(fsnotify.Write|fsnotify.Create) != 0)
	mu.Unlock()

	w.Stop()
}

func TestUnwatchedFile_HandlerNotCalled(t *testing.T) {
	called := false
	handler := func(string, fsnotify.Op) {
		called = true
	}

	w := New("test", nil, handler)

	tmpDir := t.TempDir()
	watched := filepath.Join(tmpDir, "watched.txt")
	unwatched := filepath.Join(tmpDir, "unwatched.txt")
	require.NoError(t, os.WriteFile(watched, []byte("a"), 0644))
	require.NoError(t, os.WriteFile(unwatched, []byte("b"), 0644))

	w.Watch(watched) // only watch one file
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, w.Start(ctx))

	time.Sleep(50 * time.Millisecond)
	// Modify the unwatched file (same directory)
	require.NoError(t, os.WriteFile(unwatched, []byte("changed"), 0644))

	time.Sleep(200 * time.Millisecond)
	assert.False(t, called, "handler should not be called for unwatched file")

	w.Stop()
}
