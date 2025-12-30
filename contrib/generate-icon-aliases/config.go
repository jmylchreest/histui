package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const configFile = "config.toml"
const kbCategoriesFile = "kb-categories.toml"

// Config represents the generator configuration.
type Config struct {
	// OpenRouter settings
	OpenRouter OpenRouterConfig `toml:"openrouter"`

	// Upstream metadata sources
	Upstream UpstreamConfig `toml:"upstream"`

	// Prompts for AI generation (supports {{.Year}} template)
	Prompts PromptConfig `toml:"prompts"`

	// Icon preferences for category suggestions
	IconPreferences IconPreferencesConfig `toml:"icon_preferences"`
}

// IconPreferencesConfig contains preferences for icon selection.
type IconPreferencesConfig struct {
	// PreferredSets is the order of preference for icon sets (first = highest priority)
	// Valid values: "md" (Material Design), "fa" (Font Awesome), "cod" (Codicons), "dev" (Devicons)
	PreferredSets []string `toml:"preferred_sets"`

	// PreferredStyles is the order of preference for icon styles
	// Valid values: "outline", "solid", "filled", "regular"
	PreferredStyles []string `toml:"preferred_styles"`

	// AvoidPatterns are glyph name patterns to deprioritize (e.g., "circle", "square")
	AvoidPatterns []string `toml:"avoid_patterns"`

	// MaxSuggestionsPerCategory is the number of suggestions to generate per category
	MaxSuggestionsPerCategory int `toml:"max_suggestions_per_category"`
}

// OpenRouterConfig contains OpenRouter API settings.
type OpenRouterConfig struct {
	// DefaultModel is the model to use when not specified via flag/env
	DefaultModel string `toml:"default_model"`

	// WebSearch enables real-time web search
	WebSearch bool `toml:"web_search"`

	// AppGenBatchSize is the number of icons to generate apps for per API call
	AppGenBatchSize int `toml:"app_gen_batch_size"`

	// CategoryAssignBatchSize is the number of apps to assign categories for per API call
	CategoryAssignBatchSize int `toml:"category_assign_batch_size"`

	// CategoryFallbackThreshold is the confidence threshold below which apps get assigned to categories
	// Apps with confidence scores below this threshold will be reassigned from brand icons to category fallbacks
	CategoryFallbackThreshold float64 `toml:"category_fallback_threshold"`

	// CategoryMinConfidence is the minimum confidence for an app to be assigned to a category
	// Apps below this threshold are filtered out entirely (likely AI errors)
	CategoryMinConfidence float64 `toml:"category_min_confidence"`

	// RequestTimeout is the timeout for each API request in seconds
	RequestTimeout int `toml:"request_timeout"`

	// MaxTokens is the maximum number of tokens to generate per response
	MaxTokens int `toml:"max_tokens"`
}

// UpstreamConfig contains URLs for upstream icon metadata sources.
type UpstreamConfig struct {
	// FontAwesome metadata URL
	FontAwesome string `toml:"font_awesome"`

	// MaterialDesign metadata URL
	MaterialDesign string `toml:"material_design"`

	// Devicons metadata URL
	Devicons string `toml:"devicons"`

	// Codicons metadata URL
	Codicons string `toml:"codicons"`

	// AdwaitaIcons GitHub API URL for fetching GTK symbolic icons
	AdwaitaIcons string `toml:"adwaita_icons"`
}

// PromptConfig contains customizable AI prompts.
type PromptConfig struct {
	// AppGenPrompt is the prompt template for app name generation
	AppGenPrompt string `toml:"app_gen_prompt"`

	// CategorySuggestPrompt is the prompt template for category icon suggestions
	CategorySuggestPrompt string `toml:"category_suggest_prompt"`

	// CategoryAssignPrompt is the prompt template for assigning apps to categories
	CategoryAssignPrompt string `toml:"category_assign_prompt"`
}

// PromptVars contains variables for prompt templates.
type PromptVars struct {
	Year       int
	Icons      string // newline-separated list of icons
	Categories string // formatted list of fallback categories
}

// CategorySuggestVars contains variables for category suggestion prompts.
type CategorySuggestVars struct {
	Categories        string // formatted list of categories needing icons
	AvailableGlyphs   string // formatted list of available glyphs with metadata
	AvailableGtkIcons string // formatted list of available GTK symbolic icons
	Preferences       string // formatted icon preferences
	MaxPerCategory    int    // max suggestions per category
}

// CategoryAssignVars contains variables for category assignment prompts.
type CategoryAssignVars struct {
	Categories string // formatted list of available categories
	Apps       string // formatted list of apps to assign
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		OpenRouter: OpenRouterConfig{
			DefaultModel:              "google/gemini-2.5-flash",
			WebSearch:                 true,
			AppGenBatchSize:           50,
			CategoryAssignBatchSize:   100,
			CategoryFallbackThreshold: 0.7,
			CategoryMinConfidence:     0.3,
			RequestTimeout:            600,
			MaxTokens:                 32000,
		},
		Upstream: UpstreamConfig{
			FontAwesome:    "https://raw.githubusercontent.com/FortAwesome/Font-Awesome/7.x/metadata/icons.json",
			MaterialDesign: "https://raw.githubusercontent.com/Templarian/MaterialDesign-Meta/master/meta.json",
			Devicons:       "https://raw.githubusercontent.com/devicons/devicon/master/devicon.json",
			Codicons:       "https://raw.githubusercontent.com/microsoft/vscode-codicons/main/src/template/mapping.json",
			AdwaitaIcons:   "https://api.github.com/repos/GNOME/adwaita-icon-theme/git/trees/master?recursive=1",
		},
		Prompts: PromptConfig{
			AppGenPrompt:          defaultAppGenPrompt,
			CategorySuggestPrompt: defaultCategorySuggestPrompt,
			CategoryAssignPrompt:  defaultCategoryAssignPrompt,
		},
		IconPreferences: IconPreferencesConfig{
			PreferredSets:             []string{"md", "fa", "cod", "dev"},
			PreferredStyles:           []string{"outline", "regular", "solid", "filled"},
			AvoidPatterns:             []string{"circle", "square", "box", "numeric"},
			MaxSuggestionsPerCategory: 5,
		},
	}
}

const defaultAppGenPrompt = `You are generating Linux application identifiers for icon mappings in a desktop notification system.
Current year: {{.Year}} - include current and actively maintained apps in the Linux ecosystem.

For each icon, list ALL Linux applications that would use this icon. Be comprehensive.

Include these identifier types:
- Package names (apt/pacman/dnf): discord, firefox, thunderbird
- Flatpak IDs: com.discordapp.Discord, org.mozilla.firefox
- Snap names: discord, firefox
- Desktop file names: org.mozilla.firefox, com.discordapp.Discord
- Binary names: discord, firefox-esr
- Common forks/variants: librewolf, waterfox, vesktop, armcord

For "app" type icons (brand logos like Discord, Spotify):
- List the primary app and all known variants/forks
- Include official variants (discord-canary, spotify-client)
- Include popular third-party clients (vesktop for Discord)

For "category" type icons (generic like email, music, video):
- List the most popular Linux applications in that category
- Include both mainstream (thunderbird) and alternatives (evolution, geary)
- Include newer/modern alternatives (Ghostty for terminal, Zed for code)

IMPORTANT: When an app does not have a brand-specific icon (like Signal, which has no Nerd Font glyph),
assign it to an appropriate category icon instead. Use these curated categories:

{{.Categories}}

For apps without brand icons, add them to the appropriate category's app list with:
- confidence: 0.5 (category fallback)
- source: "category"

Confidence scoring:
- 1.0: Primary/official app (discord for discord icon)
- 0.9: Official variants (discord-canary, firefox-esr)
- 0.8: Well-known Flatpak/Snap IDs
- 0.7: Popular forks (librewolf, vesktop)
- 0.6: Less common alternatives
- 0.5: Category fallback (app has no brand icon)

Icons to map (format: "name (type) - description"):
{{.Icons}}

Respond with valid JSON only:
{
  "mappings": [
    {
      "icon": "discord",
      "apps": [
        {"id": "discord", "confidence": 1.0, "source": "package"},
        {"id": "com.discordapp.Discord", "confidence": 0.9, "source": "flatpak"},
        {"id": "discord-canary", "confidence": 0.9, "source": "package"},
        {"id": "vesktop", "confidence": 0.7, "source": "package"}
      ]
    }
  ]
}`

const defaultCategorySuggestPrompt = `You are selecting icons for application category fallbacks in a Linux desktop notification system.

For each category, you need to suggest:
1. A Nerd Font glyph (for terminal/TUI display)
2. A GTK icon (for GUI display in GTK-based desktops like GNOME)

IMPORTANT RULES:
1. Use ONLY the exact category names listed below - do NOT add suffixes like "-fallback"
2. Use ONLY glyph names from the "Available glyphs" list below - do NOT invent glyph names
3. Glyph names follow the format "prefix-name" where prefix is md, fa, cod, or dev
4. Use ONLY GTK icon names from the "Available GTK icons" list below
5. GTK icons should end with "-symbolic" (e.g., "mail-send-symbolic")

Categories (use these EXACT names as keys in your response):
{{.Categories}}

Icon preferences (apply these when ranking Nerd Font glyphs):
{{.Preferences}}

Available glyphs (ONLY use names from this list, format: "prefix-name: description"):
{{.AvailableGlyphs}}

Available GTK icons (ONLY use names from this list, format: "name-symbolic (category)"):
{{.AvailableGtkIcons}}

For each category, provide up to {{.MaxPerCategory}} icon suggestions ranked by:
1. Semantic match (icon meaning matches category purpose)
2. Visual clarity (recognizable at small sizes)
3. Preference alignment (preferred sets/styles ranked higher)
4. Universality (widely understood symbols preferred)

Respond with valid JSON. Use EXACT category names as keys (e.g., "messaging" not "messaging-fallback"):
{
  "suggestions": {
    "messaging": [
      {
        "glyph": "md-chat",
        "gtk_icon": "chat-message-new-symbolic",
        "confidence": 0.95,
        "reason": "Direct semantic match for chat/messaging, clean outline style"
      }
    ],
    "email": [
      {
        "glyph": "md-email",
        "gtk_icon": "mail-send-symbolic",
        "confidence": 0.98,
        "reason": "Classic envelope icon, perfect semantic match"
      }
    ]
  }
}`

const defaultCategoryAssignPrompt = `You are assigning Linux applications to fallback icon categories.

These applications do not have brand-specific icons in Nerd Fonts, so they need to be assigned
to appropriate category fallbacks for visual representation in a desktop notification system.

Available categories (assign apps to these EXACT category names):
{{.Categories}}

Applications to categorize (format: "app_id (current_icon, confidence)"):
{{.Apps}}

For each application, determine the single most appropriate category based on:
1. Application purpose (what does it do?)
2. Category description match (read the category descriptions)
3. Similar apps in the category examples

Confidence scoring:
- 0.9+: Perfect match (app clearly belongs to this category)
- 0.8: Strong match (category is clearly appropriate)
- 0.7: Good match (reasonable category assignment)
- 0.6: Acceptable match (app could fit multiple categories)
- 0.5: Weak match (category is a fallback, not ideal)

Respond with valid JSON only:
{
  "assignments": [
    {
      "app": "evolution",
      "category": "email",
      "confidence": 0.95,
      "reason": "Email client, direct match to email category"
    },
    {
      "app": "geary",
      "category": "email",
      "confidence": 0.90,
      "reason": "Modern email client"
    }
  ]
}`

// RenderPrompt renders a prompt template with the given variables.
func RenderPrompt(tmpl string, vars PromptVars) (string, error) {
	t, err := template.New("prompt").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// CategoriesConfig represents the structure of kb-categories.toml.
type CategoriesConfig struct {
	Meta       CategoriesMeta               `toml:"meta"`
	Categories map[string]CategoryDefinition `toml:"categories"`
}

// CategoriesMeta contains metadata about the categories file.
type CategoriesMeta struct {
	Description string `toml:"description"`
	Version     int    `toml:"version"`
}

// CategoryDefinition defines a single category.
type CategoryDefinition struct {
	Description string   `toml:"description"`
	Examples    []string `toml:"examples"`
	Symbol      string   `toml:"symbol,omitempty"`   // Nerd Font symbol character
	Glyph       string   `toml:"glyph,omitempty"`    // Glyph name for documentation (e.g., "md-chat")
	GtkIcon     string   `toml:"gtk_icon,omitempty"` // GTK icon name
}

// LoadCategories loads the curated categories from kb-categories.toml.
func LoadCategories(path string) (*CategoriesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read categories: %w", err)
	}

	var config CategoriesConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse categories: %w", err)
	}

	return &config, nil
}

// SaveCategories saves the categories config to a TOML file.
func SaveCategories(config *CategoriesConfig, path string) error {
	var buf strings.Builder

	buf.WriteString("# App Categories for histui\n")
	buf.WriteString("# =========================\n")
	buf.WriteString("#\n")
	buf.WriteString("# When an app doesn't have a brand-specific icon in Nerd Fonts,\n")
	buf.WriteString("# the AI should assign it to one of these categories instead.\n")
	buf.WriteString("#\n")
	buf.WriteString("# Run `task generate:icons:suggest` to get AI icon suggestions,\n")
	buf.WriteString("# then `task generate:icons:apply-suggestions` to populate glyph fields.\n")
	buf.WriteString("\n")
	buf.WriteString("[meta]\n")
	buf.WriteString(fmt.Sprintf("description = %q\n", config.Meta.Description))
	buf.WriteString(fmt.Sprintf("version = %d\n", config.Meta.Version))
	buf.WriteString("\n")
	buf.WriteString("[categories]\n")
	buf.WriteString("\n")

	// Sort category names for consistent output
	var catNames []string
	for name := range config.Categories {
		catNames = append(catNames, name)
	}
	sortStringSlice(catNames)

	// Group categories by section for readability
	sections := map[string][]string{
		"# Communication":    {"messaging", "email", "social"},
		"# Media":            {"video", "audio", "image"},
		"# Productivity":     {"text-editor", "notes", "calendar", "office"},
		"# System":           {"terminal", "file-manager", "settings", "system-monitor", "archive"},
		"# Security & Network": {"firewall", "vpn", "password", "backup"},
		"# Network":          {"download", "torrent", "browser"},
		"# Development":      {"code", "git", "database"},
		"# Other":            {"game", "screenshot", "calculator"},
	}
	sectionOrder := []string{
		"# Communication", "# Media", "# Productivity", "# System",
		"# Security & Network", "# Network", "# Development", "# Other",
	}

	written := make(map[string]bool)

	for _, section := range sectionOrder {
		cats := sections[section]
		hasAny := false
		for _, catName := range cats {
			if _, ok := config.Categories[catName]; ok {
				hasAny = true
				break
			}
		}
		if !hasAny {
			continue
		}

		buf.WriteString(section + "\n")
		for _, catName := range cats {
			cat, ok := config.Categories[catName]
			if !ok {
				continue
			}
			written[catName] = true
			writeCategoryEntry(&buf, catName, cat)
		}
	}

	// Write any categories not in predefined sections
	for _, catName := range catNames {
		if written[catName] {
			continue
		}
		writeCategoryEntry(&buf, catName, config.Categories[catName])
	}

	return os.WriteFile(path, []byte(buf.String()), 0644)
}

// writeCategoryEntry writes a single category entry to the buffer.
func writeCategoryEntry(buf *strings.Builder, catName string, cat CategoryDefinition) {
	buf.WriteString(fmt.Sprintf("[categories.%s]\n", catName))
	buf.WriteString(fmt.Sprintf("description = %q\n", cat.Description))
	buf.WriteString(fmt.Sprintf("examples = %s\n", formatStringSlice(cat.Examples)))

	// Write symbol with glyph name as comment (using nf- namespace prefix)
	// Use literal string format to preserve the actual unicode character
	if cat.Symbol != "" && cat.Glyph != "" {
		// Convert glyph name to nf- namespace format (e.g., "md-chat" -> "nf-md-chat")
		glyphComment := cat.Glyph
		if !strings.HasPrefix(glyphComment, "nf-") {
			glyphComment = "nf-" + glyphComment
		}
		buf.WriteString(fmt.Sprintf("symbol = \"%s\"  # %s\n", cat.Symbol, glyphComment))
	} else if cat.Symbol != "" {
		buf.WriteString(fmt.Sprintf("symbol = \"%s\"\n", cat.Symbol))
	} else {
		buf.WriteString("symbol = \"\"\n")
	}

	buf.WriteString(fmt.Sprintf("gtk_icon = %q\n", cat.GtkIcon))
	buf.WriteString("\n")
}

// sortStringSlice sorts a slice of strings in place.
func sortStringSlice(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// formatStringSlice formats a string slice as a TOML array.
func formatStringSlice(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	quoted := make([]string, len(s))
	for i, v := range s {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// CategorySuggestionsFile represents the structure of kb-category-suggestions.toml.
type CategorySuggestionsFile struct {
	Suggestions map[string]CategorySuggestionEntry `toml:"suggestions"`
}

// CategorySuggestionEntry represents a single category's suggestions.
type CategorySuggestionEntry struct {
	Glyph      string  `toml:"glyph"`
	GtkIcon    string  `toml:"gtk_icon,omitempty"`
	Symbol     string  `toml:"symbol,omitempty"`
	Confidence float64 `toml:"confidence"`
	Reason     string  `toml:"reason"`
}

// LoadCategorySuggestions loads category suggestions from a TOML file.
func LoadCategorySuggestions(path string) (map[string][]CategorySuggestion, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read suggestions: %w", err)
	}

	var file CategorySuggestionsFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse suggestions: %w", err)
	}

	// Convert to the format expected by the caller
	result := make(map[string][]CategorySuggestion)
	for catName, entry := range file.Suggestions {
		result[catName] = []CategorySuggestion{{
			Glyph:      entry.Glyph,
			GtkIcon:    entry.GtkIcon,
			Confidence: entry.Confidence,
			Reason:     entry.Reason,
		}}
	}

	return result, nil
}

// CategorySuggestion represents a single icon suggestion for a category.
// This is defined here to avoid import cycles with openrouter.go.
type CategorySuggestion struct {
	Glyph      string  `json:"glyph"`
	GtkIcon    string  `json:"gtk_icon,omitempty"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// CategoryAssignmentsFile represents the kb-category-assignments.json file.
type CategoryAssignmentsFile struct {
	Version     int                                `json:"version"`
	GeneratedAt string                             `json:"generated_at"`
	Threshold   float64                            `json:"threshold"`
	Assignments map[string]CategoryAssignmentEntry `json:"assignments"`
}

// CategoryAssignmentEntry represents a single app-to-category assignment.
type CategoryAssignmentEntry struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason,omitempty"`
}

// LoadCategoryAssignments loads category assignments from a JSON file.
func LoadCategoryAssignments(path string) (*CategoryAssignmentsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read assignments: %w", err)
	}

	var file CategoryAssignmentsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse assignments: %w", err)
	}

	return &file, nil
}

// SaveCategoryAssignments saves category assignments to a JSON file.
func SaveCategoryAssignments(file *CategoryAssignmentsFile, path string) error {
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal assignments: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write assignments: %w", err)
	}

	return nil
}

// FormatCategoriesForPrompt formats categories for inclusion in the AI prompt.
func FormatCategoriesForPrompt(cats *CategoriesConfig) string {
	if cats == nil || len(cats.Categories) == 0 {
		return "(No categories defined)"
	}

	var lines []string
	for name, cat := range cats.Categories {
		examples := strings.Join(cat.Examples, ", ")
		lines = append(lines, fmt.Sprintf("- %s: %s (e.g., %s)", name, cat.Description, examples))
	}
	// Sort for consistent output
	var sortedLines []string
	sortedLines = append(sortedLines, lines...)
	// Note: Go maps iterate in random order, so we sort the lines
	// We'll use a simple sort since these are just for display
	for i := 0; i < len(sortedLines)-1; i++ {
		for j := i + 1; j < len(sortedLines); j++ {
			if sortedLines[i] > sortedLines[j] {
				sortedLines[i], sortedLines[j] = sortedLines[j], sortedLines[i]
			}
		}
	}
	return strings.Join(sortedLines, "\n")
}

// RenderAppGenPrompt renders the app generation prompt with icons and categories.
func (c *Config) RenderAppGenPrompt(iconList []string) (string, error) {
	// Try to load categories from the same directory as config
	categoriesText := "(Categories file not found)"
	cats, err := LoadCategories(kbCategoriesFile)
	if err == nil {
		categoriesText = FormatCategoriesForPrompt(cats)
	}

	vars := PromptVars{
		Year:       time.Now().Year(),
		Icons:      strings.Join(iconList, "\n"),
		Categories: categoriesText,
	}
	return RenderPrompt(c.Prompts.AppGenPrompt, vars)
}

// RenderCategoryAssignPrompt renders the category assignment prompt.
func (c *Config) RenderCategoryAssignPrompt(categories *CategoriesConfig, apps []string) (string, error) {
	categoriesText := FormatCategoriesForPrompt(categories)
	appsText := strings.Join(apps, "\n")

	vars := CategoryAssignVars{
		Categories: categoriesText,
		Apps:       appsText,
	}

	t, err := template.New("category_assign").Parse(c.Prompts.CategoryAssignPrompt)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// FormatPreferencesForPrompt formats icon preferences for inclusion in the AI prompt.
func (c *Config) FormatPreferencesForPrompt() string {
	var lines []string

	if len(c.IconPreferences.PreferredSets) > 0 {
		lines = append(lines, fmt.Sprintf("- Preferred icon sets (in order): %s",
			strings.Join(c.IconPreferences.PreferredSets, " > ")))
	}

	if len(c.IconPreferences.PreferredStyles) > 0 {
		lines = append(lines, fmt.Sprintf("- Preferred styles (in order): %s",
			strings.Join(c.IconPreferences.PreferredStyles, " > ")))
	}

	if len(c.IconPreferences.AvoidPatterns) > 0 {
		lines = append(lines, fmt.Sprintf("- Deprioritize icons containing: %s",
			strings.Join(c.IconPreferences.AvoidPatterns, ", ")))
	}

	if len(lines) == 0 {
		return "(No preferences specified)"
	}

	return strings.Join(lines, "\n")
}

// GlyphMetadata represents a glyph with its metadata for the suggestion prompt.
type GlyphMetadata struct {
	Name        string
	Prefix      string   // md, fa, cod, dev
	Symbol      string   // the actual unicode character
	Description string
	Tags        []string
}

// FormatGlyphsForPrompt formats glyphs for inclusion in the category suggestion prompt.
// It filters to only include "category" type glyphs (not brand-specific app icons).
func FormatGlyphsForPrompt(glyphs []GlyphMetadata, limit int) string {
	var lines []string
	for i, g := range glyphs {
		if limit > 0 && i >= limit {
			break
		}
		tags := ""
		if len(g.Tags) > 0 {
			tags = fmt.Sprintf(" [%s]", strings.Join(g.Tags, ", "))
		}
		lines = append(lines, fmt.Sprintf("%s-%s: %s%s", g.Prefix, g.Name, g.Description, tags))
	}
	return strings.Join(lines, "\n")
}

// RenderCategorySuggestPrompt renders the category suggestion prompt.
func (c *Config) RenderCategorySuggestPrompt(categories *CategoriesConfig, glyphs []GlyphMetadata) (string, error) {
	// Format categories
	categoriesText := FormatCategoriesForPrompt(categories)

	// Format preferences
	preferencesText := c.FormatPreferencesForPrompt()

	// Format glyphs (limit to avoid token overflow)
	glyphsText := FormatGlyphsForPrompt(glyphs, 2000)

	vars := CategorySuggestVars{
		Categories:      categoriesText,
		AvailableGlyphs: glyphsText,
		Preferences:     preferencesText,
		MaxPerCategory:  c.IconPreferences.MaxSuggestionsPerCategory,
	}

	t, err := template.New("category_suggest").Parse(c.Prompts.CategorySuggestPrompt)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// LoadConfig loads configuration from file, falling back to defaults.
func LoadConfig(path string) (*Config, error) {
	config := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil // Use defaults
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := toml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return config, nil
}

// GtkIconInfo represents a GTK symbolic icon from the Adwaita theme.
type GtkIconInfo struct {
	Name     string // Icon name without -symbolic.svg suffix
	Category string // Category directory (actions, apps, devices, etc.)
	FullName string // Full name with -symbolic suffix
}

// GtkIconCache represents the cached GTK icon data.
type GtkIconCache struct {
	Icons     []GtkIconInfo `json:"icons"`
	FetchedAt time.Time     `json:"fetched_at"`
}

// FormatGtkIconsForPrompt formats GTK icons for inclusion in the category suggestion prompt.
func FormatGtkIconsForPrompt(icons []GtkIconInfo) string {
	var lines []string
	for _, icon := range icons {
		lines = append(lines, fmt.Sprintf("%s-symbolic (%s)", icon.Name, icon.Category))
	}
	return strings.Join(lines, "\n")
}

// RenderCategorySuggestPromptWithGtk renders the category suggestion prompt with GTK icons.
func (c *Config) RenderCategorySuggestPromptWithGtk(categories *CategoriesConfig, glyphs []GlyphMetadata, gtkIcons []GtkIconInfo) (string, error) {
	// Format categories
	categoriesText := FormatCategoriesForPrompt(categories)

	// Format preferences
	preferencesText := c.FormatPreferencesForPrompt()

	// Format glyphs (limit to avoid token overflow)
	glyphsText := FormatGlyphsForPrompt(glyphs, 2000)

	// Format GTK icons
	gtkIconsText := FormatGtkIconsForPrompt(gtkIcons)

	vars := CategorySuggestVars{
		Categories:        categoriesText,
		AvailableGlyphs:   glyphsText,
		AvailableGtkIcons: gtkIconsText,
		Preferences:       preferencesText,
		MaxPerCategory:    c.IconPreferences.MaxSuggestionsPerCategory,
	}

	t, err := template.New("category_suggest").Parse(c.Prompts.CategorySuggestPrompt)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// WriteDefaultConfig writes the default configuration to a file.
func WriteDefaultConfig(path string) error {
	content := `# Icon Aliases Generator Configuration
# =====================================
#
# This tool generates icon-aliases.toml for histui by:
# 1. Fetching icon metadata from upstream sources (Font Awesome, MDI, Devicons)
# 2. Generating kb-patterns.toml from upstream metadata
# 3. Merging with kb-patterns-manual.toml (your overrides)
# 4. Using AI to generate comprehensive Linux app lists for each icon
# 5. Outputting the final icon-aliases.toml
#
# Workflow:
#   ./generate-icon-aliases --fetch        # Update patterns from upstream
#   ./generate-icon-aliases --generate-kb  # Generate AI app mappings
#   ./generate-icon-aliases                # Output icon-aliases.toml
#
# See README.md for full documentation.

[openrouter]
# Model to use for AI app generation
default_model = "google/gemini-2.5-flash"

# Enable web search for real-time Linux ecosystem data
web_search = true

# Number of icons to process per API call
app_gen_batch_size = 50

# Request timeout in seconds
request_timeout = 600

# Maximum tokens to request in response
max_tokens = 32000

[upstream]
# Font Awesome - includes "brands" style for app logos
font_awesome = "https://raw.githubusercontent.com/FortAwesome/Font-Awesome/7.x/metadata/icons.json"

# Material Design Icons - community-maintained, extensive tags
material_design = "https://raw.githubusercontent.com/Templarian/MaterialDesign-Meta/master/meta.json"

# Devicons - developer tool and language logos (ALL are app-type)
devicons = "https://raw.githubusercontent.com/devicons/devicon/master/devicon.json"

# Codicons - VS Code icons
codicons = "https://raw.githubusercontent.com/microsoft/vscode-codicons/main/src/template/mapping.json"

# Adwaita GTK icons - GNOME's standard symbolic icons for GTK apps
adwaita_icons = "https://api.github.com/repos/GNOME/adwaita-icon-theme/git/trees/master?recursive=1"

[prompts]
# App generation prompt template
# Variables: {{.Year}} = current year, {{.Icons}} = icon list, {{.Categories}} = curated categories
app_gen_prompt = '''
You are generating Linux application identifiers for icon mappings in a desktop notification system.
Current year: {{.Year}} - include current and actively maintained apps in the Linux ecosystem.

For each icon, list ALL Linux applications that would use this icon. Be comprehensive.

Include these identifier types:
- Package names (apt/pacman/dnf): discord, firefox, thunderbird
- Flatpak IDs: com.discordapp.Discord, org.mozilla.firefox
- Snap names: discord, firefox
- Desktop file names: org.mozilla.firefox, com.discordapp.Discord
- Binary names: discord, firefox-esr
- Common forks/variants: librewolf, waterfox, vesktop, armcord

For "app" type icons (brand logos like Discord, Spotify):
- List the primary app and all known variants/forks
- Include official variants (discord-canary, spotify-client)
- Include popular third-party clients (vesktop for Discord)

For "category" type icons (generic like email, music, video):
- List the most popular Linux applications in that category
- Include both mainstream (thunderbird) and alternatives (evolution, geary)
- Include newer/modern alternatives (Ghostty for terminal, Zed for code)

IMPORTANT: When an app does not have a brand-specific icon (like Signal, which has no Nerd Font glyph),
assign it to an appropriate category icon instead. Use these curated categories:

{{.Categories}}

For apps without brand icons, add them to the appropriate category's app list with:
- confidence: 0.5 (category fallback)
- source: "category"

Confidence scoring:
- 1.0: Primary/official app (discord for discord icon)
- 0.9: Official variants (discord-canary, firefox-esr)
- 0.8: Well-known Flatpak/Snap IDs
- 0.7: Popular forks (librewolf, vesktop)
- 0.6: Less common alternatives
- 0.5: Category fallback (app has no brand icon)

Icons to map (format: "name (type) - description"):
{{.Icons}}

Respond with valid JSON only:
{
  "mappings": [
    {
      "icon": "discord",
      "apps": [
        {"id": "discord", "confidence": 1.0, "source": "package"},
        {"id": "com.discordapp.Discord", "confidence": 0.9, "source": "flatpak"},
        {"id": "discord-canary", "confidence": 0.9, "source": "package"},
        {"id": "vesktop", "confidence": 0.7, "source": "package"}
      ]
    }
  ]
}
'''

# Category icon suggestion prompt template
# Variables: {{.Categories}} = categories needing icons, {{.AvailableGlyphs}} = glyph list,
#            {{.Preferences}} = icon preferences, {{.MaxPerCategory}} = max suggestions
category_suggest_prompt = '''
You are selecting icons for application category fallbacks in a Linux desktop notification system.

For each category, suggest the best Nerd Font glyphs that visually represent that category.
These icons will be used when an application doesn't have its own brand-specific icon.

Categories to find icons for:
{{.Categories}}

Icon preferences (apply these when ranking):
{{.Preferences}}

Available glyphs (format: "prefix-name: description [tags]"):
{{.AvailableGlyphs}}

For each category, provide up to {{.MaxPerCategory}} icon suggestions ranked by:
1. Semantic match (icon meaning matches category purpose)
2. Visual clarity (recognizable at small sizes)
3. Preference alignment (preferred sets/styles ranked higher)
4. Universality (widely understood symbols preferred)

Respond with valid JSON only:
{
  "suggestions": {
    "messaging": [
      {
        "glyph": "md-chat",
        "confidence": 0.95,
        "reason": "Direct semantic match for chat/messaging, clean outline style"
      }
    ]
  }
}
'''

[icon_preferences]
# Preferred icon sets (first = highest priority)
# Valid values: "md" (Material Design), "fa" (Font Awesome), "cod" (Codicons), "dev" (Devicons)
preferred_sets = ["md", "fa", "cod", "dev"]

# Preferred icon styles (first = highest priority)
# Valid values: "outline", "solid", "filled", "regular"
preferred_styles = ["outline", "regular", "solid", "filled"]

# Glyph name patterns to deprioritize (e.g., simple shapes)
avoid_patterns = ["circle", "square", "box", "numeric"]

# Maximum number of suggestions per category
max_suggestions_per_category = 5
`
	return os.WriteFile(path, []byte(content), 0644)
}
