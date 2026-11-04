//go:build linux && puregotk

// Package layershell is a deliberately narrow binding to the existing native
// gtk4-layer-shell library used by the Codeberg GTK backend.
// It owns no GTK objects and does not implement the Wayland protocol itself.
package layershell

import (
	"fmt"
	"sync"

	"codeberg.org/puregotk/purego"
	"codeberg.org/puregotk/puregotk/v4/gdk"
	"codeberg.org/puregotk/puregotk/v4/gtk"
)

type Edge int32

const (
	EdgeLeftValue Edge = iota
	EdgeRightValue
	EdgeTopValue
	EdgeBottomValue
)

type Layer int32

const (
	LayerBackgroundValue Layer = iota
	LayerBottomValue
	LayerTopValue
	LayerOverlayValue
)

type KeyboardMode int32

const (
	KeyboardModeNoneValue KeyboardMode = iota
	KeyboardModeExclusiveValue
	KeyboardModeOnDemandValue
)

var once sync.Once
var loadErr error
var api struct {
	supported func() bool
	init      func(uintptr)
	layer     func(uintptr, Layer)
	zone      func(uintptr, int32)
	keyboard  func(uintptr, KeyboardMode)
	namespace func(uintptr, string)
	anchor    func(uintptr, Edge, bool)
	margin    func(uintptr, Edge, int32)
	monitor   func(uintptr, uintptr)
	getAnchor func(uintptr, Edge) bool
	getMargin func(uintptr, Edge) int32
	getLayer  func(uintptr) Layer
	isLayer   func(uintptr) bool
}

// The upstream early-loading patch and puregotk_layershell tag must load the
// native library before generated GTK/GDK initialization; calling Load from main
// alone cannot establish that order. The soname is resolved by the platform loader;
// no working-directory or application-provided library fallback is used.
// The handle is process-owned and intentionally never closed while GTK may use it.
func Load() error {
	once.Do(func() {
		h, err := purego.Dlopen("libgtk4-layer-shell.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			loadErr = fmt.Errorf("load gtk4-layer-shell: %w", err)
			return
		}
		for _, entry := range []struct {
			name   string
			target any
		}{
			{"gtk_layer_is_supported", &api.supported}, {"gtk_layer_init_for_window", &api.init},
			{"gtk_layer_set_layer", &api.layer}, {"gtk_layer_set_exclusive_zone", &api.zone},
			{"gtk_layer_set_keyboard_mode", &api.keyboard}, {"gtk_layer_set_namespace", &api.namespace},
			{"gtk_layer_set_anchor", &api.anchor}, {"gtk_layer_set_margin", &api.margin},
			{"gtk_layer_set_monitor", &api.monitor}, {"gtk_layer_get_anchor", &api.getAnchor},
			{"gtk_layer_get_margin", &api.getMargin}, {"gtk_layer_get_layer", &api.getLayer},
			{"gtk_layer_is_layer_window", &api.isLayer},
		} {
			symbol, err := purego.Dlsym(h, entry.name)
			if err != nil {
				loadErr = fmt.Errorf("gtk4-layer-shell symbol %s: %w", entry.name, err)
				return
			}
			purego.RegisterFunc(entry.target, symbol)
		}
	})
	return loadErr
}
func mustLoad() {
	if err := Load(); err != nil {
		panic(err)
	}
}
func Available() bool   { return Load() == nil }
func IsSupported() bool { mustLoad(); return api.supported() }
func window(w *gtk.Window) uintptr {
	if w == nil {
		panic("nil layer-shell window")
	}
	return w.GoPointer()
}
func InitForWindow(w *gtk.Window)                   { mustLoad(); api.init(window(w)) }
func SetLayer(w *gtk.Window, v Layer)               { mustLoad(); api.layer(window(w), v) }
func SetExclusiveZone(w *gtk.Window, v int32)       { mustLoad(); api.zone(window(w), v) }
func SetKeyboardMode(w *gtk.Window, v KeyboardMode) { mustLoad(); api.keyboard(window(w), v) }
func SetNamespace(w *gtk.Window, v *string) {
	mustLoad()
	if v == nil {
		panic("nil layer-shell namespace")
	}
	api.namespace(window(w), *v)
}
func SetAnchor(w *gtk.Window, e Edge, v bool)  { mustLoad(); api.anchor(window(w), e, v) }
func SetMargin(w *gtk.Window, e Edge, v int32) { mustLoad(); api.margin(window(w), e, v) }
func SetMonitor(w *gtk.Window, m *gdk.Monitor) {
	mustLoad()
	var p uintptr
	if m != nil {
		p = m.GoPointer()
	}
	api.monitor(window(w), p)
}
func GetAnchor(w *gtk.Window, e Edge) bool  { mustLoad(); return api.getAnchor(window(w), e) }
func GetMargin(w *gtk.Window, e Edge) int32 { mustLoad(); return api.getMargin(window(w), e) }
func GetLayer(w *gtk.Window) Layer          { mustLoad(); return api.getLayer(window(w)) }
func IsLayerWindow(w *gtk.Window) bool      { mustLoad(); return api.isLayer(window(w)) }
