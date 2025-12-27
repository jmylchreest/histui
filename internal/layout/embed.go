package layout

import (
	"embed"
	"strings"
)

//go:embed templates/*.xml
var EmbeddedTemplates embed.FS

// GetEmbeddedTemplate returns an embedded template by name.
// The name should not include the .xml extension.
func GetEmbeddedTemplate(name string) (*LayoutConfig, bool) {
	path := "templates/" + name + ".xml"
	data, err := EmbeddedTemplates.ReadFile(path)
	if err != nil {
		return nil, false
	}

	config, err := ParseTemplateString(string(data))
	if err != nil {
		return nil, false
	}

	return config, true
}

// ListEmbeddedTemplates returns the names of all embedded templates.
func ListEmbeddedTemplates() []string {
	entries, err := EmbeddedTemplates.ReadDir("templates")
	if err != nil {
		return nil
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".xml") {
			name := strings.TrimSuffix(entry.Name(), ".xml")
			names = append(names, name)
		}
	}
	return names
}
