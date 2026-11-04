// Package gtkruntime owns dispatch to the GTK main context.
package gtkruntime

import (
	"sync"

	"codeberg.org/puregotk/puregotk/v4/glib"
)

func IdleAdd(fn func()) uint32 {
	callback := glib.SourceOnceFunc(func(uintptr) { fn() })
	return glib.IdleAddOnce(&callback, 0)
}
func TimeoutAdd(ms uint32, fn func() bool) uint32 {
	callback := glib.SourceFunc(func(uintptr) bool { return fn() })
	return glib.TimeoutAddFull(0, ms, &callback, 0, nil)
}

// Sources owns one-shot callbacks for an object. Close must run on the GTK
// thread; scheduling may come from workers. Cancellation releases callback slots.
type Sources struct {
	mu     sync.Mutex
	closed bool
	ids    map[uint32]struct{}
}

func (s *Sources) Idle(fn func())             { s.add(0, fn) }
func (s *Sources) After(ms uint32, fn func()) { s.add(ms, fn) }
func (s *Sources) add(ms uint32, fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	var id uint32
	run := func() {
		s.mu.Lock()
		delete(s.ids, id)
		valid := !s.closed
		s.mu.Unlock()
		if valid {
			fn()
		}
	}
	if ms == 0 {
		id = IdleAdd(run)
	} else {
		id = TimeoutAdd(ms, func() bool { run(); return false })
	}
	if s.ids == nil {
		s.ids = make(map[uint32]struct{})
	}
	s.ids[id] = struct{}{}
}
func (s *Sources) Close() {
	s.mu.Lock()
	s.closed = true
	ids := s.ids
	s.ids = nil
	s.mu.Unlock()
	for id := range ids {
		glib.SourceRemove(id)
	}
}
func IsMainThread() bool { return glib.MainContextDefault().IsOwner() }

// Requires the carried callback-lifetime fix; build with scripts/puregotk/go.sh.
var _ = glib.IsManagedCallback
