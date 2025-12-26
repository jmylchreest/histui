<!--
Sync Impact Report
==================
Version change: 1.1.0 → 1.2.0 (MINOR: centralized store architecture)

Modified principles:
  - I. Pluggable Adapter Architecture → Updated with import vs stream adapters
  - III. Clean Code Structure → Updated project structure with store package

Added sections:
  - ADR-002: Centralized History Store with Reactive Updates
  - Input adapter types: Import (one-shot) vs Stream (continuous)
  - Store persistence and change notification architecture

Removed sections:
  - None

Templates requiring updates:
  - .specify/templates/plan-template.md - ✅ Generic, no changes needed
  - .specify/templates/spec-template.md - ✅ Generic, no changes needed
  - .specify/templates/tasks-template.md - ✅ Generic, no changes needed

Follow-up TODOs:
  - Rename GitHub repo from dunst-history-formatter to histui when created
-->

# histui Constitution

## Overview

**histui** (history + TUI) is a notification history browser for Linux desktops. It provides a unified interface for viewing, searching, and acting on notification history from multiple notification daemons.

The tool follows a centralized store architecture with pluggable adapters:

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Input Adapters │────▶│  History Store  │────▶│ Output Adapters │
├─────────────────┤     ├─────────────────┤     ├─────────────────┤
│ IMPORT (one-shot)│    │ • In-memory     │     │ • dmenu list    │
│ • dunst         │     │ • Disk persist  │     │ • TUI (reactive)│
│ • stdin         │     │ • Source track  │     │ • waybar JSON   │
│                 │     │ • Change notify │     │ • JSON dump     │
│ STREAM (live)   │     │                 │     │                 │
│ • dbus (future) │     │                 │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

## Architecture Decisions

### ADR-001: Pluggable Adapters Over Daemon Replacement

**Context**: We need to support notification history browsing for Hyprland/Waybar users.

**Options Considered**:
1. **Wrapper only**: Parse dunstctl output directly
2. **Full daemon replacement**: Implement org.freedesktop.Notifications D-Bus interface
3. **Pluggable adapters**: Support multiple input sources with common core

**Decision**: Option 3 - Pluggable adapters

**Rationale**:
- Allows incremental development (start with dunst, add others)
- Stdin adapter enables ANY source to pipe history in
- Daemon development can be a separate project that feeds into histui
- Benefits broader community (mako, swaync users)
- Clean separation of concerns

**Consequences**:
- Slightly more abstraction in initial design
- Need to define common notification format
- Future daemon becomes separate project (histui-daemon or similar)

### ADR-002: Centralized History Store with Reactive Updates

**Context**: We need to support both one-shot CLI usage and interactive TUI with live updates.

**Options Considered**:
1. **Pass-through**: Adapters output directly to formatters
2. **Centralized store**: All notifications flow through a central store

**Decision**: Option 2 - Centralized store

**Rationale**:
- Single source of truth for all notification data
- Enables disk persistence for long-term history across sessions
- Supports reactive TUI updates when new notifications arrive
- Tracks notification source (dunst, dbus, stdin) for debugging/filtering
- Deduplication possible when same notification comes from multiple sources
- Clean separation: adapters write to store, formatters read from store

**Architecture**:
```
┌──────────────┐
│ History Store│
├──────────────┤
│ In-Memory    │◄── Hydrate on startup
│   Cache      │
├──────────────┤
│ Disk         │◄── Persist on change (optional)
│ Persistence  │
├──────────────┤
│ Change       │───▶ Notify subscribers (TUI)
│ Channel      │
└──────────────┘
```

**Store Entry Metadata**:
- Notification ID (from source or generated)
- Source identifier (dunst, dbus, stdin, etc.)
- Import timestamp (when histui received it)
- Original timestamp (from notification daemon)
- Full notification payload

**Consequences**:
- Slightly more memory usage (in-memory cache)
- Need to handle persistence format/location
- TUI must subscribe to change channel for updates
- One-shot modes (list, status) ignore change channel

## Core Principles

### I. Pluggable Adapter Architecture

The tool MUST support pluggable input and output adapters:

**Input Adapters** (notification sources):

Two types of input adapters:

1. **Import Adapters** (one-shot injection):
   - Execute command, parse output, inject into store
   - Examples: dunst (`dunstctl history`), stdin (piped JSON)
   - Used at startup or on-demand refresh

2. **Stream Adapters** (continuous injection):
   - Monitor source continuously, inject as notifications arrive
   - Examples: D-Bus monitor (future)
   - Triggers store change notifications for reactive TUI updates

All adapters:
- Implement a common `HistoryProvider` interface
- Normalize notifications to common format before store injection
- Track source identifier for each notification

**Output Adapters** (presentation formats):
- Each adapter implements a common `Formatter` interface
- **List mode**: Single-line-per-notification for dmenu/rofi/fuzzel/walker (one-shot)
- **TUI mode**: Interactive BubbleTea-based browser (reactive, subscribes to store changes)
- **Status mode**: Waybar-compatible JSON for notification icon (one-shot)
- **JSON mode**: Raw normalized JSON for scripting (one-shot)

Mode and source selection via CLI flags. Core logic is agnostic to both input source and output format.

### II. Idiomatic Go

Follow Go conventions strictly:
- Error handling via explicit returns (no panic for recoverable errors)
- Context propagation where appropriate for cancellation
- `defer` for cleanup operations
- Table-driven tests where applicable
- No exceptions to `go fmt` and `go vet`
- Use `golangci-lint` with standard configuration
- Channels for change notification (not callbacks)

### III. Clean Code Structure

Keep the codebase simple and maintainable:
- **Single Responsibility**: Each package has one clear purpose
- **Small interfaces**: Define only what's needed
- **Minimal dependencies**: Prefer stdlib where possible

Project structure:
```
cmd/histui/        # CLI entrypoint
internal/
  adapter/
    input/         # Input adapters (dunst, stdin, dbus future)
    output/        # Output adapters (list, tui, status, json)
  store/           # History store (in-memory + persistence)
  core/            # Filtering, sorting, searching logic
  model/           # Notification data structures
```

### IV. Test-First Development

TDD is encouraged for core logic:
- Input adapter parsing MUST have unit tests
- Store operations MUST have unit tests
- Output formatter output SHOULD have tests for expected format
- Core filtering/sorting MUST have unit tests
- TUI interaction testing is optional (difficult to test)
- Use `testify` for assertions if helpful

### V. Security Considerations

Even for a simple CLI tool:
- Validate JSON input before processing
- Handle malformed input gracefully (don't crash)
- No shell injection in any output formatting
- Sanitize notification content when displaying
- Don't execute arbitrary commands from notification content
- Persistence file permissions should be restrictive (0600)

### VI. User Experience Focus

This tool is for daily use by power users:
- Fast startup time (under 100ms for list mode)
- Clear error messages when daemons fail or return invalid data
- Sensible defaults that work without configuration
- Auto-detect notification daemon when possible
- Optional configuration via flags or config file
- Persistence location follows XDG spec (`~/.local/share/histui/`)

### VII. Structured Logging

Use `slog` (Go stdlib) for any diagnostic output:
- No emojis in log output
- Logs go to stderr, formatted output goes to stdout
- Default to minimal logging; verbose mode available via flag

### VIII. Build Standards

**Build Requirements:**
- Single binary output with no runtime dependencies
- Static linking where possible (CGO_ENABLED=0)
- Version injection via LDFLAGS (version, commit SHA)

**Development Tools:**
- Taskfile for build automation
- golangci-lint for code quality
- go test for testing

## Technology Stack

| Layer | Technology | Notes |
|-------|------------|-------|
| Language | Go | 1.21+ (for slog) |
| CLI | Cobra | Flag parsing and subcommands |
| TUI | BubbleTea + Lipgloss | Interactive mode |
| Logging | slog (stdlib) | Structured logging |
| Testing | testify | Assertions and mocks |
| Build System | Taskfile | Task automation |
| Persistence | JSONL file | Simple, human-readable, append-friendly |

## Supported Notification Daemons

| Daemon | Priority | Command | History Support |
|--------|----------|---------|-----------------|
| dunst | v1.0 | `dunstctl history` | Full JSON export |
| stdin | v1.0 | pipe JSON | Universal fallback |
| dbus | v1.x | D-Bus monitor | Build our own history |
| mako | Future | N/A | No history export (issue #91 open since 2018) |
| swaync | Future | N/A | No history export (issues #189, #468, #675) |

**Note**: mako and swaync do not currently support history export. When/if they add this feature, we can add adapters. In the meantime, the D-Bus capture mode will work with ANY daemon.

## Future Roadmap

### Phase 1 (v1.0): Core Functionality
- Dunst import adapter
- Stdin (JSON) import adapter
- History store with in-memory cache
- Optional disk persistence
- List output (dmenu/fuzzel/walker)
- TUI output (BubbleTea)
- Waybar status output
- Filtering and sorting

### Phase 2 (v1.x): D-Bus Capture & Live Updates
- **D-Bus notification monitor** (stream adapter)
  - Capture notifications in real-time from org.freedesktop.Notifications
  - Works with ANY notification daemon
  - Builds histui's own history independent of daemon
- Reactive TUI updates via store change channel
- Configuration file support

### Phase 3 (v2.x): Expanded Features
- Mako import adapter (when/if they add history export)
- Swaync import adapter (when/if they add history export)
- Notification actions (invoke, dismiss)
- Search/filter presets

### Phase 4 (Future): histui-daemon
- Separate project implementing org.freedesktop.Notifications
- Outputs history in histui-compatible format
- Lightweight alternative to dunst/mako
- Native integration with histui store

## Development Workflow

### Spec-Kit Phases

1. **Constitution** - This document
2. **Specify** - `/speckit.specify` - Define feature requirements
3. **Clarify** - `/speckit.clarify` - Resolve ambiguities
4. **Plan** - `/speckit.plan` - Technical design
5. **Tasks** - `/speckit.tasks` - Task breakdown
6. **Implement** - `/speckit.implement` - Code generation

### Code Standards

- All tests pass (`task test` or `go test ./...`)
- `golangci-lint` passes (`task lint`)
- Code is formatted (`go fmt ./...`)
- No TODO comments without explanation
- Exported functions have godoc comments

## Governance

This constitution guides development practices. Amendments require:
1. Clear justification for the change
2. Update to this document
3. Version increment

**Version**: 1.2.0 | **Ratified**: 2025-12-26 | **Last Amended**: 2025-12-26
