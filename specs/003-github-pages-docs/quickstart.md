# Quickstart: Contributing to Documentation

**Feature**: 003-github-pages-docs
**Date**: 2025-12-27

This guide helps contributors get started with the histui documentation site.

---

## Prerequisites

- Node.js 20+ (LTS recommended)
- npm 10+
- Git

---

## Local Development Setup

### 1. Clone the Repository

```bash
git clone https://github.com/jmylchreest/histui.git
cd histui
```

### 2. Install Documentation Dependencies

```bash
cd docs
npm install
```

### 3. Start Development Server

```bash
npm start
```

This opens `http://localhost:3000/histui/` with hot-reloading enabled.

---

## Making Changes

### Editing Existing Pages

1. Find the page in `docs/docs/` (e.g., `docs/docs/histui/commands/get.md`)
2. Edit the Markdown content
3. Save - changes appear instantly in browser

### Adding New Pages

1. Create a new `.md` file in the appropriate section:
   ```bash
   touch docs/docs/histui/commands/new-command.md
   ```

2. Add frontmatter:
   ```markdown
   ---
   title: "histui new-command"
   description: "Brief description"
   sidebar_position: 5
   ---

   # histui new-command

   Content here...
   ```

3. The page automatically appears in the sidebar

### Adding a New Section

1. Create a folder:
   ```bash
   mkdir docs/docs/new-section
   ```

2. Add category metadata:
   ```bash
   # docs/docs/new-section/_category_.json
   {
     "label": "New Section",
     "position": 4
   }
   ```

3. Add an index page:
   ```bash
   touch docs/docs/new-section/index.md
   ```

---

## Building for Production

### Full Build

```bash
npm run build
```

Output is in `docs/build/` - this is what gets deployed.

### Check for Issues

```bash
# Broken links
npm run build  # Throws on broken links

# Markdown linting (if configured)
npm run lint
```

---

## Versioning (Maintainers Only)

Versioning is automated via GitHub Actions on release. Manual versioning:

```bash
# Create a version snapshot
npm run docusaurus docs:version 1.2.3

# Files created:
# - versioned_docs/version-1.2.3/
# - versioned_sidebars/version-1.2.3-sidebars.json
# - versions.json (updated)
```

---

## File Structure Reference

```
docs/
├── docusaurus.config.ts   # Site configuration
├── sidebars.ts            # Navigation structure
├── package.json           # Dependencies
├── docs/                  # Markdown content (current version)
│   ├── intro.md
│   ├── quickstart/
│   ├── histui/
│   └── histuid/
├── src/
│   ├── components/        # Custom React components
│   ├── css/custom.css     # Global styles
│   └── pages/             # Custom pages (homepage)
├── static/
│   ├── img/               # Images
│   └── examples/          # Example files
├── versioned_docs/        # Past version snapshots
└── versions.json          # Version manifest
```

---

## Style Guidelines

### Markdown

- Use ATX-style headers (`#`, `##`, not underlines)
- One sentence per line (for better diffs)
- Fenced code blocks with language identifier
- Use relative links for internal references

### Code Examples

- Prefer complete, runnable examples
- Show expected output where helpful
- Use comments sparingly but clearly

### Screenshots

- Store in `static/img/screenshots/`
- Use descriptive filenames: `tui-filter-view.png`
- Optimize images (PNG for UI, SVG for diagrams)
- Alt text is required for accessibility

---

## Troubleshooting

### Port Already in Use

```bash
npm start -- --port 3001
```

### Build Errors

```bash
# Clear cache
npm run clear
npm run build
```

### Broken Links

The build fails on broken links. Check:
- File exists at the linked path
- Path is relative from current file
- No typos in filename

---

## Getting Help

- Open an issue: https://github.com/jmylchreest/histui/issues
- Check existing docs: https://jmylchreest.github.io/histui/
