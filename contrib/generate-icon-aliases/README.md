# generate-icon-aliases

Generates `icon-aliases.toml` for histui by mapping Linux application names to Nerd Font icons.

## Overview

This tool creates icon mappings so histui can display appropriate icons for notifications from any Linux application, even when the app doesn't provide its own icon.

**Data flow:**
```
Upstream Metadata ──► kb-patterns.toml ──► Merged Patterns ──► icon-aliases.toml
(FA, MDI, Devicons)    (auto-generated)         │                  (output)
                                                │
                       kb-patterns-manual.toml ─┘
                       (icon overrides)
                                                │
                       extra-apps.toml ─────────┤
                       (apps to research)       │
                                                │
                       kb-ai-apps.json ─────────┤
                       (brand icon app mappings)│
                                                │
                       kb-ai-categories.json ───┘
                       (category app mappings)
```

## Quick Start

```bash
# First time setup
./generate-icon-aliases --fetch          # Download upstream metadata, generate patterns
./generate-icon-aliases --ai-apps        # Generate AI app mappings for brand icons
./generate-icon-aliases --ai-categories  # Generate AI app mappings for categories

# Generate output
./generate-icon-aliases --output icon-aliases.toml

# Or use the task runner from histui root
task generate:icons
```

## Commands

| Flag | Description |
|------|-------------|
| `--fetch` | Download upstream metadata and regenerate `kb-patterns.toml` |
| `--ai-apps` | Generate AI app mappings for brand icons (saves to `kb-ai-apps.json`) |
| `--ai-categories` | Generate AI app mappings for categories (saves to `kb-ai-categories.json`) |
| `--output PATH` | Output TOML file path (default: `icon-aliases.toml`) |
| `--verbose` | Show detailed processing information |
| `--no-cache` | Disable caching of API responses |
| `--clear-cache` | Clear the API response cache before generating |

## Files

| File | Type | Description |
|------|------|-------------|
| `config.toml` | Config | OpenRouter settings, upstream URLs, AI prompts |
| `kb-patterns.toml` | Auto-generated | Icon patterns from upstream metadata |
| `kb-patterns-manual.toml` | Manual | Icon overrides (version controlled) |
| `extra-apps.toml` | Manual | Apps to research during AI generation |
| `kb-ai-apps.json` | Generated | AI-generated app mappings for brand icons |
| `kb-ai-categories.json` | Generated | AI-generated app mappings for categories |
| `kb-categories.toml` | Manual | Category definitions with icons |
| `icon-aliases.toml` | Output | Final output for histui |
| `glyphnames.json` | Downloaded | Nerd Fonts glyph data |

## How It Works

### 1. Upstream Metadata (`--fetch`)

The tool fetches icon metadata from authoritative sources:

| Source | Prefix | What it provides |
|--------|--------|------------------|
| [Font Awesome](https://fontawesome.com) | `fa-` | `styles: ["brands"]` identifies app logos |
| [Material Design Icons](https://materialdesignicons.com) | `md-` | `tags` like "Brand / Logo" |
| [Devicons](https://devicon.dev) | `dev-` | All developer tool logos (100% apps) |
| [Codicons](https://github.com/microsoft/vscode-codicons) | `cod-` | VS Code icons |

This generates `kb-patterns.toml` with entries like:

```toml
[icons.discord]
patterns = ["discord"]
search_terms = ["chat", "gaming", "voice"]
type = "app"                    # FA styles: ["brands"]
upstream = "fa"
```

### 2. Manual Overrides (`kb-patterns-manual.toml`)

Add your own patterns for:
- Icons missing from upstream (e.g., Signal has no Nerd Font glyph)
- Fixing misclassifications
- Forcing specific app → icon mappings

```toml
[icons.signal]
patterns = ["md-message_lock"]
type = "app"
description = "Signal private messenger"
upstream = "manual"
force_apps = ["signal", "signal-desktop", "org.signal.Signal"]
```

### 3. AI App Generation

AI generation is split into two steps:

#### Brand Icons (`--ai-apps`)

For each brand icon (Discord, Firefox, etc.), AI generates a comprehensive list of Linux applications including official clients, forks, and third-party clients:

```json
{
  "discord": {
    "type": "app",
    "glyph": "md-discord",
    "apps": [
      {"id": "discord", "confidence": 1.0},
      {"id": "com.discordapp.Discord", "confidence": 0.9},
      {"id": "vesktop", "confidence": 0.7},
      {"id": "webcord", "confidence": 0.7}
    ]
  }
}
```

AI is skipped for icons with `force_apps` in the manual file.

#### Categories (`--ai-categories`)

For each category (email, messaging, etc.), AI generates a list of Linux applications that belong to that category:

```json
{
  "email": {
    "type": "category",
    "glyph": "md-email",
    "apps": [
      {"id": "thunderbird", "confidence": 1.0},
      {"id": "evolution", "confidence": 1.0},
      {"id": "geary", "confidence": 1.0}
    ]
  }
}
```

This ensures apps without brand icons get appropriate category fallback icons.

### 4. Output (`icon-aliases.toml`)

The final output maps app names to icons:

```toml
[aliases]
discord = "discord"
discord-canary = "discord"
vesktop = "discord"
com.discordapp.Discord = "discord"

[symbols]
discord = "\U000F066F"  # Nerd Font glyph
```

## Configuration

Edit `config.toml` to customize:

### OpenRouter Settings

```toml
[openrouter]
default_model = "google/gemini-2.5-flash"
web_search = true
app_gen_batch_size = 50
request_timeout = 600
```

Set your API key:
```bash
export OPENROUTER_API_KEY="sk-or-..."
```

### Upstream Sources

```toml
[upstream]
font_awesome = "https://raw.githubusercontent.com/FortAwesome/Font-Awesome/7.x/metadata/icons.json"
material_design = "https://raw.githubusercontent.com/Templarian/MaterialDesign-Meta/master/meta.json"
devicons = "https://raw.githubusercontent.com/devicons/devicon/master/devicon.json"
```

### AI Prompt

The `[prompts].app_gen_prompt` template controls how AI generates app lists. Template variables:
- `{{.Year}}` - Current year
- `{{.Icons}}` - List of icons to process

## Icon Types

| Type | Description | Example |
|------|-------------|---------|
| `app` | Brand logo for a specific application | Discord, Firefox, Spotify |
| `category` | Generic icon for a class of applications | email, music, terminal |

For `app` icons, AI lists variants of that specific app.
For `category` icons, AI lists popular apps in that category.

## Adding New Icons

### If upstream has the icon

1. Run `--fetch` to update patterns from upstream
2. Run `--ai-apps` to generate app mappings for brand icons
3. The icon will appear in output

### If upstream is missing the icon

1. Add to `kb-patterns-manual.toml`:
   ```toml
   [icons.myapp]
   patterns = ["md-some-icon"]
   type = "app"
   description = "My Application"
   upstream = "manual"
   force_apps = ["myapp", "my-app-desktop"]
   ```
2. Run generation again

### If AI misses some apps

Add the app names to `extra-apps.toml`:
```toml
[apps]
include = [
    "elecwhat",      # WhatsApp third-party client
    "ripcord",       # Discord/Slack client
    "gtkcord4",      # Discord GTK client
]
```

Then regenerate:
```bash
./generate-icon-aliases --ai-apps
./generate-icon-aliases --output icon-aliases.toml
```

The AI will research each app and classify it to the appropriate icon.

### For icons without Nerd Font glyphs

Use `force_apps` in `kb-patterns-manual.toml` for complete control:
```toml
[icons.signal]
patterns = ["md-message_lock"]
type = "app"
description = "Signal private messenger"
upstream = "manual"
force_apps = ["signal", "signal-desktop", "org.signal.Signal"]
```
Changes take effect immediately on normal regeneration (no `--ai-apps` needed).

## Caching

- Upstream metadata: Re-fetched on each `--fetch`
- AI responses: Cached in `.cache/` by model and input hash
- Use `--force` to regenerate AI mappings
- Use `--clear-cache` to clear all cached responses

## Troubleshooting

### "No API key found"
```bash
export OPENROUTER_API_KEY="sk-or-..."
```

### AI outputs malformed JSON
Try a different model in `config.toml`. Claude and Gemini have best JSON reliability.

### Icon not matching
1. Check `kb-patterns.toml` for the icon entry
2. Check patterns match the Nerd Font glyph name
3. Add override in `kb-patterns-manual.toml`

### Missing apps in output
1. Check confidence threshold (apps require ≥0.7, categories ≥0.3)
2. Add apps to `extra-apps.toml`, then regenerate with `--ai-apps`
3. Or use `force_apps` in `kb-patterns-manual.toml` for complete control

## Confidence Thresholds

The generator uses a two-tier confidence system:

| Tier | Threshold | Description |
|------|-----------|-------------|
| Brand icons | ≥0.7 | Apps confidently mapped to specific brand icons (e.g., `discord → discord`) |
| Category fallbacks | 0.3-0.7 | Low-confidence apps get assigned to generic categories (e.g., `myapp → messaging`) |

Apps with confidence <0.3 are excluded entirely.

### Category Fallback Assignment

When `--assign-category-fallbacks` is used:

1. Apps with <0.7 confidence for brand icons are collected
2. AI assigns each to the most appropriate category from `kb-categories.toml`
3. Low-confidence apps get category fallback icons instead of potentially wrong brand icons

Example output with category fallbacks:
```toml
[aliases]
discord = 'discord'           # 1.00 confidence
vesktop = 'discord'           # 0.85 confidence
my-chat-app = 'messaging'     # 0.50 (category) - no brand icon, falls back to category
```

## Output Format

The output file includes:

```toml
[meta]
version = 1
generated_at = '2025-01-15T10:30:00Z'
generator = "contrib/generate-icon-aliases"

[aliases]
discord = 'discord'           # 1.00
firefox = 'firefox'           # 0.95
my-app = 'messaging'          # 0.50 (category)

[apps]
discord = { symbol = "󰙯", gtk_icon = "discord" }
firefox = { symbol = "󰈹", gtk_icon = "firefox" }

[categories]
messaging = { symbol = "󱃲", gtk_icon = "mail-symbolic" }
```

## References

- [Nerd Fonts Cheat Sheet](https://www.nerdfonts.com/cheat-sheet)
- [Font Awesome Icons](https://fontawesome.com/icons)
- [Material Design Icons](https://materialdesignicons.com/)
- [Devicons](https://devicon.dev/)
- [OpenRouter Models](https://openrouter.ai/models)
