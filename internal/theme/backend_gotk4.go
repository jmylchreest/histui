//go:build cgo && !puregotk && !headless

package theme

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type cssProvider = gtk.CSSProvider
type themeDisplay = gdk.Display

const themePriority = gtk.STYLE_PROVIDER_PRIORITY_APPLICATION

func newCSSProvider() *cssProvider       { return gtk.NewCSSProvider() }
func defaultThemeDisplay() *themeDisplay { return gdk.DisplayGetDefault() }

// gotk4 wrappers retain native objects and release them through finalizers.
func retainThemeDisplay(*themeDisplay)  {}
func releaseThemeDisplay(*themeDisplay) {}
func releaseCSSProvider(*cssProvider)   {}
func addThemeProvider(d *themeDisplay, p *cssProvider, priority uint) {
	gtk.StyleContextAddProviderForDisplay(d, p, priority)
}
func removeThemeProvider(d *themeDisplay, p *cssProvider) {
	gtk.StyleContextRemoveProviderForDisplay(d, p)
}
func addThemeIdle(fn func()) uint32 { return uint32(glib.IdleAdd(fn)) }
func removeThemeIdle(id uint32)     { glib.SourceRemove(glib.SourceHandle(id)) }
