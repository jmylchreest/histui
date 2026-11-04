//go:build puregotk && !headless

package theme

import (
	"codeberg.org/puregotk/puregotk/v4/glib"
	"codeberg.org/puregotk/puregotk/v4/gtk"
)

func initTestGTK() bool { return gtk.InitCheck() }
func drainTestGTK() {
	main := glib.MainContextDefault()
	for main.Iteration(false) {
	}
}

func testProviderCSS(p *cssProvider) string { return p.ToString() }
