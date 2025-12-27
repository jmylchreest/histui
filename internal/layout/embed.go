package layout

import (
	"github.com/jmylchreest/histui/internal/theme"
)

// GetEmbeddedTemplate returns an embedded template by name.
// Templates are stored in theme directories as layout.xml.
// The name should match the theme name (e.g., "default", "minimal").
func GetEmbeddedTemplate(name string) (*LayoutConfig, bool) {
	layoutXML, found := theme.GetEmbeddedLayout(name)
	if !found {
		return nil, false
	}

	config, err := ParseTemplateString(layoutXML)
	if err != nil {
		return nil, false
	}

	return config, true
}

// ListEmbeddedTemplates returns the names of all embedded templates.
func ListEmbeddedTemplates() []string {
	return theme.ListEmbeddedLayouts()
}
