---
title: Introduction
description: What is histui and histuid?
sidebar_position: 1
slug: /
---

# What is histui?

**histui** is a notification history browser for Linux desktops.
It provides tools to view, search, and manage your desktop notification history.

## Components

The histui project includes two main components:

### histui CLI

A command-line interface for querying and managing notification history.
Use it to:

- View recent notifications in various formats (dmenu, JSON, plain text)
- Filter notifications by app, urgency, time, and more
- Prune old notifications from history
- Get notification counts for status bars like Waybar

### histuid Daemon

A notification daemon that captures and displays desktop notifications.
Features include:

- GTK4-based notification popups with rich theming support
- Notification history persistence
- Monitor mode for running alongside other notification daemons
- Do-not-disturb functionality

## Quick Links

- [Quickstart Guide](/docs/quickstart) - Get started in under 5 minutes
- [histui Commands](/docs/histui/commands/get) - CLI reference
- [histuid Configuration](/docs/histuid/configuration) - Daemon setup
- [Theming Guide](/docs/histuid/theming) - Customize notification appearance
