package layout

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Template represents a parsed notification layout template.
type Template struct {
	Popup PopupElement `xml:"popup"`
}

// PopupElement represents the root popup container.
type PopupElement struct {
	XMLName  xml.Name   `xml:"popup"`
	Attrs    []xml.Attr `xml:",any,attr"`
	Children []Element  `xml:",any"`
}

// Element represents a layout element (generic, parsed by name).
type Element struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Children []Element  `xml:",any"`
	Content  string     `xml:",chardata"`
}

// ElementType identifies the type of layout element.
type ElementType string

const (
	ElementTypeHeader     ElementType = "header"
	ElementTypeBody       ElementType = "body"
	ElementTypeActions    ElementType = "actions"
	ElementTypeProgress   ElementType = "progress"
	ElementTypeIcon       ElementType = "icon"
	ElementTypeSummary    ElementType = "summary"
	ElementTypeAppName    ElementType = "appname"
	ElementTypeTimestamp  ElementType = "timestamp"
	ElementTypeStackCount ElementType = "stack-count"
	ElementTypeClose      ElementType = "close"
	ElementTypeImage      ElementType = "image"
	ElementTypeBox        ElementType = "box"
)

// ValidElements lists all recognized element types.
var ValidElements = map[string]ElementType{
	"header":      ElementTypeHeader,
	"body":        ElementTypeBody,
	"actions":     ElementTypeActions,
	"progress":    ElementTypeProgress,
	"icon":        ElementTypeIcon,
	"summary":     ElementTypeSummary,
	"appname":     ElementTypeAppName,
	"timestamp":   ElementTypeTimestamp,
	"stack-count": ElementTypeStackCount,
	"close":       ElementTypeClose,
	"image":       ElementTypeImage,
	"box":         ElementTypeBox,
}

// LayoutConfig represents the parsed layout structure ready for UI building.
type LayoutConfig struct {
	// Popup sizing (0 = use config default)
	// Set min=max for fixed size, or use range for flexible content sizing.
	MinWidth  int
	MaxWidth  int
	MinHeight int
	MaxHeight int
	// Child elements
	Elements []LayoutElement
}

// LayoutElement represents a single element in the layout.
type LayoutElement struct {
	Type       ElementType
	Attributes map[string]string
	Children   []LayoutElement
}

// ParseTemplate parses an XML layout template from a reader.
func ParseTemplate(r io.Reader) (*LayoutConfig, error) {
	decoder := xml.NewDecoder(r)

	// Find the root <popup> element
	var config LayoutConfig
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read template: %w", err)
		}

		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Local == "popup" {
				// Parse popup attributes for sizing
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "min-width":
						if v, err := parsePixelValue(attr.Value); err == nil {
							config.MinWidth = v
						}
					case "max-width":
						if v, err := parsePixelValue(attr.Value); err == nil {
							config.MaxWidth = v
						}
					case "min-height":
						if v, err := parsePixelValue(attr.Value); err == nil {
							config.MinHeight = v
						}
					case "max-height":
						if v, err := parsePixelValue(attr.Value); err == nil {
							config.MaxHeight = v
						}
					}
				}

				// Parse children of popup
				elements, err := parseElements(decoder)
				if err != nil {
					return nil, err
				}
				config.Elements = elements
				break
			}
		}
	}

	return &config, nil
}

// parsePixelValue parses a pixel value string (e.g., "300", "300px") to int.
func parsePixelValue(s string) (int, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "px")
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

// parseElements recursively parses child elements.
func parseElements(decoder *xml.Decoder) ([]LayoutElement, error) {
	var elements []LayoutElement

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read element: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			elemName := strings.ToLower(t.Name.Local)
			elemType, ok := ValidElements[elemName]
			if !ok {
				return nil, fmt.Errorf("unknown element type: %s", elemName)
			}

			elem := LayoutElement{
				Type:       elemType,
				Attributes: make(map[string]string),
			}

			// Parse attributes
			for _, attr := range t.Attr {
				elem.Attributes[attr.Name.Local] = attr.Value
			}

			// Parse children
			children, err := parseElements(decoder)
			if err != nil {
				return nil, err
			}
			elem.Children = children

			elements = append(elements, elem)

		case xml.EndElement:
			// End of parent element
			return elements, nil
		}
	}

	return elements, nil
}

// ParseTemplateString parses a template from a string.
func ParseTemplateString(s string) (*LayoutConfig, error) {
	return ParseTemplate(strings.NewReader(s))
}

// LoadTemplate loads a template from file.
func LoadTemplate(path string) (*LayoutConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open template: %w", err)
	}
	defer func() { _ = f.Close() }()
	return ParseTemplate(f)
}

// Loader handles loading layout templates from various sources.
type Loader struct {
	templatesDir string
}

// NewLoader creates a new template loader.
func NewLoader(templatesDir string) *Loader {
	return &Loader{templatesDir: templatesDir}
}

// Load loads a layout template by name.
// Checks user directory first, then falls back to embedded default.
func (l *Loader) Load(name string) (*LayoutConfig, error) {
	// Check user directory first
	if l.templatesDir != "" {
		templatePath := filepath.Join(l.templatesDir, name+".xml")
		if _, err := os.Stat(templatePath); err == nil {
			return LoadTemplate(templatePath)
		}
	}

	// Fall back to embedded default
	if name == "default" || name == "" {
		return DefaultLayout(), nil
	}

	return nil, fmt.Errorf("layout template not found: %s", name)
}

// DefaultLayout returns the default notification layout.
func DefaultLayout() *LayoutConfig {
	return &LayoutConfig{
		MinWidth:  300,
		MaxWidth:  300, // Fixed width
		MinHeight: 0,   // Dynamic height
		MaxHeight: 900,
		Elements: []LayoutElement{
			{
				Type: ElementTypeHeader,
				Children: []LayoutElement{
					{Type: ElementTypeIcon},
					{
						Type: ElementTypeBox,
						Attributes: map[string]string{
							"orientation": "vertical",
						},
						Children: []LayoutElement{
							{Type: ElementTypeSummary},
							{Type: ElementTypeAppName},
						},
					},
					{Type: ElementTypeStackCount},
					{Type: ElementTypeClose},
				},
			},
			{Type: ElementTypeBody},
			{Type: ElementTypeProgress},
			{Type: ElementTypeImage},
			{Type: ElementTypeActions},
		},
	}
}

// CompactLayout returns a minimal layout without app name or image.
func CompactLayout() *LayoutConfig {
	return &LayoutConfig{
		MinWidth:  300,
		MaxWidth:  300, // Fixed width
		MinHeight: 0,   // Dynamic height
		MaxHeight: 900,
		Elements: []LayoutElement{
			{
				Type: ElementTypeHeader,
				Children: []LayoutElement{
					{Type: ElementTypeIcon},
					{Type: ElementTypeSummary},
					{Type: ElementTypeStackCount},
					{Type: ElementTypeClose},
				},
			},
			{Type: ElementTypeBody},
			{Type: ElementTypeProgress},
			{Type: ElementTypeActions},
		},
	}
}

// DefaultTemplateXML returns the default template as XML string.
func DefaultTemplateXML() string {
	return `<popup>
  <header>
    <icon />
    <box orientation="vertical">
      <summary />
      <appname />
    </box>
    <stack-count />
    <close />
  </header>
  <body />
  <progress />
  <image />
  <actions />
</popup>`
}
