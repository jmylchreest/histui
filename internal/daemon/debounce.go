// Package daemon provides utilities for histuid daemon functionality.
package daemon

import (
	"sync"
	"time"
)

// Debouncer provides a simple debounce mechanism for function calls.
// Multiple calls within the debounce period are coalesced into a single call.
type Debouncer struct {
	mu       sync.Mutex
	duration time.Duration
	timer    *time.Timer
	fn       func()
}

// NewDebouncer creates a new debouncer with the specified duration.
func NewDebouncer(duration time.Duration, fn func()) *Debouncer {
	return &Debouncer{
		duration: duration,
		fn:       fn,
	}
}

// Trigger schedules the function to run after the debounce duration.
// If called again before the duration elapses, the timer is reset.
func (d *Debouncer) Trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel existing timer
	if d.timer != nil {
		d.timer.Stop()
	}

	// Schedule new timer
	d.timer = time.AfterFunc(d.duration, func() {
		d.mu.Lock()
		d.timer = nil
		d.mu.Unlock()
		d.fn()
	})
}

// Stop cancels any pending debounced call.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}
