---
title: Layout Reference
description: Complete layout.xml element and attribute reference
sidebar_position: 6
---

# Layout Reference

The `layout.xml` file defines which widgets appear in notifications and how they're arranged. This gives you complete control over notification structure without writing CSS.

## File Location

Layout files can be placed in:

```
~/.config/histui/themes/mytheme/
├── layout.xml      # Custom layout
├── theme.css       # CSS styling
└── manifest.toml   # Metadata
```

If a theme doesn't include `layout.xml`, the default layout is used automatically.

## Basic Structure

Every layout starts with a `<popup>` root element:

```xml
<popup min-width="250" max-width="400" min-height="0" max-height="600">
  <!-- Child elements here -->
</popup>
```

## Popup Attributes

The `<popup>` element accepts sizing constraints:

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| `min-width` | int | 300 | Minimum popup width in pixels |
| `max-width` | int | 450 | Maximum popup width in pixels |
| `min-height` | int | 0 | Minimum popup height in pixels |
| `max-height` | int | 900 | Maximum popup height in pixels |

Values can be specified with or without `px` suffix:

```xml
<popup min-width="300" max-width="400px">
```

### Sizing Behavior

- **Dynamic width**: Set different min/max values for content-responsive width
- **Fixed width**: Set min-width = max-width for a fixed size
- **Dynamic height**: Use `min-height="0"` to let content determine height
- **Scrolling**: Content exceeding max-height becomes scrollable

## Available Elements

### Content Elements

These elements display notification content:

| Element | Description | CSS Class |
|---------|-------------|-----------|
| `<summary />` | Notification title | `.notification-summary` |
| `<body />` | Notification message body | `.notification-body` |
| `<appname />` | Application name | `.notification-appname` |
| `<timestamp />` | Time since notification | `.notification-timestamp` |
| `<icon />` | Application icon (with Nerd Font fallback) | `.notification-icon` |
| `<image />` | Notification image attachment | `.notification-image` |
| `<progress />` | Progress bar (if hint provided) | `.notification-progress` |
| `<actions />` | Action buttons | `.notification-actions` |
| `<stack-count />` | Badge showing stacked notification count | `.notification-stack-count` |

### Container Elements

These elements group other elements:

| Element | Description | CSS Class |
|---------|-------------|-----------|
| `<header>` | Horizontal container for header elements | `.notification-header` |
| `<box>` | Generic container with configurable orientation | (none) |

## Element Attributes

### Icon Attributes

```xml
<icon size="48" />
```

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| `size` | int | 48 | Icon size in pixels |

The icon element displays:
1. Application-provided icon (if available)
2. GTK icon theme icon (if app name matches)
3. Nerd Font symbol (fallback, styled with `.notification-icon-nerd`)

### Image Element

```xml
<image />
```

The `<image />` element displays notification images from `image-data` or `image-path` hints. It has special sizing and cropping behavior:

**Sizing Rules:**
1. Images scale down to fit the popup width (minus padding)
2. Images never scale up beyond their original size
3. Tall images are cropped to max 150px height, showing the top portion
4. Cropped images display a fade gradient at the bottom to indicate truncation

**Omitting Images:**

To create a compact layout that never displays images, simply omit the `<image />` element:

```xml
<!-- Minimal layout - no images -->
<popup min-width="150" max-width="250">
  <summary />
  <body />
</popup>
```

This is useful for text-focused themes like `minimal` and `compact`.

**CSS Classes:**

| Class | Description |
|-------|-------------|
| `.notification-image` | The image itself |
| `.notification-image-container` | Wrapper for cropped images |
| `.notification-image-container.cropped` | Added when image is cropped |
| `.notification-image-fade` | Gradient overlay on cropped images |

### Box Attributes

```xml
<box orientation="vertical">
  <summary />
  <appname />
</box>
```

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| `orientation` | string | horizontal | Layout direction: `vertical` or `horizontal` |

## Element Order

**Element order in the XML determines visual order in the notification.**

For horizontal containers (like `<header>`), elements appear left-to-right in the order they're listed:

```xml
<!-- Icon on left, text in middle, stack-count on right -->
<header>
  <icon size="48" />
  <summary />
  <stack-count />
</header>

<!-- Icon on right -->
<header>
  <summary />
  <stack-count />
  <icon size="24" />
</header>
```

For vertical containers, elements appear top-to-bottom.

## Layout Examples

### Default Layout

The standard layout with icon, summary, app name, body, and actions:

```xml
<popup min-width="250" max-width="400" min-height="0" max-height="600">
  <header>
    <icon size="48" />
    <box orientation="vertical">
      <summary />
      <appname />
    </box>
    <stack-count />
  </header>
  <body />
  <progress />
  <image />
  <actions />
</popup>
```

### Minimal Layout

Text-only notifications without icons:

```xml
<popup min-width="150" max-width="250" min-height="0" max-height="200">
  <summary />
  <body />
  <progress />
</popup>
```

### Compact Layout

Smaller icon, right-aligned, minimal elements:

```xml
<popup min-width="180" max-width="280" min-height="0" max-height="300">
  <header>
    <summary />
    <stack-count />
    <icon size="24" />
  </header>
  <body />
  <progress />
  <actions />
</popup>
```

### Detailed Layout

Full layout with timestamp:

```xml
<popup min-width="250" max-width="450" min-height="0" max-height="700">
  <header>
    <icon size="48" />
    <box orientation="vertical">
      <summary />
      <box orientation="horizontal">
        <appname />
        <timestamp />
      </box>
    </box>
    <stack-count />
  </header>
  <body />
  <progress />
  <image />
  <actions />
</popup>
```

### Media-Focused Layout

Large image area, minimal text:

```xml
<popup min-width="300" max-width="400" min-height="0" max-height="500">
  <image />
  <header>
    <icon size="32" />
    <summary />
  </header>
  <body />
  <actions />
</popup>
```

### Chat-Style Layout

Optimized for messaging apps:

```xml
<popup min-width="280" max-width="380" min-height="0" max-height="400">
  <header>
    <icon size="40" />
    <box orientation="vertical">
      <appname />
      <summary />
    </box>
    <timestamp />
  </header>
  <body />
  <image />
  <actions />
</popup>
```

## Conditional Display

Elements are automatically hidden when their content is empty:

- `<body />` - Hidden if notification has no body text
- `<progress />` - Hidden if no progress hint is set
- `<image />` - Hidden if no image is attached
- `<actions />` - Hidden if no action buttons are defined
- `<stack-count />` - Hidden if notification isn't stacked
- `<timestamp />` - Always displays relative time ("2m ago")

You don't need to handle these cases in your layout.

## Nesting Elements

Elements can be nested in containers for complex layouts:

```xml
<header>
  <icon size="48" />
  <box orientation="vertical">
    <summary />
    <box orientation="horizontal">
      <appname />
      <timestamp />
    </box>
  </box>
  <stack-count />
</header>
```

This creates:
```
[icon] | [summary          ] | [stack-count]
       | [appname] [time   ] |
```

## Layout and CSS Interaction

The layout defines structure; CSS defines appearance. They work together:

**Layout** (what elements exist and where):
```xml
<header>
  <summary />
  <icon size="24" />  <!-- Icon after summary = visually on right -->
</header>
```

**CSS** (how elements look):
```css
/* Right-aligned icon needs left margin, not right */
.notification-icon {
    margin-left: 8px;
    margin-right: 0;
}
```

## Default Fallback

If your theme doesn't include `layout.xml`, histuid uses this default:

```xml
<popup min-width="300" max-width="450" min-height="0" max-height="900">
  <header>
    <icon size="48" />
    <box orientation="vertical">
      <summary />
      <appname />
    </box>
    <stack-count />
  </header>
  <body />
  <progress />
  <image />
  <actions />
</popup>
```

## Validation

histuid validates layout XML at load time. Invalid elements produce an error:

```
unknown element type: invalid-element
```

Only elements listed in [Available Elements](#available-elements) are valid.

## Tips

1. **Start with a bundled layout** - Copy from an embedded theme and modify
2. **Test with different content** - Long titles, multi-line bodies, images, actions
3. **Consider urgency** - Critical notifications may need different emphasis
4. **Icon size affects layout** - Larger icons need more header height
5. **Use timestamps sparingly** - They add visual noise

## See Also

- [CSS Reference](/docs/histuid/theming/css-reference) - Styling layout elements
- [Manifest Reference](/docs/histuid/theming/manifest-reference) - Theme metadata and audio
- [Advanced Theming](/docs/histuid/theming/advanced) - Complete theme examples
