---
title: Installation
description: Install histui and histuid on your system
sidebar_position: 2
---

# Installation

This guide covers all installation methods for histui and histuid.

## Prerequisites

### histui CLI

The CLI tool is a pure Go binary with no external dependencies:
- Linux (any distro)
- D-Bus session bus

### histuid Daemon

The daemon requires GTK4 for rendering notifications:
- **Wayland compositor** with layer-shell support (Hyprland, Sway, river, etc.)
- **GTK4** runtime libraries
- D-Bus session bus

**Arch Linux:**
```bash
sudo pacman -S gtk4 glib2
```

**Debian/Ubuntu:**
```bash
sudo apt install libgtk-4-1 libglib2.0-0
```

**Fedora:**
```bash
sudo dnf install gtk4 glib2
```

## Arch Linux (AUR)

### Binary Package (Recommended)

The fastest option - downloads pre-built binaries:

```bash
# Using an AUR helper
yay -S histui-bin

# Or manually
git clone https://aur.archlinux.org/histui-bin.git
cd histui-bin
makepkg -si
```

### Build from Source

Builds from source with full optimization:

```bash
# Using an AUR helper
yay -S histui

# Or manually
git clone https://aur.archlinux.org/histui.git
cd histui
makepkg -si
```

Both packages install:
- `/usr/bin/histui` - CLI tool
- `/usr/bin/histuid` - Notification daemon
- `/usr/lib/systemd/user/histuid.service` - Standalone systemd service
- `/usr/lib/systemd/user/histuid-monitor.service` - Monitor mode service

## Pre-built Binaries

Download binaries directly from [GitHub Releases](https://github.com/jmylchreest/histui/releases).

```bash
# Download latest release
VERSION="v0.1.0"  # Check releases for latest version
curl -LO "https://github.com/jmylchreest/histui/releases/download/${VERSION}/histui-linux-amd64"
curl -LO "https://github.com/jmylchreest/histui/releases/download/${VERSION}/histuid-linux-amd64"

# Install
chmod +x histui-linux-amd64 histuid-linux-amd64
sudo mv histui-linux-amd64 /usr/local/bin/histui
sudo mv histuid-linux-amd64 /usr/local/bin/histuid
```

## Building from Source

### Requirements

- **Go 1.21+** (required for stdlib `slog`)
- **GTK4 development libraries** (for histuid)
- **pkg-config**

**Arch Linux:**
```bash
sudo pacman -S go gtk4 glib2 pkg-config
```

**Debian/Ubuntu:**
```bash
sudo apt install golang libgtk-4-dev libglib2.0-dev pkg-config
```

### Build

```bash
# Clone the repository
git clone https://github.com/jmylchreest/histui.git
cd histui

# Build histui CLI (no CGO needed)
CGO_ENABLED=0 go build -ldflags "-s -w" -o histui ./cmd/histui

# Build histuid daemon (requires GTK4)
CGO_ENABLED=1 go build -ldflags "-s -w" -o histuid ./cmd/histuid

# Install
sudo mv histui histuid /usr/local/bin/
```

### Using Taskfile

If you have [Task](https://taskfile.dev) installed:

```bash
# Build everything
task build

# Or just the binaries you need
task build:histui
task build:histuid
```

## systemd User Services

### Standalone Mode

Run histuid as your primary notification daemon:

```bash
# Create the service directory
mkdir -p ~/.config/systemd/user

# Create the service file
cat > ~/.config/systemd/user/histuid.service << 'EOF'
[Unit]
Description=histuid notification daemon
Documentation=https://github.com/jmylchreest/histui
PartOf=graphical-session.target
After=graphical-session.target

[Service]
Type=simple
ExecStart=/usr/local/bin/histuid
Restart=on-failure
RestartSec=5

[Install]
WantedBy=graphical-session.target
EOF

# Enable and start
systemctl --user daemon-reload
systemctl --user enable --now histuid.service

# Check status
systemctl --user status histuid
```

### Monitor Mode

Run histuid alongside another notification daemon (like dunst) for history only:

```bash
cat > ~/.config/systemd/user/histuid-monitor.service << 'EOF'
[Unit]
Description=histuid notification daemon (monitor mode)
Documentation=https://github.com/jmylchreest/histui
PartOf=graphical-session.target
After=graphical-session.target
After=dunst.service

[Service]
Type=simple
ExecStart=/usr/local/bin/histuid --monitor
Restart=on-failure
RestartSec=5

[Install]
WantedBy=graphical-session.target
EOF

# Enable and start
systemctl --user daemon-reload
systemctl --user enable --now histuid-monitor.service
```

:::warning Service Conflict
Don't enable both `histuid.service` and `histuid-monitor.service` at the same time.
:::

## Compositor Auto-start

Instead of systemd, you can start histuid from your compositor config.

### Hyprland

Add to `~/.config/hypr/hyprland.conf`:

```ini
exec-once = histuid
```

### Sway

Add to `~/.config/sway/config`:

```ini
exec histuid
```

## Verifying Installation

```bash
# Check versions
histui --version
histuid --version

# Send a test notification
notify-send "Test" "Installation successful!"

# Check it was captured
histui get --limit 1
```

## Troubleshooting

### histuid won't start

**Check GTK4 is installed:**
```bash
pkg-config --modversion gtk4
```

**Check for existing notification daemons:**
```bash
# Only one daemon can own the notification D-Bus name
pgrep -a 'dunst|mako|swaync|fnott'
```

### Notifications not appearing

**Check histuid is running:**
```bash
pgrep -a histuid
```

**Check logs:**
```bash
journalctl --user -u histuid -f
```

**Test D-Bus:**
```bash
notify-send --help  # Should work if D-Bus is functional
```

## Next Steps

- [Quickstart](/docs/quickstart) - Get started in 5 minutes
- [Configuration](/docs/histuid/configuration) - Configure histuid
- [Desktop Integration](/docs/histuid/integration) - Waybar, keybindings
