# histui Documentation

This directory contains the documentation site for histui, built with [Docusaurus](https://docusaurus.io/).

## Quick Start

```bash
# Install dependencies
npm install

# Start development server (hot reload)
npm start

# Build for production
npm run build

# Serve production build locally
npm run serve
```

## Project Structure

```
docs/
├── docs/                 # Documentation content (MDX/Markdown)
│   ├── intro.md         # Introduction page
│   ├── quickstart/      # Getting started guides
│   ├── histui/          # histui CLI documentation
│   └── histuid/         # histuid daemon documentation
├── src/
│   ├── css/             # Custom styles
│   └── pages/           # Custom React pages (homepage)
├── static/              # Static assets (images, examples)
├── docusaurus.config.ts # Site configuration
└── sidebars.ts          # Sidebar navigation
```

## Development

### Starting the Dev Server

```bash
npm start
```

This starts a local development server at http://localhost:3000/histui/. Most changes are reflected live without restarting.

### Building

```bash
npm run build
```

Generates static content into the `build` directory. Use `npm run serve` to test the production build locally.

### Search

Local search (via `@cmfcmf/docusaurus-search-local`) only works on the production build:

```bash
npm run build
npm run serve
```

Search won't function during `npm start`.

## Writing Documentation

### Markdown Features

- Standard Markdown with GitHub Flavored Markdown support
- MDX for embedding React components
- [Admonitions](https://docusaurus.io/docs/markdown-features/admonitions) for notes, warnings, tips
- Code blocks with syntax highlighting

### Frontmatter

Each page should include frontmatter:

```markdown
---
title: Page Title
description: Brief description for SEO
sidebar_position: 1
---
```

### Adding New Pages

1. Create a Markdown file in the appropriate directory under `docs/`
2. Add frontmatter with title and position
3. The page appears automatically in the sidebar

### Sidebar Configuration

The sidebar is auto-generated from the file structure. To customize order or grouping, edit `sidebars.ts`.

## Versioning

Documentation versions are created automatically when a release is published:

1. GitHub Actions workflow detects new release
2. Runs `npm run docusaurus docs:version X.Y.Z`
3. Creates snapshot in `versioned_docs/version-X.Y.Z/`
4. Updates `versions.json`

Manual versioning (rarely needed):

```bash
npm run docusaurus docs:version 1.0.0
```

## Deployment

Documentation is deployed to GitHub Pages automatically:

- **Trigger**: Push to `main` branch (changes in `docs/`)
- **URL**: https://jmylchreest.github.io/histui/
- **Workflow**: `.github/workflows/docs.yml`

## Style Guide

- Use sentence case for headings
- Include code examples for all CLI commands
- Add `bash` language specifier for shell commands
- Add `toml` specifier for configuration examples
- Use admonitions sparingly for important notes
- Keep pages focused - split large topics into multiple pages

## Resources

- [Docusaurus Documentation](https://docusaurus.io/docs)
- [Markdown Features](https://docusaurus.io/docs/markdown-features)
- [Deployment Guide](https://docusaurus.io/docs/deployment)
