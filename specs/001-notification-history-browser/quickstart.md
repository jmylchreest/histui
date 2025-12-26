# Quickstart Guide: histui Development

**Date**: 2025-12-26
**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

## Prerequisites

### Required

- **Go 1.21+** (for `log/slog` stdlib)
- **dunst** notification daemon (for testing dunst adapter)
- **wl-clipboard** (for Wayland clipboard in TUI mode)

### Recommended

- **Task** (Taskfile runner) - `go install github.com/go-task/task/v3/cmd/task@latest`
- **golangci-lint** - `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
- **fuzzel/walker/dmenu/rofi** (for testing launcher integration)

### Verify Prerequisites

```bash
# Check Go version
go version  # Should be 1.21 or higher

# Check dunst is installed and running
dunstctl count

# Check clipboard tools
which wl-copy  # Should return a path

# Optional: Check task is installed
task --version
```

---

## Project Setup

### 1. Clone and Initialize

```bash
# Clone the repository
git clone https://github.com/jmylchreest/histui.git
cd histui

# Initialize Go modules
go mod init github.com/jmylchreest/histui
go mod tidy
```

### 2. Install Dependencies

```bash
# Core dependencies
go get github.com/spf13/cobra@latest
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/oklog/ulid/v2@latest
go get github.com/blacktop/go-termimg@latest   # Terminal image rendering
go get github.com/pelletier/go-toml/v2@latest  # Configuration file

# Testing dependencies
go get github.com/stretchr/testify@latest
```

### 3. Create Project Structure

```bash
# Create directory structure
mkdir -p cmd/histui
mkdir -p internal/{adapter/input,adapter/output,store,core,model}
mkdir -p examples/waybar

# Verify structure
tree -d
```

Expected output:
```
.
├── cmd
│   └── histui
├── examples
│   └── waybar
├── internal
│   ├── adapter
│   │   ├── input
│   │   └── output
│   ├── core
│   ├── model
│   └── store
└── specs
    └── 001-notification-history-browser
```

---

## Development Workflow

### Running the Application

```bash
# Build and run
go build -o histui ./cmd/histui
./histui --help

# Or use go run
go run ./cmd/histui --help

# Default behavior (no subcommand = TUI)
./histui

# With Task (if Taskfile.yml exists)
task build
task run
```

### Command Overview

```bash
# TUI mode (default)
histui              # Launch interactive TUI
histui tui          # Explicit TUI mode

# Get notifications
histui get                        # List all (default 48h)
histui get --format dmenu         # dmenu-compatible output
histui get --format json          # JSON array output
histui get --ulid --title         # Include ULID for reliable piping

# Lookup specific notification (pipe selection back)
echo "01HQG..." | histui get --body           # Get body by ULID
echo "firefox | ..." | histui get --all       # Match by content

# Status for waybar
histui status                     # JSON status output

# Prune old notifications
histui prune --dry-run            # Preview cleanup
histui prune --older-than 7d      # Remove older than 7 days
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package tests
go test -v ./internal/adapter/input/...
go test -v ./internal/store/...

# Run with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Linting

```bash
# Format code
go fmt ./...

# Run linter
golangci-lint run

# With auto-fix
golangci-lint run --fix
```

---

## Testing Scenarios

### Test Dunst Adapter

```bash
# Generate test notifications
notify-send "Test" "This is a test notification"
notify-send -u critical "Critical" "Important message"
notify-send -a "myapp" "App Test" "From custom app"

# View raw dunst history
dunstctl history | jq .

# Run histui get command
histui get                        # List with default format
histui get --format json | jq .   # JSON output
```

### Test Stdin Adapter

```bash
# Pipe dunst history directly
dunstctl history | histui get --source stdin

# Test with sample JSON
echo '[{"app_name":"test","summary":"Hello","body":"World","timestamp":1703577500,"urgency":1}]' \
  | histui get --source stdin
```

### Test Get Command

```bash
# Basic output (default 48h filter)
histui get

# With dmenu format
histui get --format dmenu

# With ULID for reliable piping
histui get --ulid --title --app

# Full JSON output
histui get --format json | jq .

# Filter by time
histui get --since 1h              # Last hour
histui get --since 7d              # Last 7 days
histui get --since 0               # All time (no filter)

# Filter by app
histui get --app-filter firefox

# Sort options
histui get --sort timestamp:desc   # Default
histui get --sort app:asc          # By app name ascending
histui get --sort urgency:desc     # Critical first

# Limit results
histui get --limit 10
```

### Test Notification Lookup

```bash
# Lookup by ULID (reliable method)
echo "01HQGXK5P0000000000000000" | histui get --body

# Lookup by content (fallback matching)
echo "firefox | Download Complete - myfile.zip | 5m ago" | histui get --all

# Get specific fields
echo "01HQG..." | histui get --app --title --body --timestamp
echo "01HQG..." | histui get --time-relative  # "5 minutes ago"
```

### Test TUI Mode

```bash
# Launch interactive TUI (default)
histui

# Explicit TUI
histui tui

# Expected controls:
# j/k or arrows: navigate
# /: search
# Enter: view details
# y: copy to clipboard
# q: quit
```

### Test Status Command (Waybar)

```bash
# Output waybar JSON
histui status | jq .

# Expected output:
# {"text":"","alt":"enabled","tooltip":"Notifications enabled\n5 in history","class":"enabled"}
```

### Test Prune Command

```bash
# Preview what would be removed
histui prune --dry-run

# Remove notifications older than 48h (default)
histui prune

# Custom retention
histui prune --older-than 7d       # Keep last 7 days
histui prune --keep 100            # Keep at most 100
histui prune --older-than 24h --keep 50  # Both constraints
```

### Test Persistence

```bash
# Enable persistence (flag on any command)
histui get --persist

# Check persistence file
cat ~/.local/share/histui/history.jsonl

# Verify history persists across restarts
histui get --persist --format json
```

---

## TUI Pipeline Usage

The TUI can be used directly in pipelines like `fzf`. Navigate with `j/k`, view with `Enter`, then press `Enter` again (or `p` in list view) to print to stdout and exit.

### Basic TUI Pipelines

```bash
# Browse interactively, print selected notification body to clipboard
histui | wl-copy

# Browse and print with custom template
histui --output-template '{{.Body}}' | wl-copy

# Browse and open URL from selected notification
histui --output-template '{{.Body}}' | xargs -r xdg-open

# Browse, select, and send to another notification
histui --output-template '{{.Summary}}: {{.Body}}' | xargs notify-send "Selected"
```

### TUI Output Templates

The TUI output template can be set via:
1. Config file: `~/.config/histui/config.toml` under `[templates] tui_output`
2. CLI flag: `--output-template '{{.Body}}'`

```bash
# Examples with different templates
histui --output-template '{{.Body}}'                    # Body only
histui --output-template '{{.AppName}}: {{.Summary}}'   # App and title
histui --output-template '{{.Timestamp | formatTime}} {{.AppName}}: {{.Summary}}\n{{.Body}}'  # Full
```

---

## Launcher Integration

### Complete Pipeline Examples

The key workflow is: list notifications → user selects one → extract desired field → action (copy, open, etc.)

```bash
# Basic pattern
histui get --format dmenu --ulid | LAUNCHER | histui get --FIELD | ACTION
```

### Fuzzel (Wayland - Recommended)

```bash
# Copy notification body to clipboard
histui get --format dmenu --ulid | fuzzel -d | histui get --body | wl-copy

# Copy all notification details
histui get --format dmenu --ulid | fuzzel -d | histui get --all | wl-copy

# Show notification in terminal
histui get --format dmenu --ulid | fuzzel -d | histui get --app --title --body --timestamp

# Open URL from notification (if body contains URL)
histui get --format dmenu --ulid | fuzzel -d | histui get --body | xargs -r xdg-open
```

**Fuzzel script** (`~/.local/bin/fuzzel-notifications.sh`):

```bash
#!/bin/bash
# Browse notifications with fuzzel and copy selection

selected=$(histui get --format dmenu --ulid | fuzzel -d -p "Notifications: ")
if [ -n "$selected" ]; then
    echo "$selected" | histui get --body | wl-copy
    notify-send "Copied" "Notification body copied to clipboard"
fi
```

### Walker (Wayland)

```bash
# Copy body to clipboard
histui get --format dmenu --ulid | walker --dmenu | histui get --body | wl-copy

# With custom prompt
histui get --format dmenu --ulid | walker --dmenu --placeholder "Select notification" \
  | histui get --all | wl-copy
```

**Walker script** (`~/.local/bin/walker-notifications.sh`):

```bash
#!/bin/bash
# Browse notifications with walker

selected=$(histui get --format dmenu --ulid | walker --dmenu)
if [ -n "$selected" ]; then
    echo "$selected" | histui get --body | wl-copy
fi
```

### dmenu (X11)

```bash
# Copy body to clipboard
histui get --format dmenu --ulid | dmenu -l 20 | histui get --body | xclip -selection clipboard

# With prompt and custom font
histui get --format dmenu --ulid | dmenu -l 15 -p "Notifications:" -fn "monospace:size=12" \
  | histui get --body | xclip -selection clipboard
```

### Rofi

```bash
# Copy body to clipboard (Wayland)
histui get --format dmenu --ulid | rofi -dmenu -p "Notifications" \
  | histui get --body | wl-copy

# Copy body to clipboard (X11)
histui get --format dmenu --ulid | rofi -dmenu -p "Notifications" \
  | histui get --body | xclip -selection clipboard

# With custom theme
histui get --format dmenu --ulid | rofi -dmenu -p "Notifications" -theme notification-browser \
  | histui get --all | wl-copy
```

### Wofi (Wayland)

```bash
# Copy body to clipboard
histui get --format dmenu --ulid | wofi -d -p "Notifications" \
  | histui get --body | wl-copy
```

---

## Waybar Integration

### Example Configuration

Add to `~/.config/waybar/config`:

```json
{
    "custom/notification": {
        "exec": "histui status",
        "return-type": "json",
        "interval": 5,
        "format": "{icon}",
        "format-icons": {
            "enabled": "󰂚",
            "paused": "󰂛",
            "paused-*": "󰂛 ({})",
            "unavailable": "󰂭"
        },
        "on-click": "~/.local/bin/fuzzel-notifications.sh",
        "on-click-middle": "histui",
        "on-click-right": "dunstctl set-paused toggle"
    }
}
```

**Actions:**
- Left click: Open fuzzel notification browser (copy body to clipboard)
- Middle click: Open TUI mode for full browsing
- Right click: Toggle dunst pause state

Add to `~/.config/waybar/style.css`:

```css
#custom-notification {
    font-size: 16px;
}

#custom-notification.enabled {
    color: #89b4fa;
}

#custom-notification.paused {
    color: #f38ba8;
}

#custom-notification.error {
    color: #fab387;
}
```

### Alternative: Inline Waybar Command

If you prefer not to use a script:

```json
{
    "custom/notification": {
        "exec": "histui status",
        "return-type": "json",
        "interval": 5,
        "format": "{icon}",
        "format-icons": {
            "enabled": "󰂚",
            "paused": "󰂛",
            "unavailable": "󰂭"
        },
        "on-click": "bash -c 'histui get --format dmenu --ulid | fuzzel -d | histui get --body | wl-copy'",
        "on-click-right": "dunstctl set-paused toggle"
    }
}
```

---

## Hyprland Keybindings

Add to `~/.config/hypr/hyprland.conf`:

```bash
# Notification browser with fuzzel
bind = $mainMod, N, exec, histui get --format dmenu --ulid | fuzzel -d | histui get --body | wl-copy

# TUI mode
bind = $mainMod SHIFT, N, exec, kitty --class histui-tui -e histui

# Quick dismiss
bind = $mainMod, D, exec, dunstctl close-all
```

Window rules for TUI:

```bash
windowrulev2 = float, class:^(histui-tui)$
windowrulev2 = size 800 600, class:^(histui-tui)$
windowrulev2 = center, class:^(histui-tui)$
```

---

## Debugging

### Enable Verbose Logging

```bash
# Run with verbose flag
histui --verbose get

# Logs go to stderr, output to stdout
histui --verbose get 2>debug.log
```

### Debug dunstctl Output

```bash
# View raw dunst history
dunstctl history | jq '.'

# Check timestamp format (microseconds since boot)
dunstctl history | jq '.data[0][0].timestamp'

# Check urgency format
dunstctl history | jq '.data[0][0].urgency'
```

### Debug Persistence

```bash
# View persistence file
cat ~/.local/share/histui/history.jsonl | head -20

# Validate JSONL format (each line should be valid JSON)
while read -r line; do echo "$line" | jq -c .; done < ~/.local/share/histui/history.jsonl | head -5

# Check file permissions (should be 0600)
ls -la ~/.local/share/histui/history.jsonl
```

### Debug Clipboard (TUI only)

```bash
# Test wl-copy directly
echo "test" | wl-copy
wl-paste  # Should output "test"

# Check environment
echo $WAYLAND_DISPLAY
echo $DISPLAY
```

### Debug Pipeline

```bash
# Step-by-step debugging of the notification pipeline

# Step 1: Verify get output
histui get --format dmenu --ulid | head -5

# Step 2: Test selection manually
echo "01HQGXK5P0000000000000000 | firefox | Download Complete | 5m ago" | histui get --body

# Step 3: Verify clipboard
histui get --format dmenu --ulid | head -1 | histui get --body | wl-copy && wl-paste
```

---

## Common Issues

### "dunstctl: command not found"

Install dunst:
```bash
# Arch Linux
sudo pacman -S dunst

# Ubuntu/Debian
sudo apt install dunst
```

### "wl-copy: command not found"

Install wl-clipboard (only needed for TUI clipboard):
```bash
# Arch Linux
sudo pacman -S wl-clipboard

# Ubuntu/Debian
sudo apt install wl-clipboard
```

Note: For `get` command pipelines, clipboard is handled by the shell (`| wl-copy`), not histui.

### TUI not rendering correctly

Ensure terminal supports required features:
```bash
# Check TERM setting
echo $TERM  # Should be xterm-256color or similar

# Try with explicit TERM
TERM=xterm-256color histui tui
```

### Timestamp showing wrong time

The dunst timestamp is microseconds since boot, not Unix time. Ensure boot time conversion is working:
```bash
# Check system uptime
cat /proc/uptime

# Compare with dunst timestamp
dunstctl history | jq '.data[0][0].timestamp.data'
```

### Launcher shows empty list

Check that notifications exist and are within the default 48h window:
```bash
# Check raw dunst history
dunstctl history | jq '.data | length'

# Check histui output without time filter
histui get --since 0 --format dmenu

# Check with verbose logging
histui --verbose get 2>&1 | head -20
```

### Notification lookup not finding match

When using content-based matching (no ULID), ensure the input matches the original format exactly:
```bash
# Use ULID for reliable matching
histui get --format dmenu --ulid | fuzzel -d | histui get --body

# Debug: see what histui receives
echo "your selection" | histui --verbose get --body 2>&1
```

---

## Next Steps

1. **Read the spec**: [spec.md](./spec.md) - Full requirements and user stories
2. **Review data model**: [data-model.md](./data-model.md) - Entity definitions
3. **Check research**: [research.md](./research.md) - Technology decisions
4. **Review contracts**: [contracts/interfaces.go](./contracts/interfaces.go) - Go interfaces
5. **Run `/speckit.tasks`**: Generate implementation task breakdown
6. **Start with model**: Implement `internal/model/notification.go` first
7. **Test-driven**: Write tests before implementations
