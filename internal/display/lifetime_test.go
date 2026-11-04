package display

import (
	"codeberg.org/puregotk/puregotk/v4/adw"
	"codeberg.org/puregotk/puregotk/v4/glib"
	"codeberg.org/puregotk/puregotk/v4/gobject"
	"codeberg.org/puregotk/puregotk/v4/gtk"
	godbus "github.com/godbus/dbus/v5"
	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/dbus"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestPopupNativeLifetime(t *testing.T) {
	if os.Getenv("HISTUI_TEST_GTK") != "1" {
		t.Skip("requires isolated GTK display")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !gtk.InitCheck() {
		t.Fatal("GTK unavailable")
	}
	adw.Init()
	context := glib.MainContextDefault()
	drain := func() {
		for context.Iteration(false) {
		}
	}
	cfg := config.DefaultDaemonConfig()
	destroyed := 0
	finalized := gobject.WeakNotify(func(uintptr, uintptr) { destroyed++ })
	expected := 0
	window := gtk.NewWindow()
	defer func() { window.Destroy(); window.Unref() }()
	for i := 0; i < 2100; i++ {
		notification := &dbus.DBusNotification{
			AppName: "lifetime", AppIcon: "missing-lifetime-icon", Summary: "Popup lifetime",
			Body: "<b>Formatted body</b>", Actions: []string{"default", "Open", "dismiss", "Dismiss"},
			Hints: map[string]godbus.Variant{"value": godbus.MakeVariant(int32(45))},
		}
		popup, err := NewPopupWidget(notification, cfg, nil, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, object := range popup.owned {
			native := object.(interface{ GoPointer() uintptr })
			gobject.ObjectNewFromInternalPtr(native.GoPointer()).WeakRef(&finalized, 0)
			expected++
		}
		window.SetChild(&popup.box.Widget)
		popup.SetStackCount(2)
		// Alternate cancellation before dispatch and cancellation of the flash timer.
		if i%2 == 0 {
			drain()
		}
		window.SetChild(nil)
		popup.Close()
		popup.dispose()
		popup.dispose()
		drain()
	}
	deadline := time.Now().Add(time.Second)
	for destroyed != expected && time.Now().Before(deadline) {
		drain()
		time.Sleep(time.Millisecond)
	}
	if destroyed != expected {
		t.Fatalf("native objects leaked: finalized %d of %d", destroyed, expected)
	}
	if err := glib.UnrefCallback(&finalized); err != nil {
		t.Fatal(err)
	}
	t.Logf("finalized %d native objects across 2100 popups", destroyed)
}
