# Implementation Plan: histui - Notification History Browser

**Branch**: `001-notification-history-browser` | **Date**: 2025-12-26 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-notification-history-browser/spec.md`

## Summary

**histui** is a notification history browser for Linux desktops that provides a unified interface for viewing, searching, and acting on notification history. The tool uses a pluggable adapter architecture with a centralized history store:

- **Commands**: `get` (query/output), `status` (waybar), `tui` (interactive, default), `prune` (cleanup)
- **Input adapters**: dunst (`dunstctl history`), stdin (piped JSON)
- **History store**: In-memory cache with optional JSONL disk persistence, change notification channel, reusable prune utility
- **Output formats**: dmenu preset, JSON, Go templates, waybar status JSON

Primary use case: Hyprland users clicking a waybar notification icon to browse missed notifications via fuzzy finder, with full notification details available through composable shell pipelines.

## Technical Context

**Language/Version**: Go 1.21+ (required for stdlib `slog`)
**Primary Dependencies**: Cobra (CLI), BubbleTea + Lipgloss (TUI), go-termimg (terminal images), go-toml (config), testify (assertions)
**Storage**: JSONL file at `~/.local/share/histui/history.jsonl` (XDG spec)
**Testing**: `go test` with testify assertions, table-driven tests
**Target Platform**: Linux (Wayland/X11, Hyprland/Waybar focus)
**Project Type**: Single CLI binary
**Performance Goals**: <100ms list mode startup, <1s TUI load with 100+ notifications
**Constraints**: Single static binary (CGO_ENABLED=0), <50MB memory typical usage
**Scale/Scope**: Typical ~100 notifications, persistence supports 1000+ notifications

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Pluggable Adapter Architecture | ✅ PASS | Input adapters (dunst, stdin) + Output adapters (list, tui, status, json) |
| II. Idiomatic Go | ✅ PASS | Error returns, context propagation, defer, channels for change notification |
| III. Clean Code Structure | ✅ PASS | Single responsibility packages, minimal dependencies |
| IV. Test-First Development | ✅ PASS | Unit tests for adapters, store, core logic |
| V. Security Considerations | ✅ PASS | JSON validation, content sanitization, 0600 persistence file |
| VI. User Experience Focus | ✅ PASS | <100ms startup, clear errors, XDG paths, auto-detect daemon |
| VII. Structured Logging | ✅ PASS | slog to stderr, no emojis, verbose flag |
| VIII. Build Standards | ✅ PASS | Single binary, static linking, version injection |
| ADR-001: Pluggable Adapters | ✅ PASS | Design follows adapter pattern |
| ADR-002: Centralized Store | ✅ PASS | In-memory + persistence + change channel |

**Gate Status**: PASS - All constitution principles satisfied

## Project Structure

### Documentation (this feature)

```text
specs/001-notification-history-browser/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
cmd/
└── histui/
    ├── main.go              # CLI entrypoint, default to TUI
    ├── root.go              # Root command setup
    ├── get.go               # get subcommand
    ├── status.go            # status subcommand
    ├── tui.go               # tui subcommand
    └── prune.go             # prune subcommand

internal/
├── adapter/
│   ├── input/
│   │   ├── input.go         # InputAdapter interface
│   │   ├── dunst.go         # DunstAdapter implementation
│   │   ├── dunst_test.go
│   │   ├── stdin.go         # StdinAdapter implementation
│   │   └── stdin_test.go
│   └── output/
│       ├── formatter.go     # Formatter interface and utilities
│       ├── get.go           # GetFormatter (field/format output)
│       ├── get_test.go
│       ├── tui.go           # TUI (BubbleTea)
│       ├── status.go        # StatusFormatter (waybar JSON)
│       └── status_test.go
├── store/
│   ├── store.go             # HistoryStore implementation
│   ├── store_test.go
│   ├── persistence.go       # JSONL file operations
│   ├── persistence_test.go
│   ├── prune.go             # Prune utility (reusable)
│   └── prune_test.go
├── core/
│   ├── filter.go            # Filtering logic (--since, --app-filter, etc.)
│   ├── filter_test.go
│   ├── sort.go              # Sorting logic (--sort field:order)
│   ├── sort_test.go
│   ├── lookup.go            # Notification lookup by ULID or content
│   └── lookup_test.go
├── clipboard/
│   ├── clipboard.go         # Clipboard utility (TUI only)
│   └── clipboard_test.go
├── config/
│   ├── config.go            # Config struct and loading
│   └── config_test.go
└── model/
    ├── notification.go      # Notification data structures
    └── notification_test.go

examples/
├── waybar/
│   └── config.jsonc         # Example waybar integration
└── scripts/
    ├── fuzzel-notifications.sh
    └── walker-notifications.sh

go.mod
go.sum
Taskfile.yml                 # Build automation
README.md
```

**Structure Decision**: Go single-binary CLI project following standard Go project layout with `cmd/` for entrypoints and `internal/` for private packages. Follows constitution principle III (Clean Code Structure) with single-responsibility packages.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations. Design follows all constitution principles.

---

## Generated Artifacts

### Phase 0: Research (Complete)

| Artifact | Path | Description |
|----------|------|-------------|
| Research | [research.md](./research.md) | Technology decisions and best practices |

**Key Decisions:**
- Cobra CLI with subcommands: `get`, `status`, `tui` (default), `prune`
- Smart `get` command: no stdin = list all, with stdin = lookup specific notification
- BubbleTea + Bubbles list component for TUI
- oklog/ulid/v2 for sortable unique IDs
- bufio.Scanner for JSONL streaming
- Default 48h time filter with `--since` override
- Reusable prune utility in store (for command and future D-Bus ingest)
- Clipboard only in TUI mode (shell pipelines handle it for `get`)

### Phase 1: Design (Complete)

| Artifact | Path | Description |
|----------|------|-------------|
| Data Model | [data-model.md](./data-model.md) | Entity definitions and relationships |
| Contracts | [contracts/interfaces.go](./contracts/interfaces.go) | Go interface definitions |
| Quickstart | [quickstart.md](./quickstart.md) | Developer setup guide |

**Key Entities:**
- `Notification` - Core data model with ULID, source tracking, freedesktop fields
- `Store` - In-memory cache with persistence and change notifications
- `InputAdapter` - Interface for notification sources (dunst, stdin)
- `OutputAdapter` - Interface for formatters (list, tui, status, json)

### Phase 2: Tasks (Pending)

Run `/speckit.tasks` to generate `tasks.md` with actionable implementation tasks.

---

## Next Steps

1. **Generate Tasks**: Run `/speckit.tasks` to create implementation task breakdown
2. **Initialize Project**: Create Go module and directory structure
3. **Implement Model**: Start with `internal/model/notification.go`
4. **Test-Driven**: Write tests before implementations
5. **Iterate**: Build adapters incrementally (dunst → stdin → list → tui)
