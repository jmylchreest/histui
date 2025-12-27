package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTemplateString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		checkLayout func(t *testing.T, config *LayoutConfig)
	}{
		{
			name: "simple popup with header and body",
			input: `<popup>
				<header>
					<icon />
					<summary />
				</header>
				<body />
			</popup>`,
			wantErr: false,
			checkLayout: func(t *testing.T, config *LayoutConfig) {
				require.Len(t, config.Elements, 2)
				assert.Equal(t, ElementTypeHeader, config.Elements[0].Type)
				assert.Equal(t, ElementTypeBody, config.Elements[1].Type)

				// Check header children
				header := config.Elements[0]
				require.Len(t, header.Children, 2)
				assert.Equal(t, ElementTypeIcon, header.Children[0].Type)
				assert.Equal(t, ElementTypeSummary, header.Children[1].Type)
			},
		},
		{
			name: "box with orientation attribute",
			input: `<popup>
				<box orientation="vertical">
					<summary />
					<appname />
				</box>
			</popup>`,
			wantErr: false,
			checkLayout: func(t *testing.T, config *LayoutConfig) {
				require.Len(t, config.Elements, 1)
				box := config.Elements[0]
				assert.Equal(t, ElementTypeBox, box.Type)
				assert.Equal(t, "vertical", box.Attributes["orientation"])
				require.Len(t, box.Children, 2)
			},
		},
		{
			name: "unknown element",
			input: `<popup>
				<unknown-element />
			</popup>`,
			wantErr: true,
		},
		{
			name: "empty popup",
			input: `<popup></popup>`,
			wantErr: false,
			checkLayout: func(t *testing.T, config *LayoutConfig) {
				assert.Empty(t, config.Elements)
			},
		},
		{
			name: "all element types",
			input: `<popup>
				<header />
				<body />
				<actions />
				<progress />
				<icon />
				<summary />
				<appname />
				<timestamp />
				<stack-count />
				<close />
				<image />
				<box />
			</popup>`,
			wantErr: false,
			checkLayout: func(t *testing.T, config *LayoutConfig) {
				assert.Len(t, config.Elements, 12)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseTemplateString(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, config)
			if tt.checkLayout != nil {
				tt.checkLayout(t, config)
			}
		})
	}
}

func TestDefaultLayout(t *testing.T) {
	layout := DefaultLayout()
	require.NotNil(t, layout)
	require.NotEmpty(t, layout.Elements)

	// Default layout should have header as first element
	assert.Equal(t, ElementTypeHeader, layout.Elements[0].Type)

	// Header should have icon, box (with summary/appname), stack-count, close
	header := layout.Elements[0]
	require.GreaterOrEqual(t, len(header.Children), 4)
	assert.Equal(t, ElementTypeIcon, header.Children[0].Type)
}

func TestGetEmbeddedTemplate(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		wantFound bool
	}{
		{"default", "default", true},
		{"compact", "compact", true},
		{"minimal", "minimal", true},
		{"detailed", "detailed", true},
		{"nonexistent", "nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, found := GetEmbeddedTemplate(tt.template)
			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.NotNil(t, config)
				// All templates should have at least one element
				assert.NotEmpty(t, config.Elements)
			}
		})
	}
}

func TestListEmbeddedTemplates(t *testing.T) {
	templates := ListEmbeddedTemplates()
	assert.Contains(t, templates, "default")
	assert.Contains(t, templates, "compact")
	assert.Contains(t, templates, "minimal")
	assert.Contains(t, templates, "detailed")
}

func TestLoader(t *testing.T) {
	loader := NewLoader("")

	// Should load embedded default
	config, err := loader.Load("default")
	require.NoError(t, err)
	assert.NotNil(t, config)

	// Should error for unknown
	_, err = loader.Load("unknown")
	assert.Error(t, err)

	// Empty name should load default
	config, err = loader.Load("")
	require.NoError(t, err)
	assert.NotNil(t, config)
}
