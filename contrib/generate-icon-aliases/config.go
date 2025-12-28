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

	// Prompts for AI generation (supports {{.Year}} template)
	Prompts PromptConfig `toml:"prompts"`
}

// OpenRouterConfig contains OpenRouter API settings.
type OpenRouterConfig struct {
	// DefaultModel is the model to use when not specified via flag/env
	// Append ":online" for web search (e.g., "anthropic/claude-sonnet-4:online")
	DefaultModel string `toml:"default_model"`

	// WebSearch enables real-time web search by default
	WebSearch bool `toml:"web_search"`

	// ClassifyBatchSize is the number of icons to classify per API call
	// Higher = fewer API calls but may hit output token limits
	// Modern models can handle 200-500 icons per batch
	ClassifyBatchSize int `toml:"classify_batch_size"`

	// AppGenBatchSize is the number of icons to generate apps for per API call
	// Keep smaller than classify since output is much larger per icon
	AppGenBatchSize int `toml:"app_gen_batch_size"`

	// RequestTimeout is the timeout for each API request in seconds
	// Web search requests need longer timeouts (300-600s recommended)
	RequestTimeout int `toml:"request_timeout"`
}

// PromptConfig contains customizable AI prompts.
// Prompts support Go templates with {{.Year}} for current year.
type PromptConfig struct {
	// ClassifyPrompt is the full prompt template for icon classification
	ClassifyPrompt string `toml:"classify_prompt"`

	// AppGenPrompt is the full prompt template for app name generation
	AppGenPrompt string `toml:"app_gen_prompt"`
}

// PromptVars contains variables for prompt templates.
type PromptVars struct {
	Year   int
	Icons  string // newline-separated list of icons
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		OpenRouter: OpenRouterConfig{
			DefaultModel:      "anthropic/claude-sonnet-4",
			WebSearch:         true,
			ClassifyBatchSize: 500, // Large batches - modern models handle easily
			AppGenBatchSize:   100, // Smaller since output per icon is larger
			RequestTimeout:    600, // 10 minutes - web search needs longer
		},
		Prompts: PromptConfig{
			ClassifyPrompt: defaultClassifyPrompt,
			AppGenPrompt:   defaultAppGenPrompt,
		},
	}
}

const defaultClassifyPrompt = `You are classifying Nerd Font icon names for a Linux desktop notification system.
Current year: {{.Year}} - use current knowledge of the Linux app ecosystem.

For each icon glyph name, determine:
- type: "app" if it represents a specific application (Discord, Firefox, Spotify), "category" if it's generic (email, browser, folder, music), or "skip" if not useful for app icons (arrows, shapes, abstract symbols)
- name: the canonical lowercase name extracted from the glyph (e.g., "md-discord" -> "discord", "fa-envelope" -> "email", "md-folder" -> "folder")

Focus on icons that would be useful for matching Linux desktop applications and notification sources.

Icons to classify:
{{.Icons}}

Respond with JSON in this format:
{
  "icons": [
    {"glyph": "md-discord", "type": "app", "name": "discord"},
    {"glyph": "fa-envelope", "type": "category", "name": "email"},
    {"glyph": "md-arrow-left", "type": "skip", "name": ""}
  ]
}`

const defaultAppGenPrompt = `You are generating Linux application identifiers for icon mappings in a desktop notification system.
Current year: {{.Year}} - include current and actively maintained apps in the Linux ecosystem.

For each icon, list all Linux apps that would use this icon. Include:
- Package names (apt, pacman, dnf, etc.): discord, firefox, thunderbird
- Desktop file base names: org.mozilla.firefox, com.discordapp.Discord
- Flatpak application IDs: com.discordapp.Discord, org.mozilla.firefox
- Common variants and forks: discord-canary, firefox-esr, firefox-nightly, librewolf
- New/popular apps from recent years (Vesktop, Zen Browser, Ghostty, Zed, Cursor, Windsurf, etc.)

Confidence scoring guidelines:
- 0.9-1.0: Official/primary app that exactly matches the icon (discord for discord icon)
- 0.7-0.9: Well-known official variants (discord-canary, firefox-esr)
- 0.5-0.7: Popular third-party clients or forks (vesktop, librewolf, evolution for email)
- 0.3-0.5: Less common alternatives or inferred matches

For "category" type icons (email, browser, file-manager, music), list the most popular Linux applications in that category, including newer alternatives.

Icons to map (format: name (type)):
{{.Icons}}

Be comprehensive but accurate. Only include apps you're confident exist on Linux.

Respond with JSON in this format:
{
  "mappings": [
    {
      "icon": "discord",
      "apps": [
        {"id": "discord", "confidence": 1.0, "source": "package"},
        {"id": "com.discordapp.Discord", "confidence": 0.95, "source": "flatpak"},
        {"id": "vesktop", "confidence": 0.6, "source": "package"}
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

// RenderClassifyPrompt renders the classification prompt with icons.
func (c *Config) RenderClassifyPrompt(iconList []string) (string, error) {
	vars := PromptVars{
		Year:  time.Now().Year(),
		Icons: strings.Join(iconList, "\n"),
	}
	return RenderPrompt(c.Prompts.ClassifyPrompt, vars)
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
# See https://openrouter.ai/models for available models
# See https://openrouter.ai/docs/features/web-search for web search

[openrouter]
# Default model for AI generation
# Recommended models (December 2025):
#   - anthropic/claude-sonnet-4.5  - Best for coding/agentic tasks
#   - anthropic/claude-sonnet-4    - Good balance of speed and quality
#   - openai/gpt-5.1               - Latest GPT with native structured output
#   - google/gemini-2.0-flash      - Fast and cheap for bulk operations
#   - anthropic/claude-opus-4.5    - Maximum quality (expensive)
default_model = "anthropic/claude-sonnet-4"

# Enable web search for real-time data (adds :online suffix)
# This ensures current package names, new apps, and ecosystem info
# Cost: ~$0.02 per request for Exa-powered search (native for Anthropic/OpenAI)
web_search = true

# Batch sizes for API calls
# Higher = fewer API calls, modern models handle large batches easily
classify_batch_size = 500  # Icons to classify per call
app_gen_batch_size = 100   # Icons to generate apps for per call (output is larger)

# Request timeout in seconds
# Web search requests need longer timeouts
request_timeout = 600  # 10 minutes

[prompts]
# Classification prompt template
# Supports {{.Year}} for current year, {{.Icons}} for icon list
classify_prompt = '''
You are classifying Nerd Font icon names for a Linux desktop notification system.
Current year: {{.Year}} - use current knowledge of the Linux app ecosystem.

For each icon glyph name, determine:
- type: "app" if it represents a specific application (Discord, Firefox, Spotify), "category" if it's generic (email, browser, folder, music), or "skip" if not useful for app icons (arrows, shapes, abstract symbols)
- name: the canonical lowercase name extracted from the glyph (e.g., "md-discord" -> "discord", "fa-envelope" -> "email", "md-folder" -> "folder")

Focus on icons that would be useful for matching Linux desktop applications and notification sources.

Icons to classify:
{{.Icons}}

Respond with JSON in this format:
{
  "icons": [
    {"glyph": "md-discord", "type": "app", "name": "discord"},
    {"glyph": "fa-envelope", "type": "category", "name": "email"},
    {"glyph": "md-arrow-left", "type": "skip", "name": ""}
  ]
}
'''

# App generation prompt template
# Supports {{.Year}} for current year, {{.Icons}} for icon list
app_gen_prompt = '''
You are generating Linux application identifiers for icon mappings in a desktop notification system.
Current year: {{.Year}} - include current and actively maintained apps in the Linux ecosystem.

For each icon, list all Linux apps that would use this icon. Include:
- Package names (apt, pacman, dnf, etc.): discord, firefox, thunderbird
- Desktop file base names: org.mozilla.firefox, com.discordapp.Discord
- Flatpak application IDs: com.discordapp.Discord, org.mozilla.firefox
- Common variants and forks: discord-canary, firefox-esr, firefox-nightly, librewolf
- New/popular apps from recent years (Vesktop, Zen Browser, Ghostty, Zed, Cursor, Windsurf, etc.)

Confidence scoring guidelines:
- 0.9-1.0: Official/primary app that exactly matches the icon (discord for discord icon)
- 0.7-0.9: Well-known official variants (discord-canary, firefox-esr)
- 0.5-0.7: Popular third-party clients or forks (vesktop, librewolf, evolution for email)
- 0.3-0.5: Less common alternatives or inferred matches

For "category" type icons (email, browser, file-manager, music), list the most popular Linux applications in that category, including newer alternatives.

Icons to map (format: name (type)):
{{.Icons}}

Be comprehensive but accurate. Only include apps you're confident exist on Linux.

Respond with JSON in this format:
{
  "mappings": [
    {
      "icon": "discord",
      "apps": [
        {"id": "discord", "confidence": 1.0, "source": "package"},
        {"id": "com.discordapp.Discord", "confidence": 0.95, "source": "flatpak"},
        {"id": "vesktop", "confidence": 0.6, "source": "package"}
      ]
    }
  ]
}
'''
`
	return os.WriteFile(path, []byte(content), 0644)
}
