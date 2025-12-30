// Package embed provides embedded assets for histui.
// This includes themes, fonts, and icon aliases.
package embed

import "embed"

// Themes contains all bundled theme directories.
// Each theme is a directory containing theme.css and optional assets.
//
//go:embed themes
var Themes embed.FS

// Fonts contains embedded font files (Nerd Font symbols).
//
//go:embed fonts
var Fonts embed.FS

// AliasesDefaultTOML contains the default icon aliases configuration.
//
//go:embed aliases_default.toml
var AliasesDefaultTOML []byte
