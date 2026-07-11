---
title: Unreleased
description: Changes in development
sidebar_position: 1
---

# Unreleased

Changes that will be included in the next release.

## Added

### Wayland Layer Configuration

New `display.layer` setting to control which Wayland layer-shell layer notifications appear on:

```toml
[display]
layer = "top"  # default - above normal windows
# layer = "overlay"  # above everything, including lock screens
```

Valid values: `background`, `bottom`, `top`, `overlay`.

Use `overlay` to keep notifications visible during fullscreen windows (note: also appears above lock screens).
