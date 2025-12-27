# generate-icon-aliases

A utility to generate icon alias mappings from Nerd Fonts glyph data.

## Purpose

This tool:
1. Downloads/parses `glyphnames.json` from the [Nerd Fonts](https://github.com/ryanoasis/nerd-fonts) repository
2. Filters for app-related icons (discord, firefox, whatsapp, etc.)
3. Maps common Linux application names to standard icon names
4. Generates `icon-aliases.toml` for embedding in histui

## Usage

```bash
# From the histui root directory, use the task:
task generate:icons

# Or run manually:
cd contrib/generate-icon-aliases
go run . --fetch --output ../../internal/icon/aliases_default.toml --verbose
```

## Flags

| Flag | Description |
|------|-------------|
| `--fetch` | Download fresh `glyphnames.json` and font from GitHub |
| `--output PATH` | Output TOML file path (default: `icon-aliases.toml`) |
| `--font-output PATH` | Output path for Nerd Font symbols TTF (optional) |
| `--verbose` | Show detailed matching information |

## Icon Categories

The tool recognizes icons from these Nerd Font prefixes:
- `nf-md-` - Material Design Icons (preferred)
- `nf-fa-` - Font Awesome
- `nf-dev-` - Devicons
- `nf-linux-` - Linux distro icons
- `nf-custom-` - Custom icons
- `nf-seti-` - Seti UI

## Adding New App Mappings

Edit the `knownAppIcons` map in `main.go` to add new mappings:

```go
var knownAppIcons = map[string][]string{
    // Icon name (matches Nerd Font) -> list of Linux app names
    "discord": {"discord", "discord-canary", "vesktop", "webcord"},
    "myapp":   {"myapp-linux", "myapp-desktop", "org.example.myapp"},
}
```

The tool will automatically find matching Nerd Font icons like `nf-md-discord`.

## Output Files

### icon-aliases.toml

User-editable configuration file mapping app names to icon names:

```toml
[aliases]
# Messaging & Social
discord-canary = "discord"
vesktop = "discord"
zapzap = "whatsapp"
telegram-desktop = "telegram"
```

The generated file is embedded at build time from `internal/icon/aliases_default.toml`.
Users can override any mapping by placing their own `icon-aliases.toml` at `~/.config/histui/icon-aliases.toml`.

## Updating When Nerd Fonts Changes

When Nerd Fonts releases an update:

1. Run with `--fetch` to get the latest glyph data
2. Review the output for any new or removed icons
3. Update `knownAppIcons` if needed for new apps
4. Regenerate the aliases

## References

- [Nerd Fonts Cheat Sheet](https://www.nerdfonts.com/cheat-sheet)
- [Nerd Fonts GitHub](https://github.com/ryanoasis/nerd-fonts)
- [Material Design Icons](https://materialdesignicons.com/)
