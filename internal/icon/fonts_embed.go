// Package icon provides icon name resolution and font loading.
package icon

import (
	"embed"

	histuiembed "github.com/jmylchreest/histui/embed"
)

// EmbeddedFonts provides access to bundled font files.
// Fonts are stored in the embed package's fonts directory.
var EmbeddedFonts embed.FS = histuiembed.Fonts
