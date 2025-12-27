# Quickstart: histuid Development

**Feature**: histuid - Wayland Notification Daemon
**Date**: 2025-12-27

This guide provides step-by-step instructions for setting up the development environment and implementing histuid.

---

## Prerequisites

### System Dependencies

```bash
# Arch Linux
sudo pacman -S \
    gtk4 \
    libadwaita \
    gtk4-layer-shell \
    gobject-introspection \
    pkg-config \
    pipewire \
    pipewire-pulse

# Ubuntu/Debian (24.04+)
sudo apt install \
    libgtk-4-dev \
    libadwaita-1-dev \
    libgtk4-layer-shell-dev \
    libglib2.0-dev \
    pkg-config
```

### Go Version

Go 1.21 or later is required (for `slog`).

```bash
go version
# go version go1.21+ linux/amd64
```

---

## Project Setup

### 1. Clone and Enter Repository

```bash
cd /home/johnm/dtkr4-cnjjf/github.com/jmylchreest/histui
git checkout 002-wayland-notification-daemon
```

### 2. Install Go Dependencies

```bash
# Existing dependencies (already in go.mod)
go get github.com/spf13/cobra@latest
go get github.com/charmbracelet/bubbletea@latest
go get github.com/fsnotify/fsnotify@latest
go get github.com/pelletier/go-toml/v2@latest

# New dependencies for histuid
go get github.com/godbus/dbus/v5@latest
go get github.com/diamondburned/gotk4@latest
go get github.com/diamondburned/gotk4-adwaita@latest
go get github.com/diamondburned/gotk4-layer-shell@latest
go get github.com/gopxl/beep@latest

go mod tidy
```

### 3. Verify CGO Environment

```bash
# CGO must be enabled for histuid
export CGO_ENABLED=1

# Verify GTK4/libadwaita are findable
pkg-config --cflags --libs gtk4
pkg-config --cflags --libs libadwaita-1
pkg-config --cflags --libs gtk4-layer-shell
```

---

## Development Workflow

### Building

```bash
# Build histui CLI (CGO_ENABLED=0, existing behavior)
CGO_ENABLED=0 go build -o histui ./cmd/histui

# Build histuid daemon (requires CGO)
CGO_ENABLED=1 go build -o histuid ./cmd/histuid

# Or use Taskfile
task build
```

### Testing

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/dbus/...
go test ./internal/daemon/...
go test ./internal/core/...

# Run with verbose output
go test -v ./internal/core/filter_test.go
```

### Running Locally

```bash
# 1. Stop any running notification daemon
killall dunst mako swaync 2>/dev/null || true

# 2. Start histuid in foreground with debug logging
./histuid --debug

# 3. In another terminal, send test notifications
notify-send "Test" "Hello from histuid"
notify-send -u critical "Critical" "This is urgent!"

# 4. Verify history
./histui get --limit 5
```

---

## Implementation Order

Follow this order for incremental development:

### Phase 1: Core D-Bus Interface

1. **`internal/dbus/types.go`** - D-Bus type definitions
2. **`internal/dbus/server.go`** - NotificationServer struct and methods
3. **`cmd/histuid/main.go`** - Minimal daemon that claims bus name

**Test milestone**: `notify-send` works, notifications logged to console.

### Phase 2: Store Integration

4. **`internal/store/state.go`** - SharedState for DnD
5. **`internal/daemon/daemon.go`** - Main daemon orchestration
6. **Connect D-Bus server to existing store**

**Test milestone**: Notifications appear in `histui get` output.

### Phase 3: GTK4 + libadwaita + Layer Shell Display

7. **`internal/display/manager.go`** - Popup window manager
8. **`internal/display/popup.go`** - Individual popup windows (GTK4/libadwaita)
9. **`internal/display/widgets.go`** - Notification widget construction
10. **`internal/display/layout.go`** - Stacking and positioning

**Test milestone**: Popups appear on screen (basic libadwaita styling).

### Phase 4: Theming and Animated Images

11. **`internal/display/animated.go`** - GdkPixbufAnimation paintable wrapper
12. **`internal/theme/loader.go`** - CSS theme loading
13. **`internal/theme/default.go`** - Embedded default theme

**Test milestone**: Styled popups with CSS theming and animated GIF support.

### Phase 5: Audio and Hot-Reload

14. **`internal/audio/player.go`** - Sound playback
15. **`internal/daemon/hotreload.go`** - File watching for config/themes
16. **`internal/config/daemon.go`** - TOML config parsing

**Test milestone**: Sounds play, config changes apply without restart.

### Phase 6: CLI Extensions

17. **`cmd/histui/set.go`** - `histui set` command
18. **`cmd/histui/dnd.go`** - `histui dnd` command
19. **`internal/core/filter.go`** - Rich filter parsing
20. **`internal/adapter/output/ids.go`** - `--format ids` output

**Test milestone**: Full CLI workflow with pipelines.

---

## Key Implementation Patterns

### D-Bus Service Pattern

```go
// internal/dbus/server.go
func (s *Server) Notify(
    appName string,
    replacesID uint32,
    appIcon string,
    summary string,
    body string,
    actions []string,
    hints map[string]dbus.Variant,
    expireTimeout int32,
) (uint32, *dbus.Error) {
    // 1. Generate notification ID
    id := s.nextID.Add(1)

    // 2. Create histui notification model
    notif, _ := model.NewNotification("histuid")
    notif.ID = int(id)
    notif.AppName = appName
    notif.Summary = summary
    notif.Body = body
    notif.SetUrgency(extractUrgency(hints))

    // 3. Persist to store
    s.store.Add(*notif)

    // 4. Signal display manager (non-blocking)
    select {
    case s.displayChan <- notif:
    default:
    }

    return id, nil
}
```

### GTK4/libadwaita Main Loop Integration

```go
// cmd/histuid/main.go
func main() {
    app := adw.NewApplication("io.github.histui.daemon", gio.ApplicationFlagsNone)

    app.ConnectActivate(func() {
        // Initialize D-Bus server
        server := dbus.NewServer(store)

        // Initialize display manager (GTK4/libadwaita + layer-shell)
        display := display.NewManager(app)

        // Connect D-Bus notifications to display
        go func() {
            for notif := range server.DisplayChannel() {
                glib.IdleAdd(func() {
                    display.ShowNotification(notif)
                })
            }
        }()
    })

    app.Run(os.Args)
}
```

### Filter Expression Parsing

```go
// internal/core/filter.go
func ParseFilter(expr string) (*FilterExpr, error) {
    // Split on comma for AND conditions
    parts := strings.Split(expr, ",")
    conditions := make([]FilterCondition, 0, len(parts))

    for _, part := range parts {
        cond, err := parseCondition(part)
        if err != nil {
            return nil, err
        }
        conditions = append(conditions, cond)
    }

    return &FilterExpr{Conditions: conditions}, nil
}

func parseCondition(s string) (FilterCondition, error) {
    // Match: field operator value
    // Operators: = ~ > >= < <=
    re := regexp.MustCompile(`^(\w+)(=|~|>=|<=|>|<)(.+)$`)
    // ...
}
```

---

## Testing Checklist

Before each PR, verify:

- [ ] `go test ./...` passes
- [ ] `golangci-lint run` passes
- [ ] `go build ./cmd/histui` works (CGO_ENABLED=0)
- [ ] `go build ./cmd/histuid` works (CGO_ENABLED=1)
- [ ] `notify-send` works with histuid running
- [ ] `histui get` shows daemon-received notifications
- [ ] `histui dnd toggle` toggles DnD state
- [ ] Popups appear with correct styling
- [ ] Config changes hot-reload without restart

---

## Common Issues

### "Cannot claim bus name"

Another notification daemon is running:

```bash
dbus-send --session --print-reply \
    --dest=org.freedesktop.DBus /org/freedesktop/DBus \
    org.freedesktop.DBus.GetNameOwner string:org.freedesktop.Notifications
```

Kill competing daemon and retry.

### "GTK4 not found"

Ensure `pkg-config` paths are correct:

```bash
export PKG_CONFIG_PATH=/usr/lib/pkgconfig:/usr/share/pkgconfig
```

### "Layer shell not working"

Verify compositor supports wlr-layer-shell:

```bash
# Should list layer-shell protocol
wayland-info | grep layer
```

### "Animated GIF not playing"

Ensure GdkPixbuf animation support:

```bash
# Check if gdk-pixbuf supports GIF
gdk-pixbuf-query-loaders | grep gif
```

---

## Resources

- [freedesktop.org Notification Spec](https://specifications.freedesktop.org/notification/latest/)
- [godbus/dbus Documentation](https://pkg.go.dev/github.com/godbus/dbus/v5)
- [gotk4 Documentation](https://pkg.go.dev/github.com/diamondburned/gotk4)
- [gotk4-adwaita Documentation](https://pkg.go.dev/github.com/diamondburned/gotk4-adwaita)
- [gtk4-layer-shell](https://github.com/wmww/gtk4-layer-shell)
- [libadwaita](https://gnome.pages.gitlab.gnome.org/libadwaita/)
- [beep Audio Library](https://github.com/gopxl/beep)
