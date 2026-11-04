//go:build puregotk_layershell && !headless

package theme

import (
	"codeberg.org/puregotk/puregotk/v4/gdk"
	"codeberg.org/puregotk/puregotk/v4/glib"
	"codeberg.org/puregotk/puregotk/v4/gtk"
)

type cssProvider = gtk.CssProvider
type themeDisplay = gdk.Display

const themePriority = uint(gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

func newCSSProvider() *cssProvider { return gtk.NewCssProvider() }

// The generated getter returns an owned reference, even for the default display.
func defaultThemeDisplay() *themeDisplay  { return gdk.DisplayGetDefault() }
func retainThemeDisplay(d *themeDisplay)  { d.Ref() }
func releaseThemeDisplay(d *themeDisplay) { d.Unref() }
func releaseCSSProvider(p *cssProvider)   { p.Unref() }
func addThemeProvider(d *themeDisplay, p *cssProvider, priority uint) {
	gtk.StyleContextAddProviderForDisplay(d, p, uint32(priority))
}
func removeThemeProvider(d *themeDisplay, p *cssProvider) {
	gtk.StyleContextRemoveProviderForDisplay(d, p)
}
func addThemeIdle(fn func()) uint32 {
	callback := glib.SourceOnceFunc(func(uintptr) { fn() })
	return glib.IdleAddOnce(&callback, 0)
}
func removeThemeIdle(id uint32) { glib.SourceRemove(id) }
