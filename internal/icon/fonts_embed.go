// Package icon provides icon name resolution and font loading.
package icon

import "embed"

// EmbeddedFonts contains all bundled font files.
// Add fonts to the internal/icon/fonts/ directory to include them.
//
//go:embed fonts
var EmbeddedFonts embed.FS
