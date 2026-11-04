package tui

import (
	"github.com/jmylchreest/histui/internal/db"
	"github.com/jmylchreest/histui/internal/model"
)

// refreshState is owned exclusively by the Update loop. Workers receive only
// their request number and database handle, never this state or the UI model.
type refreshState struct {
	requested uint64
	active    uint64
}

func (s *refreshState) request() uint64 {
	s.requested++
	if s.active != 0 {
		return 0
	}
	s.active = s.requested
	return s.active
}

// complete rejects superseded results and starts at most one follow-up read.
func (s *refreshState) complete(request uint64) (accept bool, next uint64) {
	if request == 0 || request != s.active {
		return false, 0
	}
	if s.requested != s.active {
		s.active = s.requested
		return false, s.active
	}
	s.active = 0
	return true, 0
}

type notificationsLoadedMsg struct {
	request       uint64
	notifications []model.Notification
	stackCounts   map[string]int
	err           error
}

func readNotifications(database *db.DB, request uint64) notificationsLoadedMsg {
	result := notificationsLoadedMsg{request: request, stackCounts: make(map[string]int)}
	if database == nil {
		return result
	}
	result.notifications, result.err = database.All()
	if result.err != nil {
		return result
	}
	// Keep the existing complete-history semantics. Derive stack totals from
	// the same read snapshot so concurrent writes cannot skew rows vs counts.
	for _, n := range result.notifications {
		if n.Extensions != nil && n.Extensions.StackTag != "" {
			result.stackCounts[n.Extensions.StackTag]++
		}
	}
	for tag, count := range result.stackCounts {
		if count < 2 {
			delete(result.stackCounts, tag)
		}
	}
	return result
}
