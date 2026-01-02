package theme

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEmbeddedTheme_Default(t *testing.T) {
	css, found := GetEmbeddedTheme("default")
	require.True(t, found, "default theme should be found")
	assert.NotEmpty(t, css)
	assert.Contains(t, css, ".notification-popup")
	assert.Contains(t, css, ".notification-summary")
	// Should use Adwaita variables
	assert.Contains(t, css, "@window_bg_color")
	assert.Contains(t, css, "@window_fg_color")
}

func TestGetEmbeddedTheme_Minimal(t *testing.T) {
	css, found := GetEmbeddedTheme("minimal")
	require.True(t, found, "minimal theme should be found")
	assert.NotEmpty(t, css)
	assert.Contains(t, css, ".notification-popup")
	// Should use Adwaita variables
	assert.Contains(t, css, "@window_bg_color")
	// Should have compact icons (32px)
	assert.Contains(t, css, "min-width: 32px")
}

func TestGetEmbeddedTheme_Catppuccin(t *testing.T) {
	css, found := GetEmbeddedTheme("catppuccin")
	require.True(t, found, "catppuccin theme should be found")
	assert.NotEmpty(t, css)
	assert.Contains(t, css, ".notification-popup")
	// Should have Catppuccin color tokens
	assert.Contains(t, css, "--ctp-text")
	assert.Contains(t, css, "--ctp-base")
	// Should support light/dark via .dark class
	assert.Contains(t, css, ".dark")
}

func TestGetEmbeddedTheme_NotFound(t *testing.T) {
	css, found := GetEmbeddedTheme("nonexistent")
	assert.False(t, found)
	assert.Empty(t, css)
}

func TestBundledThemes_HaveRequiredClasses(t *testing.T) {
	requiredClasses := []string{
		".notification-popup",
		".notification-summary",
		".notification-body",
		".notification-appname",
		".notification-close",
		".urgency-low",
		".urgency-normal",
		".urgency-critical",
	}

	for _, themeName := range BundledThemes {
		t.Run(themeName, func(t *testing.T) {
			css, found := GetEmbeddedTheme(themeName)
			require.True(t, found)

			for _, class := range requiredClasses {
				assert.True(t, strings.Contains(css, class),
					"theme %s should contain %s", themeName, class)
			}
		})
	}
}

func TestBundledThemes_ValidCSS(t *testing.T) {
	for _, themeName := range BundledThemes {
		t.Run(themeName, func(t *testing.T) {
			css, found := GetEmbeddedTheme(themeName)
			require.True(t, found)

			// Basic CSS validity checks
			// Braces should be balanced
			openBraces := strings.Count(css, "{")
			closeBraces := strings.Count(css, "}")
			assert.Equal(t, openBraces, closeBraces,
				"theme %s should have balanced braces", themeName)

			// Should not have obvious syntax errors
			assert.NotContains(t, css, "{{")
			assert.NotContains(t, css, "}}")
		})
	}
}

func TestGetEmbeddedPartial_Effects(t *testing.T) {
	// Test that effects.css partial can be loaded
	css, found := GetEmbeddedPartial("effects.css")
	require.True(t, found, "effects.css partial should be found")
	assert.Contains(t, css, "@keyframes sparkle")
	assert.Contains(t, css, "@keyframes pulse-glow")
	assert.Contains(t, css, ".anim-sparkle")
}

func TestGetEmbeddedPartial_EffectsLegacy(t *testing.T) {
	// Test that _effects.css (underscore prefix) also works
	css, found := GetEmbeddedPartial("_effects.css")
	require.True(t, found, "_effects.css partial should be found (legacy naming)")
	assert.Contains(t, css, "@keyframes sparkle")
}

func TestGetEmbeddedPartial_NotFound(t *testing.T) {
	css, found := GetEmbeddedPartial("_nonexistent.css")
	assert.False(t, found)
	assert.Empty(t, css)
}
