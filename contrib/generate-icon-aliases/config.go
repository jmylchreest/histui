package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/BurntSushi/toml"
)

const configFile = "config.toml"

// Config represents the generator configuration.
type Config struct {
	// OpenRouter settings
	OpenRouter OpenRouterConfig `toml:"openrouter"`

	// Upstream metadata sources
	Upstream UpstreamConfig `toml:"upstream"`

	// Prompts for AI generation (supports {{.Year}} template)
	Prompts PromptConfig `toml:"prompts"`
}

// OpenRouterConfig contains OpenRouter API settings.
type OpenRouterConfig struct {
	// DefaultModel is the model to use when not specified via flag/env
	DefaultModel string `toml:"default_model"`

	// WebSearch enables real-time web search
	WebSearch bool `toml:"web_search"`

	// AppGenBatchSize is the number of icons to generate apps for per API call
	AppGenBatchSize int `toml:"app_gen_batch_size"`

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
}

// PromptConfig contains customizable AI prompts.
type PromptConfig struct {
	// AppGenPrompt is the prompt template for app name generation
	AppGenPrompt string `toml:"app_gen_prompt"`
}

// PromptVars contains variables for prompt templates.
type PromptVars struct {
	Year  int
	Icons string // newline-separated list of icons
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		OpenRouter: OpenRouterConfig{
			DefaultModel:    "google/gemini-2.5-flash",
			WebSearch:       true,
			AppGenBatchSize: 50,
			RequestTimeout:  600,
			MaxTokens:       32000,
		},
		Upstream: UpstreamConfig{
			FontAwesome:    "https://raw.githubusercontent.com/FortAwesome/Font-Awesome/6.x/metadata/icons.json",
			MaterialDesign: "https://raw.githubusercontent.com/Templarian/MaterialDesign-Meta/master/meta.json",
			Devicons:       "https://raw.githubusercontent.com/devicons/devicon/master/devicon.json",
			Codicons:       "https://raw.githubusercontent.com/microsoft/vscode-codicons/main/src/template/mapping.json",
		},
		Prompts: PromptConfig{
			AppGenPrompt: defaultAppGenPrompt,
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

Confidence scoring:
- 1.0: Primary/official app (discord for discord icon)
- 0.9: Official variants (discord-canary, firefox-esr)
- 0.8: Well-known Flatpak/Snap IDs
- 0.7: Popular forks (librewolf, vesktop)
- 0.6: Less common alternatives

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

// RenderAppGenPrompt renders the app generation prompt with icons.
func (c *Config) RenderAppGenPrompt(iconList []string) (string, error) {
	vars := PromptVars{
		Year:  time.Now().Year(),
		Icons: strings.Join(iconList, "\n"),
	}
	return RenderPrompt(c.Prompts.AppGenPrompt, vars)
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

	if _, err := toml.Decode(string(data), config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return config, nil
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
font_awesome = "https://raw.githubusercontent.com/FortAwesome/Font-Awesome/6.x/metadata/icons.json"

# Material Design Icons - community-maintained, extensive tags
material_design = "https://raw.githubusercontent.com/Templarian/MaterialDesign-Meta/master/meta.json"

# Devicons - developer tool and language logos (ALL are app-type)
devicons = "https://raw.githubusercontent.com/devicons/devicon/master/devicon.json"

# Codicons - VS Code icons
codicons = "https://raw.githubusercontent.com/microsoft/vscode-codicons/main/src/template/mapping.json"

[prompts]
# App generation prompt template
# Variables: {{.Year}} = current year, {{.Icons}} = icon list
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

Confidence scoring:
- 1.0: Primary/official app (discord for discord icon)
- 0.9: Official variants (discord-canary, firefox-esr)
- 0.8: Well-known Flatpak/Snap IDs
- 0.7: Popular forks (librewolf, vesktop)
- 0.6: Less common alternatives

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
`
	return os.WriteFile(path, []byte(content), 0644)
}
