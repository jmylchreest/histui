---
title: histuid Daemon Quickstart
description: Install and configure the histuid notification daemon
sidebar_position: 3
---

# histuid Daemon Quickstart

Install and configure histuid to capture and display desktop notifications.

## Prerequisites

- GTK4 development libraries
- D-Bus session bus

### Installing GTK4 (Arch Linux)

```bash
sudo pacman -S gtk4
```

### Installing GTK4 (Debian/Ubuntu)

```bash
sudo apt install libgtk-4-dev
```

## Installation

### From Source

```bash
git clone https://github.com/jmylchreest/histui.git
cd histui
CGO_ENABLED=1 go build -o histuid ./cmd/histuid
sudo mv histuid /usr/local/bin/
```

## Starting the Daemon

```bash
histuid
```

histuid will start capturing notifications and displaying popups.

## Configuration

histuid works with sensible defaults.
Create a configuration file for customization:

```bash
mkdir -p ~/.config/histui
touch ~/.config/histui/histuid.toml
```

See [Configuration Reference](/docs/histuid/configuration) for all options.

## Next Steps

- [Desktop Integration](/docs/histuid/integration) - Auto-start with Hyprland, Sway, systemd
- [Configuration](/docs/histuid/configuration) - All configuration options
- [Theming](/docs/histuid/theming) - Customize notification appearance
- [Monitor Mode](/docs/histuid/monitor-mode) - Run alongside dunst
