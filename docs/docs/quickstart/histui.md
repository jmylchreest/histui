---
title: histui CLI Quickstart
description: Install and use the histui command-line interface
sidebar_position: 2
---

# histui CLI Quickstart

Install and start using the histui CLI to query notification history.

## Installation

### From Source

```bash
git clone https://github.com/jmylchreest/histui.git
cd histui
go build -o histui ./cmd/histui
sudo mv histui /usr/local/bin/
```

### From AUR (Arch Linux)

```bash
# Coming soon
```

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
