package store

import (
	"testing"
	"time"

	"github.com/jmylchreest/histui/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	s := NewStore(nil)
	assert.NotNil(t, s)
	assert.Equal(t, 0, s.Count())
}

func TestStore_Add(t *testing.T) {
	s := NewStore(nil)
	defer s.Close()

	n := testNotification("test1")
	err := s.Add(n)
	require.NoError(t, err)
	assert.Equal(t, 1, s.Count())

	// Add duplicate - should be skipped
	err = s.Add(n)
	require.NoError(t, err)
	assert.Equal(t, 1, s.Count())

	// Add different notification
	n2 := testNotification("test2")
	err = s.Add(n2)
	require.NoError(t, err)
	assert.Equal(t, 2, s.Count())
}

func TestStore_AddBatch(t *testing.T) {
	s := NewStore(nil)
	defer s.Close()

	ns := []model.Notification{
		testNotification("batch1"),
		testNotification("batch2"),
		testNotification("batch3"),
	}

	err := s.AddBatch(ns)
	require.NoError(t, err)
	assert.Equal(t, 3, s.Count())

	// Add batch with duplicates
	ns2 := []model.Notification{
		testNotification("batch3"), // duplicate
		testNotification("batch4"), // new
	}
	err = s.AddBatch(ns2)
	require.NoError(t, err)
	assert.Equal(t, 4, s.Count())
}

func TestStore_All(t *testing.T) {
	s := NewStore(nil)
	defer s.Close()

	now := time.Now().Unix()
	n1 := testNotificationWithTime("old", now-100)
	n2 := testNotificationWithTime("new", now)

	s.Add(n1)
	s.Add(n2)

	all := s.All()
	require.Len(t, all, 2)

	// Should be sorted newest first
	assert.Equal(t, "new", all[0].HistuiID)
	assert.Equal(t, "old", all[1].HistuiID)
}

func TestStore_Filter(t *testing.T) {
	s := NewStore(nil)
	defer s.Close()

	now := time.Now().Unix()

	// Add notifications with different properties
	n1 := model.Notification{
		HistuiID:         "filter1",
		HistuiSource:     "test",
		HistuiImportedAt: now,
		AppName:          "firefox",
		Summary:          "Old Firefox",
		Timestamp:        now - 3600, // 1 hour ago
		Urgency:          model.UrgencyNormal,
		UrgencyName:      "normal",
	}
	n2 := model.Notification{
		HistuiID:         "filter2",
		HistuiSource:     "test",
		HistuiImportedAt: now,
		AppName:          "slack",
		Summary:          "Recent Slack",
		Timestamp:        now - 60, // 1 minute ago
		Urgency:          model.UrgencyNormal,
		UrgencyName:      "normal",
	}
	n3 := model.Notification{
		HistuiID:         "filter3",
		HistuiSource:     "test",
		HistuiImportedAt: now,
		AppName:          "firefox",
		Summary:          "New Firefox",
		Timestamp:        now, // now
		Urgency:          model.UrgencyCritical,
		UrgencyName:      "critical",
	}

	s.Add(n1)
	s.Add(n2)
	s.Add(n3)

	t.Run("filter by since", func(t *testing.T) {
		result := s.Filter(FilterOptions{Since: 30 * time.Minute})
		assert.Len(t, result, 2) // Only last 30 minutes (n2 and n3)
	})

	t.Run("filter by app", func(t *testing.T) {
		result := s.Filter(FilterOptions{AppFilter: "firefox"})
		assert.Len(t, result, 2) // n1 and n3
	})

	t.Run("filter by urgency", func(t *testing.T) {
		urgency := model.UrgencyCritical
		result := s.Filter(FilterOptions{Urgency: &urgency})
		assert.Len(t, result, 1) // n3
	})

	t.Run("filter with limit", func(t *testing.T) {
		result := s.Filter(FilterOptions{Limit: 2})
		assert.Len(t, result, 2)
	})

	t.Run("sort by app asc", func(t *testing.T) {
		result := s.Filter(FilterOptions{SortField: "app", SortOrder: "asc"})
		assert.Equal(t, "firefox", result[0].AppName)
	})

	t.Run("combined filters", func(t *testing.T) {
		result := s.Filter(FilterOptions{
			AppFilter: "firefox",
			Limit:     1,
		})
		assert.Len(t, result, 1)
	})
}

func TestStore_Lookup(t *testing.T) {
	s := NewStore(nil)
	defer s.Close()

	n := testNotification("ULID12345678901234567890")
	n.AppName = "firefox"
	n.Summary = "Download Complete"
	s.Add(n)

	t.Run("lookup by exact ULID", func(t *testing.T) {
		result := s.Lookup("ULID12345678901234567890")
		require.NotNil(t, result)
		assert.Equal(t, "firefox", result.AppName)
	})

	t.Run("lookup by ULID prefix in string", func(t *testing.T) {
		result := s.Lookup("ULID12345678901234567890 | firefox | Download Complete")
		require.NotNil(t, result)
		assert.Equal(t, "firefox", result.AppName)
	})

	t.Run("lookup by content fallback", func(t *testing.T) {
		result := s.Lookup("firefox | Download Complete | 5m ago")
		require.NotNil(t, result)
		assert.Equal(t, "firefox", result.AppName)
	})

	t.Run("lookup not found", func(t *testing.T) {
		result := s.Lookup("nonexistent")
		assert.Nil(t, result)
	})
}

func TestStore_Delete(t *testing.T) {
	s := NewStore(nil)
	defer s.Close()

	n1 := testNotification("delete1")
	n2 := testNotification("delete2")
	s.Add(n1)
	s.Add(n2)

	assert.Equal(t, 2, s.Count())

	err := s.Delete("delete1")
	require.NoError(t, err)
	assert.Equal(t, 1, s.Count())

	// Verify n2 is still there
	result := s.GetByID("delete2")
	require.NotNil(t, result)
}

func TestStore_Subscribe(t *testing.T) {
	s := NewStore(nil)
	defer s.Close()

	ch := s.Subscribe()
	require.NotNil(t, ch)

	// Add notification
	go func() {
		s.Add(testNotification("sub1"))
	}()

	// Should receive event
	select {
	case event := <-ch:
		assert.Equal(t, ChangeTypeAdd, event.Type)
		assert.Equal(t, 1, event.Count)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestStore_Unsubscribe(t *testing.T) {
	s := NewStore(nil)

	ch := s.Subscribe()
	s.Unsubscribe(ch)

	// Channel should be closed
	_, ok := <-ch
	assert.False(t, ok)

	s.Close()
}

func TestStore_Clear(t *testing.T) {
	s := NewStore(nil)
	defer s.Close()

	s.Add(testNotification("clear1"))
	s.Add(testNotification("clear2"))
	assert.Equal(t, 2, s.Count())

	err := s.Clear()
	require.NoError(t, err)
	assert.Equal(t, 0, s.Count())
}

func TestStore_Close(t *testing.T) {
	s := NewStore(nil)
	s.Add(testNotification("close1"))

	err := s.Close()
	require.NoError(t, err)

	// Operations should fail on closed store
	err = s.Add(testNotification("close2"))
	assert.ErrorIs(t, err, ErrStoreClosed)
}

// Helper functions

func testNotification(id string) model.Notification {
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

func testNotificationWithTime(id string, timestamp int64) model.Notification {
	n := testNotification(id)
	n.Timestamp = timestamp
	return n
}

func testNotificationWithApp(app string, timestamp int64) model.Notification {
	n := testNotification(app + "-" + time.Now().Format("150405.000"))
	n.AppName = app
	n.Timestamp = timestamp
	return n
}
