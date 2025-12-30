# Data Model: GitHub Pages Documentation Site

**Feature**: 003-github-pages-docs
**Date**: 2025-12-27

## Overview

This document defines the structure and content model for the histui/histuid documentation site. In a documentation context, the "data model" represents the organization of content, metadata, and navigation.

---

## Content Entities

### 1. Documentation Page

A single Markdown/MDX file representing one documentation topic.

**Frontmatter Schema**:
```yaml
---
title: "Page Title"                    # Required: H1 heading
description: "Brief description"       # Required: SEO meta description
sidebar_position: 1                    # Optional: Order in sidebar
sidebar_label: "Sidebar Label"         # Optional: Different label for nav
keywords: [keyword1, keyword2]         # Optional: Search keywords
---
```

**Relationships**:
- Belongs to one Section
- May reference other Pages (cross-links)
- May be versioned (copied to versioned_docs/)

### 2. Section

A folder containing related documentation pages with shared navigation.

**Structure**:
```
section-name/
├── _category_.json    # Section metadata
├── index.md           # Section landing page
└── *.md               # Individual pages
```

**_category_.json Schema**:
```json
{
  "label": "Section Name",
  "position": 1,
  "link": {
    "type": "generated-index",
    "description": "Section description"
  }
}
```

### 3. Version

A snapshot of all documentation at a specific release point.

**Attributes**:
- Version label (e.g., "1.0.0")
- Path prefix (e.g., "/docs/1.0.0/")
- Creation timestamp
- Source tag reference

**Storage**:
- `versioned_docs/version-X.Y.Z/` - Content snapshot
- `versioned_sidebars/version-X.Y.Z-sidebars.json` - Navigation snapshot
- `versions.json` - Version manifest

### 4. Navigation Sidebar

Hierarchical navigation structure for documentation sections.

**Schema** (sidebars.ts):
```typescript
const sidebars: SidebarsConfig = {
  docs: [
    'intro',
    {
      type: 'category',
      label: 'Section Name',
      items: ['section/page1', 'section/page2'],
    },
  ],
};
```

---

## Content Structure

### Site Map

```
/                           # Homepage (custom React page)
├── /docs/                  # Latest documentation
│   ├── /intro              # Overview
│   ├── /quickstart/        # Getting started
│   │   ├── /               # Combined quickstart
│   │   ├── /histui         # histui quickstart
│   │   └── /histuid        # histuid quickstart
│   ├── /histui/            # CLI tool
│   │   ├── /commands/      # Command reference
│   │   │   ├── /get
│   │   │   ├── /prune
│   │   │   ├── /status
│   │   │   └── /tui
│   │   └── /filtering      # Filter syntax
│   └── /histuid/           # Daemon
│       ├── /configuration
│       ├── /theming/       # CSS theming
│       │   ├── /           # Overview
│       │   ├── /css-reference
│       │   └── /examples
│       └── /monitor-mode
└── /docs/X.Y.Z/            # Versioned documentation
    └── [same structure]
```

### Navigation Hierarchy

```typescript
// sidebars.ts
const sidebars = {
  docs: [
    'intro',
    {
      type: 'category',
      label: 'Getting Started',
      items: [
        'quickstart/index',
        'quickstart/histui',
        'quickstart/histuid',
      ],
    },
    {
      type: 'category',
      label: 'histui CLI',
      items: [
        {
          type: 'category',
          label: 'Commands',
          items: [
            'histui/commands/get',
            'histui/commands/prune',
            'histui/commands/status',
            'histui/commands/tui',
          ],
        },
        'histui/filtering',
      ],
    },
    {
      type: 'category',
      label: 'histuid Daemon',
      items: [
        'histuid/configuration',
        {
          type: 'category',
          label: 'Theming',
          items: [
            'histuid/theming/index',
            'histuid/theming/css-reference',
            'histuid/theming/examples',
          ],
        },
        'histuid/monitor-mode',
      ],
    },
  ],
};
```

---

## Page Templates

### Command Reference Page

Template for CLI command documentation (e.g., `histui get`):

```markdown
---
title: "histui get"
description: "Query and output notification history"
sidebar_label: "get"
---

# histui get

Query notification history and output in various formats.

## Synopsis

\`\`\`bash
histui get [index|id] [flags]
\`\`\`

## Description

[Detailed description of what the command does]

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--app` | string | | Filter by application name |
| `--format` | string | dmenu | Output format (dmenu, json, plain) |

## Examples

\`\`\`bash
# List all notifications in dmenu format
histui get

# Filter by app and time
histui get --app firefox --since 1h
\`\`\`

## See Also

- [histui status](./status.md) - Status output
- [Filtering](../filtering.md) - Filter syntax
```

### Configuration Reference Page

Template for config file documentation:

```markdown
---
title: "Configuration"
description: "histuid configuration file reference"
---

# Configuration

histuid is configured via `~/.config/histui/histuid.toml`.

## Minimal Configuration

histuid works with no configuration file. The following shows all defaults:

\`\`\`toml
# All values shown are defaults - omit to use defaults
\`\`\`

## Reference

### [display]

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| position | string | "top-right" | Popup position |
| width | int | 350 | Popup width in pixels |

[Continue for each section...]
```

---

## Search Index Configuration

Algolia DocSearch crawler configuration (provided to Algolia):

```json
{
  "index_name": "histui",
  "start_urls": [
    "https://jmylchreest.github.io/histui/"
  ],
  "sitemap_urls": [
    "https://jmylchreest.github.io/histui/sitemap.xml"
  ],
  "selectors": {
    "lvl0": "article h1",
    "lvl1": "article h2",
    "lvl2": "article h3",
    "text": "article p, article li, article td"
  }
}
```

---

## Version Lifecycle

```
New Release Tag (vX.Y.Z)
        │
        ▼
┌───────────────────────┐
│ GitHub Action trigger │
└───────────────────────┘
        │
        ▼
┌───────────────────────┐
│ docusaurus docs:version │
│       X.Y.Z            │
└───────────────────────┘
        │
        ▼
┌───────────────────────┐
│ versioned_docs/       │
│ version-X.Y.Z/        │
│ created with snapshot │
└───────────────────────┘
        │
        ▼
┌───────────────────────┐
│ versions.json updated │
│ with new version      │
└───────────────────────┘
        │
        ▼
┌───────────────────────┐
│ Deploy to GitHub Pages│
│ All versions live     │
└───────────────────────┘
```

---

## Static Assets

### Images

| Path | Purpose |
|------|---------|
| `static/img/logo.svg` | Site logo |
| `static/img/favicon.ico` | Browser favicon |
| `static/img/screenshots/` | Feature screenshots |
| `static/img/diagrams/` | Architecture diagrams |

### Examples

| Path | Purpose |
|------|---------|
| `static/examples/histuid.toml` | Example config file |
| `static/examples/themes/` | Example theme CSS files |
