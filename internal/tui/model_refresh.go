package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) waitForFileChange() tea.Cmd {
	if m.fileWatch == nil {
		return nil
	}
	return m.fileWatch.wait
}

func (m *Model) requestRefresh() tea.Cmd {
	return m.readNotificationsCmd(m.refresh.request())
}

func (m Model) readNotificationsCmd(request uint64) tea.Cmd {
	if request == 0 {
		return nil
	}
	database := m.db
	return func() tea.Msg { return readNotifications(database, request) }
}

func (m Model) finishRefresh(result notificationsLoadedMsg) (tea.Model, tea.Cmd) {
	accept, next := m.refresh.complete(result.request)
	if !accept {
		return m, m.readNotificationsCmd(next)
	}
	if result.err != nil {
		m.statusMsg = fmt.Sprintf("Failed to refresh notifications: %v", result.err)
		m.statusErr = true
		return m, nil
	}

	var selectedID string
	if selected, ok := m.list.SelectedItem().(notificationItem); ok {
		selectedID = selected.notification.HistuiID
	}
	m.notifications = result.notifications
	m.stackTagCounts = result.stackCounts
	cmd := m.rebuildListItems()
	for index, item := range m.list.Items() {
		if item.(notificationItem).notification.HistuiID == selectedID {
			m.list.Select(index)
			break
		}
	}
	if m.previewActive && m.mode == ModeList {
		if selected, ok := m.list.SelectedItem().(notificationItem); ok {
			n := selected.notification
			m.selected = &n
		} else {
			m.selected = nil
		}
	}
	// A notification can be announced before its image has finished encoding.
	// Recollect image sources on reconciliation, without caching missing images.
	if m.mode == ModeDetail && m.selected != nil {
		for _, n := range m.notifications {
			if n.HistuiID == m.selected.HistuiID {
				m.selected = &n
				var previous imageSource
				if m.detailImageIndex >= 0 && m.detailImageIndex < len(m.detailImages) {
					previous = m.detailImages[m.detailImageIndex]
				}
				m.detailImages = m.collectImages(n)
				m.detailImageIndex = 0
				for index, source := range m.detailImages {
					if source.label == previous.label && source.path == previous.path &&
						source.imageRef == previous.imageRef && source.hintKey == previous.hintKey {
						m.detailImageIndex = index
						break
					}
				}
				m.viewport.SetContent(m.renderDetail(n, m.calculateDetailTextWidth()))
				break
			}
		}
	}
	return m, cmd
}

// rebuildListItems keeps selection valid when filtering or deleting shortens the
// list. SetItems alone can leave Bubbles' cursor past the last visible item.
func (m *Model) rebuildListItems() tea.Cmd {
	cmd := m.list.SetItems(m.buildListItems())
	m.list.Select(max(0, min(m.list.Index(), len(m.list.Items())-1)))
	return cmd
}
