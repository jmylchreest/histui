//go:build cgo && !puregotk && !headless

package theme

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func initTestGTK() bool { return gtk.InitCheck() }
func drainTestGTK() {
	main := glib.MainContextDefault()
	for main.Iteration(false) {
	}
}

func testProviderCSS(p *cssProvider) string { return p.String() }
