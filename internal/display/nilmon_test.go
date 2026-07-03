package display

import (
	"testing"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

// nullMonitor must resolve to a NULL native pointer so layershell.SetMonitor
// forwards gtk_layer_set_monitor(window, NULL) ("let the compositor choose")
// instead of panicking on a typed-nil *gdk.Monitor. Guards the unsafe layout
// assumption against gotk4 upgrades.
func TestNullMonitorIsNullNative(t *testing.T) {
	m := nullMonitor()
	if m == nil {
		t.Fatal("nullMonitor returned a typed-nil pointer; SetMonitor would panic on it")
	}
	if base := coreglib.BaseObject(m); base != nil {
		t.Fatalf("nullMonitor base object = %v, want nil (native pointer must be NULL)", base)
	}
}
