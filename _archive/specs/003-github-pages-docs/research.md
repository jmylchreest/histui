# Research: GitHub Pages Documentation Site

**Feature**: 003-github-pages-docs
**Date**: 2025-12-27

## Research Summary

This document captures the research and decisions made during the planning phase for the histui/histuid documentation site.

---

## Decision 1: Documentation Framework

**Decision**: Docusaurus v3.x

**Rationale**:
- Only framework with mature, built-in versioning support (critical requirement)
- Enterprise-grade version selector UI with automatic version snapshots
- Active development with Meta backing (v3.9.2 current)
- Large ecosystem and community for troubleshooting
- Excellent GitHub Pages integration with dedicated deployment guides

**Alternatives Considered**:

| Framework | Versioning | Why Rejected |
|-----------|------------|--------------|
| MkDocs Material | Via `mike` tool | Entering maintenance mode (Nov 2025), creator moving to new project |
| VitePress | Plugin required | No native versioning, smaller ecosystem |
| Starlight/Astro | Plugin required | Versioning support new/experimental |
| mdBook | Not supported | No versioning capability at all |

**Trade-offs Accepted**:
- Requires Node.js tooling (separate from Go codebase)
- Larger build output than simpler alternatives
- React/MDX learning curve for customization (minimal for basic docs)

---

## Decision 2: Search Implementation

**Decision**: @cmfcmf/docusaurus-search-local (local search plugin)

**Rationale**:
- No external service dependency or approval process required
- Full versioning support (auto-detects version from current page)
- Works offline (index builds at compile time)
- Zero ongoing cost
- Actively maintained (v2.0.1, Oct 2025)
- Uses Algolia autocomplete UI (familiar interface) without Algolia backend

**Alternatives Considered**:

| Option | Why Rejected |
|--------|--------------|
| Algolia DocSearch | Requires application/approval process, external dependency |
| @easyops-cn/docusaurus-search-local | Less explicit versioning support |
| Pagefind | Not integrated with Docusaurus |
| Typesense | Requires self-hosting or cloud service |

**Implementation**:
1. Install: `npm install @cmfcmf/docusaurus-search-local`
2. Add to plugins array in `docusaurus.config.ts`
3. Build docs: `npm run build` (search only works on built site, not dev mode)
4. Index automatically generated during build

**Trade-offs Accepted**:
- No AI assistant feature (was P3 requirement, deferred)
- Search only works on built site, not during `npm start` dev mode
- Search index downloaded to browser (fine for small-medium docs)

---

## Decision 3: Versioning Strategy

**Decision**: All tagged releases + main branch

**Rationale**:
- Mirrors GitHub releases exactly (user expectation)
- No cleanup/retention limit (storage is cheap)
- Clear mapping: `v1.2.3` tag → `v1.2.3` docs version
- "latest" (main branch) always shows development state

**Implementation**:
- Use `docusaurus docs:version X.Y.Z` to create version snapshots
- GitHub Action triggers on release tag creation
- Version selector automatically populated from `versions.json`

---

## Decision 4: AI Assistant Integration

**Decision**: Deferred (no AI assistant in initial release)

**Rationale**:
- AI assistant was P3 priority (nice-to-have)
- All AI options require external service dependencies
- Local search provides solid baseline functionality
- Can revisit after documentation site is established

**Alternatives Considered** (for future):

| Option | Notes |
|--------|-------|
| Algolia DocSearch Ask AI | Requires Algolia service dependency |
| Kapa.ai OSS Program | Free for OSS, requires application |
| Custom OpenAI integration | Maintenance burden, API costs |

**Note**: If AI assistance becomes critical, Kapa.ai OSS program (https://kapa.ai/oss) is the best path forward for documentation-specific AI.

---

## Decision 5: Documentation Structure

**Decision**: Hierarchical by tool, then by topic

**Rationale**:
- Users typically know which tool they need help with
- Separates concerns clearly (histui CLI vs histuid daemon)
- Allows independent quickstarts per tool
- Supports cross-referencing where tools interact

**Structure**:
```
docs/
├── intro.md           # "What is histui?"
├── quickstart/        # Getting started (combined + per-tool)
├── histui/            # CLI tool documentation
│   ├── commands/      # Per-command reference
│   └── filtering.md   # Shared filtering concepts
└── histuid/           # Daemon documentation
    ├── configuration.md
    ├── theming/       # CSS theming (extensive)
    └── monitor-mode.md
```

---

## Decision 6: CI/CD Pipeline

**Decision**: GitHub Actions with two workflows

**Rationale**:
- Repository already uses GitHub Actions
- Docusaurus has official GitHub Pages deployment action
- Separate workflows for clarity: deploy vs version

**Workflows**:

1. **docs.yml** - Deploy on push to main
   - Trigger: push to main (docs/ changes only)
   - Build Docusaurus site
   - Deploy to GitHub Pages (gh-pages branch)

2. **docs-version.yml** - Create version on release
   - Trigger: release tag created
   - Run `docusaurus docs:version X.Y.Z`
   - Commit version files
   - Trigger docs.yml for deployment

---

## Decision 7: Screenshot Generation

**Decision**: Manual screenshots stored in repository

**Rationale**:
- Low volume expected (TUI, notification popups)
- Automated screenshot generation adds complexity
- Manual allows for careful composition and annotation
- Can revisit automation if update frequency increases

**Storage**: `docs/static/img/screenshots/`

---

## External References

### Upstream Citations (for documentation)

| Standard | Reference | Used For |
|----------|-----------|----------|
| D-Bus Notifications Spec | https://specifications.freedesktop.org/notification-spec/ | histuid D-Bus interface |
| GTK4 CSS Reference | https://docs.gtk.org/gtk4/css-properties.html | Theming documentation |
| XDG Base Directory | https://specifications.freedesktop.org/basedir-spec/ | File locations |
| Waybar Custom Modules | https://github.com/Alexays/Waybar/wiki/Module:-Custom | Status integration |

### Documentation Best Practices

- Docusaurus v3 docs: https://docusaurus.io/docs
- Algolia DocSearch: https://docsearch.algolia.com/
- Diátaxis framework: https://diataxis.fr/ (tutorials, how-tos, reference, explanation)
