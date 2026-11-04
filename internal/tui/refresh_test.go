package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jmylchreest/histui/internal/db"
	"github.com/jmylchreest/histui/internal/model"
	"github.com/stretchr/testify/require"
)

func TestRefreshCoalescesAndRejectsStaleResults(t *testing.T) {
	m := New(nil, nil, nil, nil)
	first := m.requestRefresh()
	require.NotNil(t, first)
	for range 20 {
		require.Nil(t, m.requestRefresh(), "only one read may run at a time")
	}
	old := first().(notificationsLoadedMsg)
	old.notifications = []model.Notification{{HistuiID: "stale"}}
	updated, followup := m.finishRefresh(old)
	m = updated.(Model)
	require.Empty(t, m.notifications, "invalidation makes old snapshot obsolete")
	require.NotNil(t, followup)
	current := followup().(notificationsLoadedMsg)
	current.notifications = []model.Notification{{HistuiID: "current"}}
	updated, _ = m.finishRefresh(current)
	m = updated.(Model)
	require.Equal(t, "current", m.notifications[0].HistuiID)
	updated, cmd := m.finishRefresh(old)
	require.Nil(t, cmd)
	require.Equal(t, "current", updated.(Model).notifications[0].HistuiID)
}

func TestRefreshPreservesRowsOnReadError(t *testing.T) {
	m := New(nil, nil, nil, nil)
	m.notifications = []model.Notification{{HistuiID: "existing"}}
	cmd := m.requestRefresh()
	result := cmd().(notificationsLoadedMsg)
	result.err = fmt.Errorf("read failed")
	updated, _ := m.finishRefresh(result)
	m = updated.(Model)
	require.Len(t, m.notifications, 1)
	require.Equal(t, "existing", m.notifications[0].HistuiID)
	require.True(t, m.statusErr)
	require.Contains(t, m.statusMsg, "read failed")
}

func TestRefreshReadDoesNotAccessMutableModel(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "history.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	m := New(nil, database, nil, nil)
	cmd := m.requestRefresh()
	completed := make(chan notificationsLoadedMsg, 1)
	go func() { completed <- cmd().(notificationsLoadedMsg) }()
	// The worker must not access these UI-owned fields. Run this under -race.
	for i := range 1000 {
		m.searchQuery = fmt.Sprint(i)
		m.notifications = append(m.notifications, model.Notification{})
		m.activeIDs = map[string]bool{fmt.Sprint(i): true}
	}
	require.NoError(t, (<-completed).err)
}

func TestRefreshUsesFullHistoryCountsAndCurrentFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	writer, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })
	reader, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })
	for i := range 3 {
		n, err := model.NewNotification("test")
		require.NoError(t, err)
		n.Summary = fmt.Sprintf("record %d", i)
		n.Extensions = &model.Extensions{StackTag: "group"}
		if i == 2 {
			n.HistuiDismissedAt = 1
		}
		require.NoError(t, writer.AddNotification(n))
	}
	m := New(nil, reader, nil, nil)
	cmd := m.requestRefresh()
	// A user can change filters while the database query is running.
	m.filterMode = FilterDismissed
	updated, _ := m.finishRefresh(cmd().(notificationsLoadedMsg))
	m = updated.(Model)
	require.Len(t, m.notifications, 3)
	require.Len(t, m.list.Items(), 1)
	require.Equal(t, 3, m.list.Items()[0].(notificationItem).stackCount)
	require.Equal(t, FilterDismissed, m.filterMode)
}

func TestRefreshRecollectsLateImageInDetail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	writer, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })
	reader, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })
	n, err := model.NewNotification("test")
	require.NoError(t, err)
	n.Summary = "late image"
	require.NoError(t, writer.AddNotification(n))
	m := New(nil, reader, nil, nil)
	m.mode, m.selected = ModeDetail, n
	cmd := m.requestRefresh()
	updated, _ := m.finishRefresh(cmd().(notificationsLoadedMsg))
	m = updated.(Model)
	require.Empty(t, m.detailImages)
	require.NoError(t, writer.SaveImage(n.HistuiID, db.ImageRefImage, []byte("stored image bytes")))
	cmd = m.requestRefresh()
	updated, _ = m.finishRefresh(cmd().(notificationsLoadedMsg))
	m = updated.(Model)
	require.Len(t, m.detailImages, 1)
	require.Equal(t, db.ImageRefImage, m.detailImages[0].imageRef)
}

func TestRefreshPreservesSelectionAfterConcurrentInsert(t *testing.T) {
	m := New(nil, nil, nil, nil)
	m.filterMode = FilterAll
	m.notifications = []model.Notification{{HistuiID: "selected"}}
	m.list.SetItems(m.buildListItems())
	cmd := m.requestRefresh()
	result := cmd().(notificationsLoadedMsg)
	result.notifications = []model.Notification{{HistuiID: "newest"}, {HistuiID: "selected"}}
	updated, _ := m.finishRefresh(result)
	m = updated.(Model)
	require.Equal(t, "selected", m.list.SelectedItem().(notificationItem).notification.HistuiID)
}

func TestSearchFromLaterSelectionCanOpenOnlyMatch(t *testing.T) {
	m := New(nil, nil, nil, nil)
	m.filterMode = FilterAll
	m.notifications = []model.Notification{{HistuiID: "alpha", Summary: "Alpha"}, {HistuiID: "beta", Summary: "Beta"}}
	m.rebuildListItems()
	m.list.Select(1)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Alpha")})
	m = updated.(Model)
	require.Len(t, m.list.Items(), 1)
	require.NotNil(t, m.list.SelectedItem())
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	require.Equal(t, ModeDetail, m.mode)
	require.Equal(t, "alpha", m.selected.HistuiID)
}

func TestRefreshClampsSelectionWhenLastRowDisappears(t *testing.T) {
	m := New(nil, nil, nil, nil)
	m.filterMode = FilterAll
	m.notifications = []model.Notification{{HistuiID: "first"}, {HistuiID: "deleted"}}
	m.rebuildListItems()
	m.list.Select(1)
	cmd := m.requestRefresh()
	result := cmd().(notificationsLoadedMsg)
	result.notifications = []model.Notification{{HistuiID: "first"}}
	updated, _ := m.finishRefresh(result)
	m = updated.(Model)
	require.NotNil(t, m.list.SelectedItem())
	require.Equal(t, "first", m.list.SelectedItem().(notificationItem).notification.HistuiID)
}

func TestDetailResizeKeepsContentAndScrollOffset(t *testing.T) {
	m := New(nil, nil, nil, nil)
	m.mode = ModeDetail
	m.selected = &model.Notification{HistuiID: "resize", Summary: "Resize", Body: strings.Repeat("body-line with enough text to preserve a scrollable viewport\n", 80)}
	m.viewport.SetContent(m.renderDetail(*m.selected, 80))
	m.viewport.SetYOffset(10)
	require.Equal(t, 10, m.viewport.YOffset)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 72, Height: 24})
	m = updated.(Model)
	require.Equal(t, 10, m.viewport.YOffset)
	require.Contains(t, m.viewport.View(), "body-line")
	require.Equal(t, 72, m.viewport.Width)
	require.Equal(t, 20, m.viewport.Height)
}

func BenchmarkNotificationRefresh(b *testing.B) {
	database, err := db.Open(filepath.Join(b.TempDir(), "history.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = database.Close() })
	for i := range 1000 {
		n, err := model.NewNotification("benchmark")
		if err != nil {
			b.Fatal(err)
		}
		n.Summary = fmt.Sprintf("notification %d", i)
		n.Extensions = &model.Extensions{StackTag: fmt.Sprint(i % 20)}
		if err := database.AddNotification(n); err != nil {
			b.Fatal(err)
		}
	}
	b.Run("PreviousReadAndAggregate", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := database.All(); err != nil {
				b.Fatal(err)
			}
			if _, err := database.GetStackTagCounts(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("SingleSnapshot", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if result := readNotifications(database, 1); result.err != nil {
				b.Fatal(result.err)
			}
		}
	})
}
