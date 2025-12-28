---
title: histui CLI Quickstart
description: Install and use the histui command-line interface
sidebar_position: 2
---

# histui CLI Quickstart

Start using the histui CLI to query notification history.

:::tip Installation
See the [Installation Guide](/docs/installation) for all installation methods including AUR, pre-built binaries, and building from source.
:::

## Basic Usage

### View Recent Notifications

```bash
histui get
```

### Filter by Application

```bash
histui get --app firefox
```

### JSON Output

```bash
histui get --format json
```

## Next Steps

- [Command Reference](/docs/histui/commands/get) - Full command documentation
- [Filtering Guide](/docs/histui/filtering) - Advanced filter syntax
