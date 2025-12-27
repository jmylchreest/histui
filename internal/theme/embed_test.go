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
	// Should hide icons
	assert.Contains(t, css, "-gtk-icon-size: 0")
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

func TestListEmbeddedThemes(t *testing.T) {
	themes := ListEmbeddedThemes()

	// Should have all bundled themes
	assert.GreaterOrEqual(t, len(themes), 3)
	assert.Contains(t, themes, "default", "should contain default theme")
	assert.Contains(t, themes, "minimal", "should contain minimal theme")
	assert.Contains(t, themes, "catppuccin", "should contain catppuccin theme")
}

func TestIsEmbeddedTheme(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"default", true},
		{"minimal", true},
		{"catppuccin", true},
		{"nonexistent", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmbeddedTheme(tt.name)
			assert.Equal(t, tt.expected, result)
		})
	}
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

func TestGetEmbeddedPartial_NotFound(t *testing.T) {
	// No bundled partials currently, so any request should return not found
	css, found := GetEmbeddedPartial("_nonexistent.css")
	assert.False(t, found)
	assert.Empty(t, css)
}

func TestListEmbeddedThemes_ExcludesPartials(t *testing.T) {
	themes := ListEmbeddedThemes()

	// Should not include partials (files starting with _)
	for _, name := range themes {
		assert.False(t, strings.HasPrefix(name, "_"),
			"theme list should not include partials, found: %s", name)
	}
}
