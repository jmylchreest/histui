package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmylchreest/histui/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJSONLPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	p, err := NewJSONLPersistence(path)
	require.NoError(t, err)
	defer p.Close()

	// File should exist
	_, err = os.Stat(path)
	require.NoError(t, err)

	// File should have header
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "histui_schema_version")
}

func TestNewJSONLPersistence_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "test.jsonl")

	p, err := NewJSONLPersistence(path)
	require.NoError(t, err)
	defer p.Close()

	// Directory should exist
	_, err = os.Stat(filepath.Dir(path))
	require.NoError(t, err)
}

func TestJSONLPersistence_AppendAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	p, err := NewJSONLPersistence(path)
	require.NoError(t, err)

	// Append notifications
	n1 := persistTestNotification("persist1")
	n2 := persistTestNotification("persist2")

	err = p.Append(n1)
	require.NoError(t, err)

	err = p.Append(n2)
	require.NoError(t, err)

	// Load and verify
	notifications, err := p.Load()
	require.NoError(t, err)
	assert.Len(t, notifications, 2)
	assert.Equal(t, "persist1", notifications[0].HistuiID)
	assert.Equal(t, "persist2", notifications[1].HistuiID)

	p.Close()
}

func TestJSONLPersistence_AppendBatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	p, err := NewJSONLPersistence(path)
	require.NoError(t, err)

	ns := []model.Notification{
		persistTestNotification("batch1"),
		persistTestNotification("batch2"),
		persistTestNotification("batch3"),
	}

	err = p.AppendBatch(ns)
	require.NoError(t, err)

	notifications, err := p.Load()
	require.NoError(t, err)
	assert.Len(t, notifications, 3)

	p.Close()
}

func TestJSONLPersistence_Rewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	p, err := NewJSONLPersistence(path)
	require.NoError(t, err)

	// Add initial notifications
	p.Append(persistTestNotification("old1"))
	p.Append(persistTestNotification("old2"))
	p.Append(persistTestNotification("old3"))

	// Rewrite with new set
	newNs := []model.Notification{
		persistTestNotification("new1"),
		persistTestNotification("new2"),
	}

	err = p.Rewrite(newNs)
	require.NoError(t, err)

	// Verify
	notifications, err := p.Load()
	require.NoError(t, err)
	assert.Len(t, notifications, 2)
	assert.Equal(t, "new1", notifications[0].HistuiID)
	assert.Equal(t, "new2", notifications[1].HistuiID)

	// Backup should be removed
	_, err = os.Stat(path + ".bak")
	assert.True(t, os.IsNotExist(err))

	p.Close()
}

func TestJSONLPersistence_Clear(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	p, err := NewJSONLPersistence(path)
	require.NoError(t, err)

	p.Append(persistTestNotification("clear1"))
	p.Append(persistTestNotification("clear2"))

	err = p.Clear()
	require.NoError(t, err)

	notifications, err := p.Load()
	require.NoError(t, err)
	assert.Len(t, notifications, 0)

	// File should still have header
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "histui_schema_version")

	p.Close()
}

func TestJSONLPersistence_LoadWithReopenedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Create and write
	p1, err := NewJSONLPersistence(path)
	require.NoError(t, err)
	p1.Append(persistTestNotification("reopen1"))
	p1.Append(persistTestNotification("reopen2"))
	p1.Close()

	// Reopen and load
	p2, err := NewJSONLPersistence(path)
	require.NoError(t, err)
	defer p2.Close()

	notifications, err := p2.Load()
	require.NoError(t, err)
	assert.Len(t, notifications, 2)
}

func TestJSONLPersistence_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	p, err := NewJSONLPersistence(path)
	require.NoError(t, err)
	p.Close()

	info, err := os.Stat(path)
	require.NoError(t, err)

	// Should be 0600
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestJSONLPersistence_SkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Write file with malformed lines
	content := `{"histui_schema_version":1,"created_at":1703577600}
{"histui_id":"valid1","histui_source":"test","app_name":"test","summary":"Test","timestamp":1703577600,"urgency":1,"urgency_name":"normal"}
{invalid json}
{"histui_id":"valid2","histui_source":"test","app_name":"test","summary":"Test","timestamp":1703577601,"urgency":1,"urgency_name":"normal"}
`
	err := os.WriteFile(path, []byte(content), 0600)
	require.NoError(t, err)

	p, err := NewJSONLPersistence(path)
	require.NoError(t, err)
	defer p.Close()

	notifications, err := p.Load()
	require.NoError(t, err)
	assert.Len(t, notifications, 2)
}

func TestJSONLPersistence_SchemaVersionCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Write file with future schema version
	content := `{"histui_schema_version":999,"created_at":1703577600}
{"histui_id":"test1","histui_source":"test","app_name":"test","summary":"Test","timestamp":1703577600,"urgency":1,"urgency_name":"normal"}
`
	err := os.WriteFile(path, []byte(content), 0600)
	require.NoError(t, err)

	p, err := NewJSONLPersistence(path)
	require.NoError(t, err)
	defer p.Close()

	_, err = p.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported schema version")
}

func TestStoreWithPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Create store with persistence
	p, err := NewJSONLPersistence(path)
	require.NoError(t, err)

	s := NewStore(p)

	// Add notifications
	s.Add(persistTestNotification("persist1"))
	s.Add(persistTestNotification("persist2"))

	s.Close()

	// Create new store and hydrate
	p2, err := NewJSONLPersistence(path)
	require.NoError(t, err)

	s2 := NewStore(p2)
	err = s2.Hydrate()
	require.NoError(t, err)

	assert.Equal(t, 2, s2.Count())

	s2.Close()
}

func TestRecoverFromCorruption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Write file with corruption
	content := `{"histui_schema_version":1,"created_at":1703577600}
{"histui_id":"valid1","histui_source":"test","app_name":"test","summary":"Test","timestamp":1703577600,"urgency":1,"urgency_name":"normal","histui_imported_at":1703577600}
corrupt line that will break things
{"histui_id":"valid2","histui_source":"test","app_name":"test","summary":"Test","timestamp":1703577601,"urgency":1,"urgency_name":"normal","histui_imported_at":1703577601}
`
	err := os.WriteFile(path, []byte(content), 0600)
	require.NoError(t, err)

	// Recover
	err = RecoverFromCorruption(path)
	require.NoError(t, err)

	// Verify recovered file
	p, err := NewJSONLPersistence(path)
	require.NoError(t, err)
	defer p.Close()

	notifications, err := p.Load()
	require.NoError(t, err)
	assert.Len(t, notifications, 2)

	// Backup should exist
	matches, _ := filepath.Glob(path + ".corrupted.*")
	assert.Len(t, matches, 1)
}

func persistTestNotification(id string) model.Notification {
	return model.Notification{
		HistuiID:         id,
		HistuiSource:     "test",
		HistuiImportedAt: time.Now().Unix(),
		AppName:          "test-app",
		Summary:          "Test Summary " + id, // Include ID to make content unique
		Body:             "Test Body",
		Timestamp:        time.Now().Unix(),
		Urgency:          model.UrgencyNormal,
		UrgencyName:      "normal",
	}
}
