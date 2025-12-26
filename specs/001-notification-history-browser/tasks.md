# Tasks: histui - Notification History Browser

**Input**: Design documents from `/specs/001-notification-history-browser/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/interfaces.go

**Tests**: Test tasks included per constitution principle IV (Test-First Development).

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Include exact file paths in descriptions

## Path Conventions

Based on plan.md:
- **CLI commands**: `cmd/histui/`
- **Internal packages**: `internal/model/`, `internal/store/`, `internal/adapter/`, `internal/core/`, `internal/clipboard/`
- **Examples**: `examples/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [x] T001 Initialize Go module with `go mod init github.com/jmylchreest/histui`
- [x] T002 Create directory structure per plan.md (cmd/histui/, internal/*)
- [x] T003 [P] Add Taskfile.yml with build, test, lint targets
- [x] T004 [P] Add .golangci.yml for linting configuration
- [x] T005 Install core dependencies (cobra, bubbletea, bubbles, lipgloss, ulid, go-termimg, go-toml, testify)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story

**Note**: No user story work can begin until this phase is complete

### Core Model

- [x] T006 Create Notification struct in internal/model/notification.go
- [x] T007 [P] Create Extensions struct in internal/model/notification.go
- [x] T008 [P] Add Notification validation methods in internal/model/notification.go
- [x] T009 [P] Add Notification helper methods (RelativeTime, BodyTruncated, DedupeKey) in internal/model/notification.go
- [x] T010 Write unit tests for Notification in internal/model/notification_test.go

### Configuration

- [x] T011 Create Config struct in internal/config/config.go
- [x] T012 [P] Implement LoadConfig with TOML parsing in internal/config/config.go
- [x] T013 [P] Implement config path resolution (XDG) in internal/config/config.go
- [x] T014 Write unit tests for config loading in internal/config/config_test.go
- [x] T014a Wire config loading in root.go PersistentPreRun hook

### Store Core

- [x] T015 Create Store interface and struct in internal/store/store.go
- [x] T016 [P] Implement Add, AddBatch methods in internal/store/store.go
- [x] T017 [P] Implement All, Count methods in internal/store/store.go
- [x] T018 [P] Implement Subscribe, Unsubscribe, Close methods in internal/store/store.go
- [x] T019 Write unit tests for store core operations in internal/store/store_test.go

### Persistence Layer

- [x] T020 Create Persistence interface in internal/store/persistence.go
- [x] T021 [P] Implement JSONLPersistence Load method in internal/store/persistence.go
- [x] T022 [P] Implement JSONLPersistence Append, AppendBatch methods in internal/store/persistence.go
- [x] T023 [P] Implement JSONLPersistence Rewrite, Clear, Close methods in internal/store/persistence.go
- [x] T024 Write unit tests for persistence in internal/store/persistence_test.go

### CLI Root Command

- [x] T025 Create root command with Cobra in cmd/histui/root.go
- [x] T026 [P] Add global flags (--verbose, --persist, --config) in cmd/histui/root.go
- [x] T027 [P] Implement slog logger setup in cmd/histui/root.go
- [x] T028 Create main.go entrypoint in cmd/histui/main.go

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Quick Notification Lookup via Waybar (Priority: P1) MVP

**Goal**: Click waybar icon → fuzzel shows notifications → select → copy to clipboard

**Independent Test**: `histui get --format dmenu --ulid | fuzzel -d | histui get --body | wl-copy`

### Input Adapters for US1

- [ ] T029 [P] [US1] Create InputAdapter interface in internal/adapter/input/input.go
- [ ] T030 [P] [US1] Implement DunstAdapter with dunstctl history parsing in internal/adapter/input/dunst.go
- [ ] T031 [P] [US1] Handle dunst timestamp conversion (microseconds since boot) in internal/adapter/input/dunst.go
- [ ] T032 [US1] Write unit tests for DunstAdapter in internal/adapter/input/dunst_test.go

### Core Filtering/Sorting for US1

- [ ] T033 [P] [US1] Implement Filter function in internal/core/filter.go
- [ ] T034 [P] [US1] Implement Sort function in internal/core/sort.go
- [ ] T035 [P] [US1] Implement duration parsing (48h, 7d, 0) in internal/core/filter.go
- [ ] T036 [P] [US1] Write unit tests for filter in internal/core/filter_test.go
- [ ] T037 [P] [US1] Write unit tests for sort in internal/core/sort_test.go

### Store Filter/Lookup for US1

- [ ] T038 [US1] Implement Store.Filter method in internal/store/store.go
- [ ] T039 [US1] Implement Store.Lookup method in internal/store/store.go
- [ ] T040 [US1] Write unit tests for Filter and Lookup in internal/store/store_test.go

### Lookup Logic for US1

- [ ] T041 [US1] Implement LookupNotification (ULID match, content fallback) in internal/core/lookup.go
- [ ] T042 [US1] Write unit tests for lookup in internal/core/lookup_test.go

### Get Command Output Formatter for US1

- [ ] T043 [P] [US1] Create Formatter interface in internal/adapter/output/formatter.go
- [ ] T044 [P] [US1] Implement template functions (formatTime, truncate, etc.) in internal/adapter/output/formatter.go
- [ ] T045 [US1] Implement GetFormatter with field flags and format presets in internal/adapter/output/get.go
- [ ] T046 [US1] Write unit tests for GetFormatter in internal/adapter/output/get_test.go

### Get Command for US1

- [ ] T047 [US1] Create get subcommand with all flags in cmd/histui/get.go
- [ ] T048 [US1] Implement stdin detection (list all vs lookup mode) in cmd/histui/get.go
- [ ] T049 [US1] Wire get command to store, filter, formatter in cmd/histui/get.go
- [ ] T050 [US1] Integration test: dmenu pipeline end-to-end

**Checkpoint**: User Story 1 complete - `histui get` works with dmenu/fuzzel pipeline

---

## Phase 4: User Story 2 - Interactive TUI Browser (Priority: P2)

**Goal**: Full-screen TUI with keyboard navigation, search, detail view, clipboard

**Independent Test**: Run `histui` or `histui tui`, navigate with j/k, search with /, view with Enter

### Clipboard Utility for US2

- [ ] T051 [P] [US2] Create Clipboard interface in internal/clipboard/clipboard.go
- [ ] T052 [P] [US2] Implement wl-copy with xclip fallback in internal/clipboard/clipboard.go
- [ ] T053 [P] [US2] Write unit tests for clipboard detection in internal/clipboard/clipboard_test.go

### TUI Core for US2

- [ ] T054 [US2] Create TUI model struct with BubbleTea in internal/adapter/output/tui.go
- [ ] T055 [US2] Implement Init, Update, View methods in internal/adapter/output/tui.go
- [ ] T056 [US2] Implement list view with bubbles list component in internal/adapter/output/tui.go
- [ ] T057 [US2] Implement keyboard navigation (j/k, g/G, Ctrl-d/u) in internal/adapter/output/tui.go

### TUI Search Mode for US2

- [ ] T058 [US2] Implement search mode with `/` trigger in internal/adapter/output/tui.go
- [ ] T059 [US2] Implement real-time fuzzy filtering with match highlighting in internal/adapter/output/tui.go
- [ ] T060 [US2] Implement search cancel with Esc in internal/adapter/output/tui.go

### TUI Detail View for US2

- [ ] T061 [US2] Implement detail view layout in internal/adapter/output/tui.go
- [ ] T062 [US2] Implement icon rendering with go-termimg in internal/adapter/output/tui.go
- [ ] T063 [US2] Implement graceful fallback for unsupported terminals in internal/adapter/output/tui.go

### TUI Actions for US2

- [ ] T064 [US2] Implement copy to clipboard (y=body, Y=all) in internal/adapter/output/tui.go
- [ ] T065 [US2] Implement print to stdout (Enter/p keys) in internal/adapter/output/tui.go
- [ ] T066 [US2] Implement open URL (o key) with URL extraction from body in internal/adapter/output/tui.go
- [ ] T067 [US2] Implement delete notification (d key) in internal/adapter/output/tui.go

### TUI Styling for US2

- [ ] T068 [US2] Add lipgloss styles for list, selection, urgency in internal/adapter/output/tui.go
- [ ] T069 [US2] Add footer with keybind hints in internal/adapter/output/tui.go

### TUI Command for US2

- [ ] T070 [US2] Create tui subcommand in cmd/histui/tui.go
- [ ] T071 [US2] Set TUI as default when no subcommand in cmd/histui/root.go
- [ ] T072 [US2] Add --output-template flag for TUI stdout output in cmd/histui/root.go

### TUI Reactive Updates for US2

- [ ] T073 [US2] Subscribe TUI to store change events in internal/adapter/output/tui.go
- [ ] T074 [US2] Refresh list on store changes in internal/adapter/output/tui.go

**Checkpoint**: User Story 2 complete - TUI fully functional with all keybindings

---

## Phase 5: User Story 3 - Waybar Status Integration (Priority: P3)

**Goal**: Waybar icon shows notification status and count

**Independent Test**: `histui status | jq .` outputs valid waybar JSON

### Status Formatter for US3

- [ ] T075 [P] [US3] Implement StatusFormatter for waybar JSON in internal/adapter/output/status.go
- [ ] T076 [P] [US3] Query dunst pause state via dunstctl in internal/adapter/output/status.go
- [ ] T077 [US3] Write unit tests for StatusFormatter in internal/adapter/output/status_test.go

### Status Command for US3

- [ ] T078 [US3] Create status subcommand in cmd/histui/status.go
- [ ] T079 [US3] Handle dunst unavailable gracefully in cmd/histui/status.go

**Checkpoint**: User Story 3 complete - waybar integration works

---

## Phase 6: User Story 4 - Filtering and Sorting (Priority: P4)

**Goal**: Filter by app/urgency/time, sort by various fields

**Independent Test**: `histui get --app-filter firefox --since 1h --sort urgency:desc`

### Enhanced Filtering for US4

- [ ] T080 [P] [US4] Add --app-filter flag implementation in cmd/histui/get.go
- [ ] T081 [P] [US4] Add --urgency flag implementation in cmd/histui/get.go
- [ ] T082 [P] [US4] Add --limit flag implementation in cmd/histui/get.go
- [ ] T083 [US4] Write integration tests for filter combinations in internal/core/filter_test.go

### Enhanced Sorting for US4

- [ ] T084 [P] [US4] Parse --sort field:order syntax in cmd/histui/get.go
- [ ] T085 [P] [US4] Implement sort by app, urgency fields in internal/core/sort.go
- [ ] T086 [US4] Write tests for sort combinations in internal/core/sort_test.go

**Checkpoint**: User Story 4 complete - all filter/sort flags work

---

## Phase 7: User Story 5 - Persistent History (Priority: P5)

**Goal**: History persists across restarts

**Independent Test**: Import notifications, restart histui, verify they're still visible

### Persistence Integration for US5

- [ ] T087 [US5] Wire persistence to store on startup (hydrate) in internal/store/store.go
- [ ] T088 [US5] Wire persistence on Add/AddBatch (append) in internal/store/store.go
- [ ] T089 [US5] Implement schema version header in JSONL in internal/store/persistence.go
- [ ] T090 [US5] Handle corrupted file gracefully (backup, recreate) in internal/store/persistence.go
- [ ] T091 [US5] Write integration tests for persistence across restarts in internal/store/persistence_test.go

**Checkpoint**: User Story 5 complete - notifications persist

---

## Phase 8: User Story 6 - Multiple Input Sources (Priority: P6)

**Goal**: Import from dunst or stdin JSON

**Independent Test**: `dunstctl history | histui get --source stdin`

### Stdin Adapter for US6

- [ ] T092 [P] [US6] Implement StdinAdapter for JSON input in internal/adapter/input/stdin.go
- [ ] T093 [P] [US6] Handle both array and dunst history format in internal/adapter/input/stdin.go
- [ ] T094 [US6] Write unit tests for StdinAdapter in internal/adapter/input/stdin_test.go

### Source Auto-Detection for US6

- [ ] T095 [US6] Implement daemon auto-detection in internal/adapter/input/input.go
- [ ] T096 [US6] Add --source flag to get command in cmd/histui/get.go
- [ ] T097 [US6] Track notification source in store in internal/store/store.go

**Checkpoint**: User Story 6 complete - multiple input sources work

---

## Phase 9: User Story 7 - History Maintenance (Priority: P7)

**Goal**: Prune old notifications from storage

**Independent Test**: `histui prune --dry-run` shows what would be removed

### Prune Utility for US7

- [ ] T098 [P] [US7] Implement Store.Prune method as reusable utility in internal/store/prune.go
- [ ] T099 [P] [US7] Implement --older-than duration parsing in internal/store/prune.go
- [ ] T100 [P] [US7] Implement --keep N limit in internal/store/prune.go
- [ ] T101 [P] [US7] Implement --dry-run preview in internal/store/prune.go
- [ ] T102 [US7] Write unit tests for prune in internal/store/prune_test.go

### Prune Command for US7

- [ ] T103 [US7] Create prune subcommand in cmd/histui/prune.go
- [ ] T104 [US7] Wire prune to persistence (rewrite file) in cmd/histui/prune.go

**Checkpoint**: User Story 7 complete - prune command works

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, examples, cleanup

- [ ] T105 [P] Create example waybar config in examples/waybar/config.jsonc
- [ ] T106 [P] Create fuzzel-notifications.sh script in examples/scripts/fuzzel-notifications.sh
- [ ] T107 [P] Create walker-notifications.sh script in examples/scripts/walker-notifications.sh
- [ ] T108 [P] Create README.md with installation and usage
- [ ] T109 Add version injection via ldflags in Taskfile.yml
- [ ] T110 Add static binary build (CGO_ENABLED=0) in Taskfile.yml
- [ ] T111 Run golangci-lint and fix issues
- [ ] T112 Verify all quickstart.md examples work
- [ ] T113 Performance test: <100ms for get, <1s for TUI with 100 notifications

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies - start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 - BLOCKS all user stories
- **Phase 3-9 (User Stories)**: All depend on Phase 2 completion
- **Phase 10 (Polish)**: Depends on desired user stories being complete

### User Story Dependencies

| Story | Can Start After | Dependencies on Other Stories |
|-------|-----------------|-------------------------------|
| US1 (P1) | Phase 2 | None - fully independent |
| US2 (P2) | Phase 2 | None - uses store independently |
| US3 (P3) | Phase 2 | None - status is independent |
| US4 (P4) | US1 | Extends get command from US1 |
| US5 (P5) | Phase 2 | None - persistence is foundational |
| US6 (P6) | US1 | Extends input adapters from US1 |
| US7 (P7) | US5 | Requires persistence from US5 |

### Parallel Opportunities

**Within Phase 2 (Foundational)**:
- T006-T010 (Model) can run in parallel with T011-T014 (Config)
- T015-T019 (Store) depends on T006-T010 (Model)
- T020-T024 (Persistence) can run in parallel with T015-T019

**Within US1**:
- T029-T032 (Dunst adapter), T033-T037 (Filter/Sort), T043-T046 (Formatter) can all start in parallel
- T047-T049 (Get command) waits for above to complete

**Within US2**:
- T051-T053 (Clipboard) can run in parallel with T054-T057 (TUI core)
- All TUI features (T058-T074) mostly sequential due to same file

**Cross-Story Parallel**:
- After Phase 2, US1, US2, US3, and US5 can all start in parallel
- US4 waits for US1, US6 waits for US1, US7 waits for US5

---

## Parallel Execution Examples

### Phase 2: Foundation
```
# Parallel batch 1: Model + Config
Task T006: Create Notification struct
Task T011: Create Config struct

# Parallel batch 2: Store + Persistence (after model)
Task T015: Create Store interface
Task T020: Create Persistence interface

# Parallel batch 3: CLI
Task T025: Create root command
```

### User Story 1: Quick Lookup
```
# Parallel batch 1: Core components
Task T029: InputAdapter interface
Task T033: Filter function
Task T043: Formatter interface

# Parallel batch 2: Implementations
Task T030: DunstAdapter
Task T034: Sort function
Task T044: Template functions

# Sequential: Get command (needs all above)
Task T047: Create get subcommand
```

### Multi-Story Parallel (if team capacity)
```
# Developer A: User Story 1
# Developer B: User Story 2
# Developer C: User Story 3

# All can start after Phase 2 completes
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (model, store, persistence, CLI root)
3. Complete Phase 3: User Story 1 (get command with dmenu pipeline)
4. **STOP and VALIDATE**: Test with `histui get --format dmenu --ulid | fuzzel -d`
5. Deploy/demo if ready

### Incremental Delivery

| Increment | Stories | Value Delivered |
|-----------|---------|-----------------|
| MVP | US1 | Waybar → fuzzel pipeline works |
| +TUI | US1+US2 | Full interactive browser |
| +Status | US1+US2+US3 | Complete waybar integration |
| +Filters | +US4 | Power user filtering |
| +Persist | +US5 | History survives restarts |
| +Sources | +US6 | Multi-daemon support |
| +Prune | +US7 | History maintenance |

### Suggested MVP Scope

**Minimum**: User Story 1 only (21 tasks: T029-T050)
- Provides core value: waybar → fuzzel → clipboard pipeline
- Can be completed and validated independently

**Recommended MVP**: User Story 1 + User Story 3 (US3 adds 5 tasks)
- Complete waybar integration (status + click to browse)
- Natural pairing for the primary use case

---

## Notes

- Constitution requires tests (Principle IV) - test tasks included
- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Each user story is independently completable and testable
- Write tests first, ensure they fail before implementation
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently

---

## Summary

| Phase | Task Range | Count | Description |
|-------|------------|-------|-------------|
| Setup | T001-T005 | 5 | Project initialization |
| Foundational | T006-T028 (+T014a) | 24 | Core model, store, persistence, CLI, config |
| US1 (P1) | T029-T050 | 22 | Quick lookup via waybar |
| US2 (P2) | T051-T074 | 24 | Interactive TUI browser |
| US3 (P3) | T075-T079 | 5 | Waybar status integration |
| US4 (P4) | T080-T086 | 7 | Filtering and sorting |
| US5 (P5) | T087-T091 | 5 | Persistent history |
| US6 (P6) | T092-T097 | 6 | Multiple input sources |
| US7 (P7) | T098-T104 | 7 | History maintenance |
| Polish | T105-T113 | 9 | Documentation, examples |
| **Total** | | **114** | |
