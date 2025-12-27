# Research: histuid Technical Decisions

**Date**: 2025-12-27
**Feature**: histuid - Wayland Notification Daemon

## 1. Wayland Layer-Shell Support

### Decision: gotk4 + gotk4-layer-shell

Use `github.com/diamondburned/gotk4` for GTK4 bindings and `github.com/diamondburned/gotk4-layer-shell` for wlr-layer-shell protocol support.

### Rationale

- **Production Ready**: gotk4 is actively maintained by diamondburned with regular updates
- **GTK4 Layer-Shell**: Provides complete bindings for gtk4-layer-shell C library
- **WebView Compatibility**: GTK4 integrates with WebKitGTK for rich content rendering
- **Compositor Support**: Works with all wlroots-based compositors (Hyprland, Sway, etc.)
- **Community Adoption**: Used by other Go Wayland applications

### Alternatives Considered

| Option | Rejected Because |
|--------|------------------|
| rajveermalviya/go-wayland | Archived/unmaintained; requires manual protocol handling |
| dlasky/gotk3-layershell | GTK3 is legacy; webkit2gtk4 better integrates with GTK4 |
| Pure Wayland (no GTK) | Would require reimplementing widget toolkit; excessive scope |
| wlroots bindings | Low-level; would need to build UI layer from scratch |

### Build Requirements

```bash
# Runtime/build dependencies
sudo pacman -S gtk4 gtk4-layer-shell webkit2gtk-4.1

# Go dependencies
go get github.com/diamondburned/gotk4@latest
go get github.com/diamondburned/gotk4-layer-shell@latest
```

### CGO Requirement

`CGO_ENABLED=1` is required. This is a justified deviation from the constitution's preference for static binaries because:
1. No pure-Go Wayland layer-shell implementation exists
2. WebKit rendering requires C bindings
3. GTK4 is the standard toolkit for Linux desktop applications

---

## 2. Notification Rendering

### Decision: Pure GTK4 + libadwaita (No WebKit)

Use native GTK4 widgets with libadwaita styling for notification popups. No WebKit/WebView dependency.

### Rationale

- **Reduced Attack Surface**: WebKit is a full browser engine with millions of LOC; GTK4 widgets are much simpler
- **Fewer Dependencies**: No `webkit2gtk` or `webkitgtk-6.0` package required
- **GTK4 CSS Support**: GTK4 has excellent native CSS theming capabilities
- **libadwaita Integration**: Modern GNOME styling with rounded corners, shadows, animations
- **Animated GIF Support**: GdkPixbufAnimation + custom GdkPaintable wrapper
- **Lower Memory**: No WebKit process overhead
- **Faster Startup**: No WebKit initialization

### Alternatives Considered

| Option | Rejected Because |
|--------|------------------|
| gotk4-webkitgtk | Unclear GTK4 support; WebKit adds attack surface |
| malivvan/webkitgtk | Pre-release; WebKit overhead for simple notifications |
| webview/webview | GTK3 only; separate window management |

### Architecture

```go
// Notification popup structure using GTK4/libadwaita
popup := adw.NewWindow()
layershell.InitForWindow(popup)
layershell.SetLayer(popup, layershell.LayerTop)

// Content box with CSS styling
box := gtk.NewBox(gtk.OrientationHorizontal, 12)
box.AddCSSClass("notification")
box.AddCSSClass("urgency-normal")

// Icon (supports animated GIFs via custom paintable)
icon := gtk.NewPicture()
icon.SetPaintable(animatedPaintable)

// Text content
textBox := gtk.NewBox(gtk.OrientationVertical, 4)
summary := gtk.NewLabel(notification.Summary)
summary.AddCSSClass("summary")
body := gtk.NewLabel(notification.Body)
body.AddCSSClass("body")

textBox.Append(summary)
textBox.Append(body)
box.Append(icon)
box.Append(textBox)
popup.SetContent(box)
```

### Animated GIF Support

GTK4 supports animated GIFs via `GdkPixbufAnimation`:

```go
// Load animated GIF from file or bytes
animation, _ := gdkpixbuf.NewPixbufAnimationFromFile(path)

// Or from base64/bytes (e.g., from D-Bus image-data hint)
stream := gio.NewMemoryInputStreamFromBytes(gifBytes)
animation, _ := gdkpixbuf.NewPixbufAnimationFromStream(stream, nil)

// Wrap in custom GdkPaintable for GtkPicture
paintable := NewAnimatedPaintable(animation)
picture.SetPaintable(paintable)
```

Preloading strategy:
- Load app-specific icons from `~/.config/histui/icons/` on startup
- Cache frequently-used animations in memory
- Decode D-Bus `image-data` hints on-demand

### Build Requirements

```bash
# GTK4 + libadwaita + layer-shell (no WebKit needed)
sudo pacman -S gtk4 libadwaita gtk4-layer-shell

# Ubuntu/Debian
sudo apt install libgtk-4-dev libadwaita-1-dev libgtk4-layer-shell-dev
```

---

## 3. Audio Playback

### Decision: beep library

Use `github.com/gopxl/beep` for audio playback with format decoders.

### Rationale

- **Format Support**: WAV, MP3, OGG/Vorbis, FLAC via separate decoder packages
- **Non-blocking**: Uses speaker package for background playback via goroutines
- **Volume Control**: Built-in volume effects via `beep/effects`
- **Low Latency**: Suitable for notification sounds (<50ms startup)
- **Pure Go Decoders**: No CGO required for audio (WAV/OGG are pure Go)
- **Well Maintained**: Active fork at gopxl/beep with ongoing development

### Alternatives Considered

| Option | Rejected Because |
|--------|------------------|
| oto | Lower-level; would need to implement format decoding manually |
| CGO to PipeWire/PulseAudio | Adds complexity; beep already uses oto which connects to audio server |
| SDL2 audio | Heavy dependency for simple sound playback |

### Volume Control Approach

```go
// Volume is 0.0 to 1.0, configured in histuid.toml
volume := effects.Volume{
    Streamer: decoder,
    Base:     2,
    Volume:   math.Log2(configVolume), // Convert 0-1 to dB scale
}
speaker.Play(volume)
```

### Format Support Details

| Format | Package | CGO Required |
|--------|---------|--------------|
| WAV | beep/wav | No |
| OGG/Vorbis | beep/vorbis | No |
| MP3 | beep/mp3 | No |
| FLAC | beep/flac | No |

### Build Requirements

```go
import (
    "github.com/gopxl/beep"
    "github.com/gopxl/beep/speaker"
    "github.com/gopxl/beep/wav"
    "github.com/gopxl/beep/vorbis"
    "github.com/gopxl/beep/mp3"
    "github.com/gopxl/beep/effects"
)
```

---

## 4. D-Bus Service Implementation

### Decision: godbus/dbus/v5

Use `github.com/godbus/dbus/v5` for implementing the org.freedesktop.Notifications interface.

### Rationale

- **Mature Library**: Widely used, well-documented, stable API
- **Server Support**: Full support for exporting objects and claiming bus names
- **Signal Emission**: Built-in support for emitting D-Bus signals
- **Type System**: Proper Go<->D-Bus type mapping including variants for hints
- **No CGO**: Pure Go implementation

### Key Patterns

**1. Claiming the Bus Name:**

```go
conn, _ := dbus.ConnectSessionBus()
reply, _ := conn.RequestName(
    "org.freedesktop.Notifications",
    dbus.NameFlagDoNotQueue|dbus.NameFlagReplaceExisting,
)
if reply != dbus.RequestNameReplyPrimaryOwner {
    return errors.New("another notification daemon is running")
}
```

**2. Exporting the Interface:**

```go
type NotificationServer struct {
    conn *dbus.Conn
    // ... state
}

// Method: Notify
func (s *NotificationServer) Notify(
    appName string,
    replacesID uint32,
    appIcon string,
    summary string,
    body string,
    actions []string,
    hints map[string]dbus.Variant,
    expireTimeout int32,
) (uint32, *dbus.Error) {
    // Implementation
    return notificationID, nil
}

// Export to D-Bus
conn.Export(server, "/org/freedesktop/Notifications", "org.freedesktop.Notifications")
conn.Export(introspect.Introspectable(introspectXML), "/org/freedesktop/Notifications",
    "org.freedesktop.DBus.Introspectable")
```

**3. Emitting Signals:**

```go
// NotificationClosed signal
conn.Emit(
    "/org/freedesktop/Notifications",
    "org.freedesktop.Notifications.NotificationClosed",
    notificationID,
    closeReason,
)

// ActionInvoked signal
conn.Emit(
    "/org/freedesktop/Notifications",
    "org.freedesktop.Notifications.ActionInvoked",
    notificationID,
    actionKey,
)
```

**4. Handling Hints (Variants):**

```go
func extractUrgency(hints map[string]dbus.Variant) int {
    if v, ok := hints["urgency"]; ok {
        if urgency, ok := v.Value().(byte); ok {
            return int(urgency)
        }
    }
    return 1 // Normal
}
```

### Introspection XML

```xml
<!DOCTYPE node PUBLIC "-//freedesktop//DTD D-BUS Object Introspection 1.0//EN"
 "http://www.freedesktop.org/standards/dbus/1.0/introspect.dtd">
<node>
  <interface name="org.freedesktop.Notifications">
    <method name="GetCapabilities">
      <arg direction="out" type="as"/>
    </method>
    <method name="Notify">
      <arg direction="in" type="s" name="app_name"/>
      <arg direction="in" type="u" name="replaces_id"/>
      <arg direction="in" type="s" name="app_icon"/>
      <arg direction="in" type="s" name="summary"/>
      <arg direction="in" type="s" name="body"/>
      <arg direction="in" type="as" name="actions"/>
      <arg direction="in" type="a{sv}" name="hints"/>
      <arg direction="in" type="i" name="expire_timeout"/>
      <arg direction="out" type="u"/>
    </method>
    <method name="CloseNotification">
      <arg direction="in" type="u" name="id"/>
    </method>
    <method name="GetServerInformation">
      <arg direction="out" type="s" name="name"/>
      <arg direction="out" type="s" name="vendor"/>
      <arg direction="out" type="s" name="version"/>
      <arg direction="out" type="s" name="spec_version"/>
    </method>
    <signal name="NotificationClosed">
      <arg type="u" name="id"/>
      <arg type="u" name="reason"/>
    </signal>
    <signal name="ActionInvoked">
      <arg type="u" name="id"/>
      <arg type="s" name="action_key"/>
    </signal>
  </interface>
</node>
```

### Concurrency

- godbus is thread-safe for method calls and signal emission
- Use Go channels to communicate between D-Bus handlers and main event loop
- GTK4 main loop integration via `glib.IdleAdd()` for UI updates

---

## 5. File Watching

### Decision: fsnotify (existing dependency)

Continue using `github.com/fsnotify/fsnotify` which is already a project dependency.

### Rationale

- **Already Used**: histui already uses fsnotify for store watching
- **Cross-Platform**: Works on Linux via inotify
- **Directory Watching**: Can watch entire directories for theme/audio file changes
- **Event Debouncing**: Can be easily debounced for hot-reload

### Hot-Reload Pattern

```go
watcher, _ := fsnotify.NewWatcher()

// Watch config, themes, and audio directories
watcher.Add(configPath)
watcher.Add(themesDir)
watcher.Add(audioDir)

// Debounced reload
var debounceTimer *time.Timer
for event := range watcher.Events {
    if debounceTimer != nil {
        debounceTimer.Stop()
    }
    debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
        switch {
        case strings.Contains(event.Name, "histuid.toml"):
            reloadConfig()
        case strings.HasSuffix(event.Name, ".css"):
            reloadTheme()
        case isAudioFile(event.Name):
            reloadAudio()
        }
    })
}
```

---

## 6. freedesktop.org Notification Specification Reference

### D-Bus Interface: org.freedesktop.Notifications

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| GetCapabilities | `() → as` | Returns array of capability strings |
| Notify | `(susssasa{sv}i) → u` | Send notification, returns ID |
| CloseNotification | `(u) → ()` | Force close notification |
| GetServerInformation | `() → ssss` | Returns name, vendor, version, spec_version |

**Signals:**

| Signal | Signature | Description |
|--------|-----------|-------------|
| NotificationClosed | `(uu)` | ID and reason code |
| ActionInvoked | `(us)` | ID and action key |
| ActivationToken | `(us)` | ID and activation token (v1.2+) |

**Close Reason Codes:**

| Code | Meaning |
|------|---------|
| 1 | Expired |
| 2 | Dismissed by user |
| 3 | Closed via CloseNotification |
| 4 | Undefined/reserved |

**Urgency Levels:**

| Value | Level | Typical Behavior |
|-------|-------|------------------|
| 0 | Low | Can be delayed, no sound |
| 1 | Normal | Standard notification |
| 2 | Critical | Never expires, always show |

**Standard Hints:**

| Hint | Type | Purpose |
|------|------|---------|
| urgency | byte | 0=low, 1=normal, 2=critical |
| category | string | Notification category (e.g., "email.arrived") |
| desktop-entry | string | Application .desktop file |
| image-data | (iiibiiay) | Raw image data |
| image-path | string | Path to image file |
| sound-file | string | Path to sound file |
| sound-name | string | Named sound from spec |
| suppress-sound | boolean | Suppress audio |
| transient | boolean | Don't persist |
| x, y | int32 | Position hints |
| action-icons | boolean | Actions are icon names |
| resident | boolean | Don't auto-remove after action |

**Capabilities to Advertise:**

```go
[]string{
    "actions",           // Support notification actions
    "body",              // Support body text
    "body-hyperlinks",   // Support hyperlinks in body
    "body-images",       // Support <img> in body
    "body-markup",       // Support HTML markup in body
    "icon-static",       // Support static icons
    "persistence",       // Persist notifications
    "sound",             // Play sounds
    "x-histui-rich",     // Vendor: rich CSS theming
}
```

---

## Summary: Technology Stack

| Component | Library | CGO | Notes |
|-----------|---------|-----|-------|
| Wayland Popups | gotk4 + gotk4-layer-shell | Yes | GTK4 with layer-shell |
| UI Styling | gotk4-adwaita (libadwaita) | Yes | Modern GNOME theming |
| Notification Widgets | GTK4 native (GtkBox, GtkLabel, GtkPicture) | Yes | CSS-styled widgets |
| Animated Images | GdkPixbufAnimation + custom Paintable | Yes | GIF, APNG support |
| Audio Playback | gopxl/beep | No | WAV, OGG, MP3 support |
| D-Bus | godbus/dbus/v5 | No | Service implementation |
| File Watching | fsnotify | No | Hot-reload support |
| Config Parsing | pelletier/go-toml/v2 | No | Already used by histui |

### Build Command

```bash
CGO_ENABLED=1 go build -o histuid ./cmd/histuid
```

### Runtime Dependencies

```bash
# Arch Linux
sudo pacman -S gtk4 libadwaita gtk4-layer-shell

# Ubuntu/Debian
sudo apt install libgtk-4-dev libadwaita-1-dev libgtk4-layer-shell-dev
```

### Security Improvement

By using pure GTK4/libadwaita instead of WebKit:
- No browser engine attack surface
- No JavaScript execution context
- No web content parsing vulnerabilities
- Simpler dependency chain for security audits
