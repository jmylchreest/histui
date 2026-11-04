package display

// Native constructors return owned references, including sunk floating widget
// references. Parent containers hold separate references. Release in reverse
// construction order after the popup is detached from its parent.
type nativeObject interface{ Unref() }

func own[T nativeObject](p *Popup, object T) T {
	p.owned = append(p.owned, object)
	return object
}
func ptr[T any](value T) *T { return &value }

// dispose is GTK-thread-only and runs after stack detachment. Close merely
// records logical dismissal and must not free widgets while a signal runs.
func (p *Popup) dispose() {
	if p.disposed {
		return
	}
	p.disposed = true
	p.sources.Close()
	for i := len(p.owned) - 1; i >= 0; i-- {
		p.owned[i].Unref()
	}
	p.owned = nil
}
