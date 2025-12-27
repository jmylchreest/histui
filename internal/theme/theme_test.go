package theme

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessImports_NoImports(t *testing.T) {
	css := `.notification-popup { color: red; }`
	result := ProcessImports(css, "", nil)
	assert.Equal(t, css, result)
}

func TestProcessImports_FileImport(t *testing.T) {
	// Create a temporary directory with test CSS files
	tmpDir := t.TempDir()

	// Create a partial file
	partialContent := `:root { --custom: #ff0000; }`
	partialPath := filepath.Join(tmpDir, "_custom.css")
	err := os.WriteFile(partialPath, []byte(partialContent), 0644)
	require.NoError(t, err)

	// Create main CSS that imports the partial
	mainCSS := `@import "_custom.css";
.notification-popup { color: var(--custom); }`

	result := ProcessImports(mainCSS, tmpDir, nil)

	assert.Contains(t, result, "/* imported: _custom.css */")
	assert.Contains(t, result, "--custom: #ff0000")
	assert.Contains(t, result, ".notification-popup")
}

func TestProcessImports_NestedImports(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested structure: main imports child, child imports grandchild
	grandchildContent := `.grandchild { color: blue; }`
	grandchildPath := filepath.Join(tmpDir, "_grandchild.css")
	err := os.WriteFile(grandchildPath, []byte(grandchildContent), 0644)
	require.NoError(t, err)

	childContent := `@import "_grandchild.css";
.child { color: green; }`
	childPath := filepath.Join(tmpDir, "_child.css")
	err = os.WriteFile(childPath, []byte(childContent), 0644)
	require.NoError(t, err)

	mainCSS := `@import "_child.css";
.main { color: red; }`

	result := ProcessImports(mainCSS, tmpDir, nil)

	assert.Contains(t, result, "/* imported: _child.css */")
	assert.Contains(t, result, "/* imported: _grandchild.css */")
	assert.Contains(t, result, ".grandchild")
	assert.Contains(t, result, ".child")
	assert.Contains(t, result, ".main")
}

func TestProcessImports_CircularPrevention(t *testing.T) {
	tmpDir := t.TempDir()

	// Create circular imports: a imports b, b imports a
	aContent := `@import "_b.css";
.a { color: red; }`
	aPath := filepath.Join(tmpDir, "_a.css")
	err := os.WriteFile(aPath, []byte(aContent), 0644)
	require.NoError(t, err)

	bContent := `@import "_a.css";
.b { color: blue; }`
	bPath := filepath.Join(tmpDir, "_b.css")
	err = os.WriteFile(bPath, []byte(bContent), 0644)
	require.NoError(t, err)

	// Start with a
	result := ProcessImports(`@import "_a.css";`, tmpDir, nil)

	// Should have both imports but one marked as circular
	assert.Contains(t, result, "/* imported: _a.css */")
	assert.Contains(t, result, "/* imported: _b.css */")
	assert.Contains(t, result, "/* circular import prevented: _a.css */")
}

func TestProcessImports_MissingFile(t *testing.T) {
	css := `@import "nonexistent.css";`

	result := ProcessImports(css, "/tmp", nil)

	assert.Contains(t, result, "/* import failed: nonexistent.css")
}

func TestProcessImports_FallbackToEmbeddedTheme(t *testing.T) {
	// When importing a non-existent file, it should try embedded themes
	css := `@import "default.css";`

	result := ProcessImports(css, "/nonexistent/path", nil)

	// Should fallback to embedded default theme
	assert.Contains(t, result, "/* imported (embedded): default.css */")
	assert.Contains(t, result, ".notification-popup")
}

func TestImportRegex(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`@import "file.css";`, "file.css"},
		{`@import 'file.css';`, "file.css"},
		{`@import url("file.css");`, "file.css"},
		{`@import url('file.css');`, "file.css"},
		{`@import url( "file.css" );`, "file.css"},
		{`@import "_partial.css"`, "_partial.css"}, // Without semicolon
		{`@import   "spaced.css"  ;`, "spaced.css"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			matches := importRegex.FindStringSubmatch(tt.input)
			require.Len(t, matches, 2, "should match import statement")
			assert.Equal(t, tt.expected, matches[1])
		})
	}
}

func TestNewTheme_ProcessesImports(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a partial
	partialContent := `:root { --custom: #ff0000; }`
	partialPath := filepath.Join(tmpDir, "_colors.css")
	err := os.WriteFile(partialPath, []byte(partialContent), 0644)
	require.NoError(t, err)

	// Create main theme that imports the partial
	themeContent := `@import "_colors.css";
.notification-popup { color: var(--custom); }`
	themePath := filepath.Join(tmpDir, "custom.css")
	err = os.WriteFile(themePath, []byte(themeContent), 0644)
	require.NoError(t, err)

	theme, err := NewTheme("custom", themePath)
	require.NoError(t, err)

	// CSS should have processed imports
	assert.Contains(t, theme.CSS, "/* imported: _colors.css */")
	assert.Contains(t, theme.CSS, "--custom: #ff0000")
	assert.Contains(t, theme.CSS, ".notification-popup")
}

func TestTheme_Reload_ProcessesImports(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial theme
	themeContent := `.notification-popup { color: red; }`
	themePath := filepath.Join(tmpDir, "test.css")
	err := os.WriteFile(themePath, []byte(themeContent), 0644)
	require.NoError(t, err)

	theme, err := NewTheme("test", themePath)
	require.NoError(t, err)
	assert.Contains(t, theme.CSS, "color: red")

	// Create a partial
	partialContent := `:root { --new-color: blue; }`
	partialPath := filepath.Join(tmpDir, "_new.css")
	err = os.WriteFile(partialPath, []byte(partialContent), 0644)
	require.NoError(t, err)

	// Update theme to import the partial
	newContent := `@import "_new.css";
.notification-popup { color: var(--new-color); }`
	err = os.WriteFile(themePath, []byte(newContent), 0644)
	require.NoError(t, err)

	// Reload should process imports
	changed, err := theme.Reload()
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Contains(t, theme.CSS, "/* imported: _new.css */")
	assert.Contains(t, theme.CSS, "--new-color: blue")
}
