//go:build puregotk_layershell && !headless

package theme

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Explicit opt-in prevents ordinary unit tests from connecting to the user's
// desktop. Run under xvfb-run with HISTUI_TEST_GTK=1 and GDK_BACKEND=x11.
func TestGTKLoaderLifecycle(t *testing.T) {
	if os.Getenv("HISTUI_TEST_GTK") != "1" {
		t.Skip("requires isolated GTK display")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !initTestGTK() {
		t.Fatal("GTK initialization failed")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	drain := drainTestGTK
	// t.Run starts a new goroutine; keep native operations on this locked thread.
	run := func(name string, fn func(*testing.T)) { t.Log(name); fn(t) }

	run("bundled themes", func(t *testing.T) {
		l := NewLoader(nil)
		defer l.Close()
		for _, name := range BundledThemes {
			if err := l.LoadTheme(name); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if l.GetTheme().Name != name || testProviderCSS(l.provider) == "" {
				t.Fatalf("%s was not loaded", name)
			}
		}
	})

	run("resolution and ownership", func(t *testing.T) {
		l := NewLoader(nil)
		defer l.Close()
		dir := filepath.Join(l.themesDir, "custom")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "colors.css"), []byte(".notification { color: rgb(12, 34, 56); }"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "theme.css"), []byte("@import \"colors.css\";"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := l.LoadTheme("custom"); err != nil {
			t.Fatal(err)
		}
		theme := l.GetTheme()
		if theme.Manifest == nil || theme.Layout != "" || len(theme.ImportedFiles) != 1 {
			t.Fatalf("lost partial-theme inheritance/imports: %+v", theme)
		}
		if !strings.Contains(testProviderCSS(l.provider), "12") {
			t.Fatal("imported CSS was not loaded into GTK")
		}
		// Font overrides created before Apply must also be attached and released.
		l.ApplyFontOverrides("Sans", 14)
		l.Apply(nil)
		l.Apply(l.display)
		l.ApplyFontOverrides("", 0)
		if l.fontProvider != nil {
			t.Fatal("font provider not released")
		}
		l.Close()
		l.Close()
		if l.display != nil || l.provider != nil {
			t.Fatal("native resources retained after close")
		}
		if err := l.LoadTheme("default"); err == nil {
			t.Fatal("closed loader accepted a theme")
		}
	})

	run("cancel queued and stale reloads", func(t *testing.T) {
		l := NewLoader(nil)
		defer l.Close()
		called := 0
		old := l.watchGeneration
		l.queueReload(old, func() { called++ })
		l.StopHotReload()
		l.queueReload(old, func() { called++ })
		drain()
		if called != 0 {
			t.Fatal("stale reload ran after stop")
		}
		l.queueReload(l.watchGeneration, func() { called++ })
		drain()
		if called != 1 || len(l.pendingReloads) != 0 {
			t.Fatal("live reload did not finish cleanly")
		}
		// More than the native callback pool size; both fired and cancelled sources
		// must release their callback registrations rather than exhausting the pool.
		for i := 0; i < 2100; i++ {
			l.queueReload(l.watchGeneration, func() { called++ })
			if i%2 == 0 {
				l.StopHotReload()
			} else {
				drain()
			}
		}
		l.queueReload(l.watchGeneration, func() { t.Error("reload ran after close") })
		l.Close()
		drain()
	})

	run("watcher reload and shutdown", func(t *testing.T) {
		l := NewLoader(nil)
		defer l.Close()
		if err := l.LoadTheme("custom"); err != nil {
			t.Fatal(err)
		}
		reloaded := false
		l.SetHotReloadCallback(func(name, file string) { reloaded = true })
		l.StartHotReload(context.Background())
		path := filepath.Join(l.themesDir, "custom", "colors.css")
		if err := os.WriteFile(path, []byte(".notification { color: rgb(98, 76, 54); }"), 0600); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for !reloaded && time.Now().Before(deadline) {
			drain()
			time.Sleep(10 * time.Millisecond)
		}
		if !reloaded || !strings.Contains(testProviderCSS(l.provider), "98") {
			t.Fatal("watcher did not apply imported CSS update")
		}
		generation := l.watchGeneration
		queued := make(chan struct{})
		go func() {
			defer close(queued)
			for i := 0; i < 100; i++ {
				l.queueReload(generation, func() {})
			}
		}()
		l.StopHotReload()
		<-queued
		drain()
		if len(l.pendingReloads) != 0 {
			t.Fatal("reloads remained after stop")
		}
	})
}
