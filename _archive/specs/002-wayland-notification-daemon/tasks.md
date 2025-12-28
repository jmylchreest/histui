# Tasks: histuid - Wayland Notification Daemon

**Input**: Design documents from `/specs/002-wayland-notification-daemon/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Tests are included where specified by TDD principles in the constitution.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

Based on plan.md structure:
- `cmd/histui/` - CLI commands (extended)
- `cmd/histuid/` - Daemon entry point (new)
- `internal/dbus/` - D-Bus interface (new)
- `internal/daemon/` - Daemon orchestration (new)
- `internal/display/` - GTK4/libadwaita popups (new)
- `internal/theme/` - CSS theming (new)
- `internal/audio/` - Sound playback (new)
- `internal/store/` - Shared state (extended)
- `internal/config/` - Configuration (extended)
- `internal/core/` - Filter parsing (extended)
- `internal/adapter/output/` - Output formatters (extended)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and dependencies for histuid

- [x] T001 Add new Go dependencies to go.mod: godbus/dbus/v5, gotk4, gotk4-adwaita, gotk4-layer-shell, gopxl/beep
- [x] T002 [P] Create cmd/histuid/ directory with placeholder main.go
- [x] T003 [P] Create internal/dbus/ package directory structure
- [x] T004 [P] Create internal/daemon/ package directory structure
- [x] T005 [P] Create internal/display/ package directory structure
- [x] T006 [P] Create internal/theme/ package directory structure
- [x] T007 [P] Create internal/audio/ package directory structure
- [x] T008 Update Taskfile.yml with histuid build target (CGO_ENABLED=1)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T009 Implement D-Bus type definitions in internal/dbus/types.go (DBusNotification, Action, CloseReason constants)
- [x] T010 [P] Implement SharedState struct in internal/store/state.go (DnDEnabled, DnDEnabledAt, SchemaVersion)
- [x] T011 [P] Implement DaemonConfig struct in internal/config/daemon.go with all config sections
- [x] T012 [P] Implement DefaultDaemonConfig() function in internal/config/daemon.go
- [x] T013 Implement LoadDaemonConfig() function in internal/config/daemon.go (TOML parsing)
- [x] T014 [P] Implement LoadSharedState() and SaveSharedState() functions in internal/store/state.go
- [x] T015 Extend Notification model Extensions struct with D-Bus fields in internal/model/notification.go (Actions, ImageData, SoundFile, DesktopEntry, Resident, Transient)

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Receive and Display Notifications (Priority: P1) 🎯 MVP

**Goal**: Receive D-Bus notifications and display them as styled popups on Wayland layer-shell surfaces

**Independent Test**: Run histuid, send `notify-send "Test" "Hello"`, verify popup appears on screen

### Implementation for User Story 1

- [x] T016 [US1] Implement NotificationServer struct in internal/dbus/server.go with nextID counter and channels
- [x] T017 [US1] Implement GetCapabilities method in internal/dbus/server.go (return supported features list)
- [x] T018 [US1] Implement GetServerInformation method in internal/dbus/server.go (histuid, histui, version, spec 1.2)
- [x] T019 [US1] Implement Notify method in internal/dbus/server.go (parse hints, generate ID, return ID)
- [x] T020 [US1] Implement CloseNotification method in internal/dbus/server.go
- [x] T021 [US1] Implement signal emission helpers in internal/dbus/signals.go (NotificationClosed, ActionInvoked)
- [x] T022 [US1] Implement D-Bus bus name claiming and object export in internal/dbus/server.go (Start method)
- [x] T023 [US1] Implement DisplayManager struct in internal/display/manager.go with popup tracking
- [x] T024 [US1] Implement popup window creation with layer-shell in internal/display/popup.go (InitLayerShell, SetLayer, SetAnchor)
- [x] T025 [US1] Implement notification widget construction in internal/display/widgets.go (icon, summary, body, app-name, timestamp)
- [x] T026 [US1] Implement Pango markup parsing for body text in internal/display/widgets.go
- [x] T027 [US1] Implement popup stacking and positioning in internal/display/layout.go (CalculatePosition, UpdateStack)
- [x] T028 [US1] Implement timeout handling in internal/display/manager.go (per-urgency timeouts, never-expire for critical)
- [x] T029 [US1] Implement multi-monitor support in internal/display/layout.go (0=all, 1+=specific, fallback to primary)
- [x] T030 [US1] Embed default CSS theme in internal/theme/default.go
- [x] T031 [US1] Implement GtkCssProvider loading in internal/theme/loader.go
- [x] T032 [US1] Apply urgency CSS classes to popups in internal/display/popup.go (urgency-low, urgency-normal, urgency-critical)
- [x] T033 [US1] Implement main daemon entry point in cmd/histuid/main.go (adw.Application, D-Bus + display integration)
- [x] T034 [US1] Implement graceful shutdown handling in cmd/histuid/main.go (SIGINT, SIGTERM)

**Checkpoint**: `notify-send` displays popup on screen with basic styling

---

## Phase 4: User Story 2 - Shared History with histui (Priority: P2)

**Goal**: Notifications received by histuid appear immediately in histui's history

**Independent Test**: Send notification, run `histui get --limit 1`, verify notification appears

### Implementation for User Story 2

- [x] T035 [US2] Connect D-Bus Notify handler to existing store.Add() in internal/dbus/server.go
- [x] T036 [US2] Set histui_source="histuid" when creating notifications in internal/dbus/server.go
- [x] T037 [US2] Implement DisplayState struct in internal/daemon/display_state.go (HistuiID, DBusID, Status, CreatedAt, ExpiresAt)
- [x] T038 [US2] Implement DisplayStateManager for mapping ULIDs to popup state in internal/daemon/display_state.go
- [x] T039 [US2] Implement store file watching in internal/daemon/hotreload.go for external state changes
- [x] T040 [US2] Connect store changes to popup updates in internal/display/manager.go (close popup when dismissed externally)
- [x] T041 [US2] Update popup state when notification dismissed via click in internal/display/popup.go
- [x] T042 [US2] Emit NotificationClosed signal with correct reason codes in internal/dbus/signals.go

**Checkpoint**: Notifications persist to store, `histui get` shows them, dismissing in histui closes popup

---

## Phase 5: User Story 3 - CSS Theming and Rich Rendering (Priority: P3)

**Goal**: Support custom CSS themes with urgency-based colors and animated images

**Independent Test**: Create custom CSS theme, configure histuid to use it, verify styling applies

### Implementation for User Story 3

- [x] T043 [US3] Implement Theme struct in internal/theme/theme.go (Name, Path, CSS, ModTime)
- [x] T044 [US3] Implement ThemeManager with LoadTheme() in internal/theme/loader.go
- [x] T045 [US3] Implement theme directory scanning in internal/theme/loader.go (~/.config/histui/themes/)
- [x] T046 [US3] Implement CSS hot-reload via file watching in internal/theme/watcher.go
- [x] T047 [US3] Apply CSS variables support via GtkCssProvider in internal/theme/loader.go
- [x] T048 [US3] Implement GdkPixbufAnimation wrapper for animated images in internal/display/animated.go
- [x] T049 [US3] Implement animated GIF/APNG loading from file path in internal/display/animated.go
- [x] T050 [US3] Implement animated image loading from D-Bus image-data hint in internal/display/animated.go
- [x] T051 [US3] Implement symbolic icon styling with CSS filters in internal/display/widgets.go
- [x] T052 [US3] Apply theme changes without daemon restart in internal/display/manager.go

**Checkpoint**: Custom CSS themes work, animated GIFs play, theme hot-reload works

---

## Phase 6: User Story 4 - Audio Notifications (Priority: P4)

**Goal**: Play per-urgency notification sounds

**Independent Test**: Configure sounds, send notifications of each urgency, verify correct sounds play

### Implementation for User Story 4

- [x] T053 [US4] Implement AudioPlayer interface in internal/audio/player.go (PlaySound, SetVolume, Stop)
- [x] T054 [US4] Implement beep-based audio playback in internal/audio/player.go (speaker.Init, streamer)
- [x] T055 [US4] Implement sound file loading with format detection in internal/audio/loader.go (WAV, OGG, MP3)
- [x] T056 [US4] Implement volume control via beep/effects in internal/audio/player.go
- [x] T057 [US4] Implement sound caching/preloading in internal/audio/loader.go
- [x] T058 [US4] Implement audio file hot-reload in internal/audio/watcher.go
- [x] T059 [US4] Connect notification urgency to sound selection in internal/daemon/daemon.go
- [x] T060 [US4] Handle audio playback failures gracefully (log warning, continue) in internal/audio/player.go

**Checkpoint**: Per-urgency sounds play, volume control works, missing files handled gracefully

---

## Phase 7: User Story 5 - Do Not Disturb Mode (Priority: P5)

**Goal**: Suppress popups and sounds while still persisting notifications

**Independent Test**: Enable DnD, send notification, verify no popup/sound but appears in `histui get`

### Implementation for User Story 5

- [x] T061 [US5] Read DnD state from SharedState on daemon startup in internal/daemon/daemon.go
- [x] T062 [US5] Watch state.json for DnD changes in internal/daemon/hotreload.go
- [x] T063 [US5] Suppress popup display when DnD enabled in internal/display/manager.go
- [x] T064 [US5] Suppress audio playback when DnD enabled in internal/audio/player.go
- [x] T065 [US5] Implement critical bypass option (show critical notifications despite DnD) in internal/daemon/daemon.go
- [x] T066 [US5] Continue persisting notifications to store during DnD in internal/dbus/server.go
- [x] T067 [US5] Implement `histui dnd` command in cmd/histui/dnd.go (on, off, toggle subcommands)
- [x] T068 [US5] Implement `histui dnd` quiet mode with exit codes in cmd/histui/dnd.go
- [x] T069 [US5] Extend `histui status` output with DnD state in cmd/histui/status.go

**Checkpoint**: DnD toggle works via CLI, popups/sounds suppressed, notifications still persist

---

## Phase 8: User Story 6 - Interactive Notification Actions (Priority: P6)

**Goal**: Support mouse clicks to dismiss, invoke actions, or close all

**Independent Test**: Click notification, verify it dismisses; middle-click, verify action invoked

### Implementation for User Story 6

- [ ] T070 [US6] Implement configurable mouse button actions in internal/display/popup.go (ConnectClicked)
- [ ] T071 [US6] Implement dismiss action in internal/display/popup.go (close popup, update store, emit signal)
- [ ] T072 [US6] Implement do-action handler in internal/display/popup.go (emit ActionInvoked signal)
- [ ] T073 [US6] Implement close-all action in internal/display/manager.go
- [ ] T074 [US6] Implement hover-to-show action buttons in internal/display/popup.go (ConnectEnter, ConnectLeave)
- [ ] T075 [US6] Implement action button widgets in internal/display/widgets.go (hidden by default, show on hover)
- [ ] T076 [US6] Implement pause-on-hover for timeout in internal/display/popup.go
- [ ] T077 [US6] Implement duplicate notification stacking with count in internal/display/manager.go
- [ ] T078 [US6] Display stack count "(2)" in popup when stacking in internal/display/widgets.go

**Checkpoint**: Mouse interactions work, action buttons appear on hover, stacking works

---

## Phase 9: User Story CLI Extensions (Priority: P2-continued)

**Goal**: Extend histui CLI with set command, filter flag, and ids format

**Independent Test**: Run `histui get --format ids | histui set --stdin --dismiss`, verify bulk operation works

### Implementation for CLI Extensions

- [ ] T079 [US2] Implement FilterExpr and FilterCondition structs in internal/core/filter.go
- [ ] T080 [US2] Implement ParseFilter() function in internal/core/filter.go (parse "field=value,field2>value2")
- [ ] T081 [US2] Implement FilterExpr.Match() method in internal/core/filter.go
- [ ] T082 [US2] Implement time duration parsing (5m, 1h, 1d, 1w) in internal/core/filter.go
- [ ] T083 [US2] Add --filter flag to `histui get` command in cmd/histui/get.go
- [ ] T084 [US2] Implement --format ids output adapter in internal/adapter/output/ids.go
- [ ] T085 [US2] Register ids formatter in output adapter registry
- [ ] T086 [US2] Implement `histui set` command in cmd/histui/set.go (ULID positional arg)
- [ ] T087 [US2] Implement --stdin flag for bulk operations in cmd/histui/set.go
- [ ] T088 [US2] Implement ULID extraction from stdin lines in cmd/histui/set.go (bare ULID or scan for pattern)
- [ ] T089 [US2] Implement --dismiss, --undismiss, --seen, --delete flags in cmd/histui/set.go
- [ ] T090 [US2] Implement --format json stdin parsing in cmd/histui/set.go (extract histui_id field)

**Checkpoint**: Full CLI pipeline works: get --filter | set --stdin --dismiss

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T091 [P] Add slog structured logging throughout daemon code
- [ ] T092 [P] Implement rate limiting for high-frequency notifications (100+ per second) in internal/dbus/server.go
- [ ] T093 [P] Handle screen resolution changes (reposition popups) in internal/display/layout.go
- [ ] T094 [P] Handle monitor disconnect (fallback to primary) in internal/display/layout.go
- [ ] T095 Implement Daemon struct orchestrating all components in internal/daemon/daemon.go
- [ ] T096 Implement lifecycle management (Start, Stop, Reload) in internal/daemon/lifecycle.go
- [ ] T097 [P] Add D-Bus Introspectable interface support in internal/dbus/server.go
- [ ] T098 [P] Create example waybar configuration in docs/waybar-example.json
- [ ] T099 [P] Create example CSS theme (catppuccin) in examples/themes/catppuccin.css
- [ ] T100 Run quickstart.md validation (full development workflow test)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in priority order (P1 → P2 → P3 → P4 → P5 → P6)
  - Some parallel opportunities within stories
- **Polish (Phase 10)**: Can start after Phase 3 (US1) is complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - Core D-Bus + Display
- **User Story 2 (P2)**: Depends on US1 - Extends store integration
- **User Story 3 (P3)**: Can start after US1 - Adds theming layer
- **User Story 4 (P4)**: Can start after US1 - Adds audio layer (independent of US2/US3)
- **User Story 5 (P5)**: Depends on US2 (shared state) - Adds DnD behavior
- **User Story 6 (P6)**: Depends on US1 - Adds interactivity layer
- **CLI Extensions**: Depends on US2 - Extends CLI commands

### Within Each User Story

- Models/types before services
- Services before handlers
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

**Setup Phase:**
```
T002, T003, T004, T005, T006, T007 can run in parallel (directory creation)
```

**Foundational Phase:**
```
T010, T011, T012, T014 can run in parallel (independent structs)
```

**User Story 1:**
```
T023-T027 (display) can start in parallel with T016-T022 (D-Bus) once T009 is complete
T030, T031 (theme) can run in parallel with display work
```

---

## Parallel Example: User Story 1

```bash
# Launch D-Bus implementation tasks:
T016: "Implement NotificationServer struct in internal/dbus/server.go"
T017: "Implement GetCapabilities method in internal/dbus/server.go"
T018: "Implement GetServerInformation method in internal/dbus/server.go"

# Parallel - Launch display implementation tasks:
T023: "Implement DisplayManager struct in internal/display/manager.go"
T024: "Implement popup window creation with layer-shell in internal/display/popup.go"
T030: "Embed default CSS theme in internal/theme/default.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Run `notify-send`, verify popup appears
5. This is the minimum viable notification daemon

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test with notify-send → MVP daemon works!
3. Add User Story 2 → Test with histui get → History integration works!
4. Add User Story 3 → Test with custom CSS → Theming works!
5. Add User Story 4 → Test with sound files → Audio works!
6. Add User Story 5 → Test with histui dnd → DnD works!
7. Add User Story 6 → Test with mouse clicks → Interactions work!
8. Add CLI Extensions → Test with pipelines → Full CLI workflow!
9. Each story adds value without breaking previous stories

### Suggested Implementation Order

Given the dependencies, the recommended order is:

1. **Phase 1-2**: Setup + Foundational (T001-T015)
2. **Phase 3**: User Story 1 (T016-T034) - GET POPUPS WORKING FIRST
3. **Phase 4**: User Story 2 (T035-T042) - Integrate with store
4. **Phase 9**: CLI Extensions (T079-T090) - Extends US2
5. **Phase 5**: User Story 3 (T043-T052) - Theming
6. **Phase 6**: User Story 4 (T053-T060) - Audio
7. **Phase 7**: User Story 5 (T061-T069) - DnD
8. **Phase 8**: User Story 6 (T070-T078) - Interactions
9. **Phase 10**: Polish (T091-T100)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- User Story 7 (Window Rules) is marked Future and NOT included in this task list
