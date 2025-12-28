---
title: histui apps
description: List unique application names from history
sidebar_label: apps
sidebar_position: 7
---

# histui apps

List unique application names from notification history.

## Synopsis

```bash
histui apps [flags]
```

## Description

The `apps` command lists all unique application names that have sent notifications
in your history. Useful for shell completion, scripting, or discovering what apps
are in your notification history.

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--source` | string | auto | Notification source: `histuid`, `dunst`, `stdin` |

## Examples

### List all apps

```bash
histui apps
```

Output:
```
discord
firefox
slack
spotify
telegram
```

### Use for shell completion

```bash
# Select app with fzf
histui get --app "$(histui apps | fzf)"

# Combine with get for filtering
APP=$(histui apps | fzf)
histui get --app "$APP" | fuzzel -d
```

### Count notifications per app

```bash
# Using histui get with jq
histui get --format json | jq -r '.[].app_name' | sort | uniq -c | sort -rn
```

### Check if an app exists in history

```bash
if histui apps | grep -q "discord"; then
  echo "Discord notifications found"
fi
```

## See Also

- [histui get](/docs/histui/commands/get) - Query notifications with `--app` filter
- [Filtering Guide](/docs/histui/filtering) - Advanced filter syntax
