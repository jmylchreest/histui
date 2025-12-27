# Tasks: GitHub Pages Documentation Site

**Input**: Design documents from `/specs/003-github-pages-docs/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Docusaurus project**: `docs/` at repository root
- **Content**: `docs/docs/` (Docusaurus content folder)
- **Static assets**: `docs/static/`
- **Workflows**: `.github/workflows/`

---

## Phase 1: Setup (Project Initialization)

**Purpose**: Initialize Docusaurus project and basic structure

- [x] T001 Initialize Docusaurus project in docs/ folder using `npx create-docusaurus@latest docs classic --typescript`
- [x] T002 Configure docusaurus.config.ts for GitHub Pages deployment (baseUrl, organizationName, projectName)
- [x] T003 [P] Create docs/src/css/custom.css with custom styling variables
- [x] T004 [P] Add logo and favicon to docs/static/img/
- [x] T005 Configure sidebars.ts with navigation structure from data-model.md
- [x] T006 [P] Add .gitignore entries for docs/node_modules/, docs/build/, docs/.docusaurus/
- [x] T007 Update root Taskfile.yml with docs commands (docs:dev, docs:build, docs:serve)

---

## Phase 2: Foundational (CI/CD Infrastructure)

**Purpose**: Set up deployment pipeline - MUST complete before content can be published

**⚠️ CRITICAL**: Documentation cannot be published until CI/CD is in place

- [x] T008 Create .github/workflows/docs.yml for deployment on push to main (from contracts/docs-workflow.yml)
- [x] T009 [P] Create .github/workflows/docs-version.yml for versioning on release (from contracts/docs-version-workflow.yml)
- [x] T010 Configure GitHub Pages in repository settings (source: GitHub Actions)
- [x] T011 Verify deployment pipeline with initial build

**Checkpoint**: CI/CD ready - documentation content can now be authored and deployed

---

## Phase 3: User Story 1 - New User Discovers and Installs histui (Priority: P1) 🎯 MVP

**Goal**: Provide clear introduction and quickstart guides so users can install and run histui within 5 minutes

**Independent Test**: Visit homepage, follow quickstart, verify histui runs with `histui --help`

### Implementation for User Story 1

- [x] T012 [US1] Create docs/docs/intro.md with project overview, key features, and links to quickstart
- [x] T013 [US1] Create docs/docs/quickstart/_category_.json for section metadata
- [x] T014 [P] [US1] Create docs/docs/quickstart/index.md with combined quickstart overview
- [x] T015 [P] [US1] Create docs/docs/quickstart/histui.md with histui CLI installation and basic usage
- [x] T016 [P] [US1] Create docs/docs/quickstart/histuid.md with histuid daemon installation and basic usage
- [x] T017 [US1] Create docs/src/pages/index.tsx homepage with hero section and feature highlights
- [x] T018 [US1] Add installation commands for common package managers (AUR, source build)
- [x] T019 [US1] Verify quickstart can be completed in under 5 minutes

**Checkpoint**: User Story 1 complete - new users can discover and install histui

---

## Phase 4: User Story 2 - User Searches for Information (Priority: P1)

**Goal**: Enable full-text search across all documentation

**Independent Test**: Search for "filter", "theme", "dnd" and verify relevant results appear within 1 second

### Implementation for User Story 2

- [x] T020 [US2] Install @cmfcmf/docusaurus-search-local: `npm install @cmfcmf/docusaurus-search-local`
- [x] T021 [US2] Add local search plugin configuration to docusaurus.config.ts (from contracts/docusaurus-config.ts)
- [x] T022 [US2] Build and verify search works: `npm run build && npm run serve`
- [x] T023 [US2] Verify search returns relevant results for key terms (filter, theme, configuration)
- [x] T024 [US2] Test version-aware search (results match selected version)

**Note**: Local search only works on built site (`npm run serve`), not during development (`npm start`).

**Checkpoint**: User Story 2 complete - users can search documentation

---

## Phase 5: User Story 3 - User Customizes histuid Theme (Priority: P2)

**Goal**: Comprehensive theming documentation with CSS examples and design tokens

**Independent Test**: Follow theming guide to create custom theme, verify it applies to notifications

### Implementation for User Story 3

- [x] T026 [US3] Create docs/docs/histuid/theming/_category_.json for section metadata
- [x] T027 [P] [US3] Migrate existing docs/THEMING.md content to docs/docs/histuid/theming/index.md
- [x] T028 [P] [US3] Create docs/docs/histuid/theming/css-reference.md with all CSS variables and selectors
- [x] T029 [P] [US3] Create docs/docs/histuid/theming/examples.md with complete theme examples (Catppuccin, Nord, Dracula)
- [x] T030 [US3] Add docs/static/examples/themes/ with example theme CSS files
- [ ] T031 [US3] Add screenshots of different themes to docs/static/img/screenshots/themes/
- [x] T032 [US3] Add upstream citation for GTK4 CSS reference

**Checkpoint**: User Story 3 complete - users can customize notification themes

---

## Phase 6: User Story 4 - User References Specific Version (Priority: P2)

**Goal**: Version selector shows all releases, documentation matches installed version

**Independent Test**: Select a version from dropdown, verify content reflects that version

### Implementation for User Story 4

- [x] T033 [US4] Configure version dropdown in docusaurus.config.ts navbar
- [x] T034 [US4] Configure lastVersion and versions settings in docs preset
- [ ] T035 [US4] Create initial version snapshot with `npm run docusaurus docs:version 0.1.0` (or current version) - awaits first release
- [x] T036 [US4] Verify version selector appears and works correctly
- [x] T037 [US4] Document versioning process in CONTRIBUTING.md or similar

**Checkpoint**: User Story 4 complete - users can access version-specific documentation

---

## Phase 7: User Story 5 - User Asks AI for Help (Priority: P3) - DEFERRED

**Goal**: AI-assisted search provides contextual answers from documentation

**Status**: DEFERRED - Requires external service dependency (Algolia or Kapa.ai)

**Future Implementation Path**:
- Apply to Kapa.ai OSS program (https://kapa.ai/oss) when documentation site is established
- Alternative: Evaluate Algolia DocSearch if service dependency becomes acceptable

~~- [ ] T038 [US5] Enable AI assistant in docusaurus.config.ts~~
~~- [ ] T039 [US5] Add fallback search suggestions for common queries~~
~~- [ ] T040 [US5] Document AI search limitations in a help/faq section~~
~~- [ ] T041 [US5] Test AI responses for accuracy against actual CLI behavior~~

**Checkpoint**: User Story 5 deferred - revisit after documentation site is stable

---

## Phase 8: User Story 6 - Contributor Updates Documentation (Priority: P3)

**Goal**: Clear contribution workflow with local preview capability

**Independent Test**: Clone repo, edit markdown, run local preview, see changes

### Implementation for User Story 6

- [x] T042 [US6] Create CONTRIBUTING.md with documentation contribution guide
- [x] T043 [US6] Add docs/README.md with local development instructions
- [x] T044 [US6] Verify `npm start` works for local preview in docs/ folder
- [x] T045 [US6] Add PR template for documentation changes
- [ ] T046 [US6] Configure markdownlint or similar for content quality - optional

**Checkpoint**: User Story 6 complete - contributors can easily update documentation

---

## Phase 9: Content - histui CLI Documentation

**Purpose**: Complete command reference and filtering guide for histui CLI

- [x] T047 Create docs/docs/histui/_category_.json for section metadata
- [x] T048 Create docs/docs/histui/commands/_category_.json for commands section
- [x] T049 [P] Create docs/docs/histui/commands/get.md with full command reference from `histui get --help`
- [x] T050 [P] Create docs/docs/histui/commands/prune.md with full command reference from `histui prune --help`
- [x] T051 [P] Create docs/docs/histui/commands/status.md with full command reference from `histui status --help`
- [x] T052 [P] Create docs/docs/histui/commands/tui.md with full command reference from `histui tui --help`
- [x] T053 Create docs/docs/histui/filtering.md with filter syntax, operators, and examples
- [x] T054 Add Waybar integration example to status command docs (upstream citation)
- [ ] T055 Add screenshots of TUI to docs/static/img/screenshots/tui/

---

## Phase 10: Content - histuid Daemon Documentation

**Purpose**: Complete configuration reference and operational documentation for histuid

- [x] T056 Create docs/docs/histuid/_category_.json for section metadata
- [x] T057 Create docs/docs/histuid/configuration.md with full config reference from daemon.go defaults
- [x] T058 Add docs/static/examples/histuid.toml with annotated example configuration
- [x] T059 Create docs/docs/histuid/monitor-mode.md explaining passive monitoring alongside dunst
- [x] T060 Add upstream citation for D-Bus Notifications spec
- [ ] T061 Add screenshots of notification popups to docs/static/img/screenshots/popups/

---

## Phase 11: Polish & Cross-Cutting Concerns

**Purpose**: Final refinements affecting multiple pages

- [x] T062 [P] Add consistent frontmatter (title, description, keywords) to all pages
- [x] T063 [P] Verify all internal links work (no broken links)
- [x] T064 [P] Add "Edit this page" links to all content pages
- [ ] T065 Run accessibility check and fix any issues (target: 90+ score) - manual verification
- [ ] T066 Verify mobile responsiveness on all pages - manual verification
- [x] T067 Add OpenGraph/social sharing metadata - user declined
- [ ] T068 Create 404 page with helpful navigation - optional (Docusaurus has default)
- [x] T069 Final build verification: `npm run build` succeeds without warnings
- [x] T070 Validate contributor quickstart.md from specs/ works correctly

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS publishing
- **User Story 1 (Phase 3)**: Depends on Foundational - MVP target
- **User Story 2 (Phase 4)**: Depends on Foundational - local search, no external dependency
- **User Story 3-6 (Phases 5-8)**: Depend on Foundational - can run in parallel
- **Content Phases (9-10)**: Can run in parallel with User Stories after Setup
- **Polish (Phase 11)**: Depends on all content being complete

### User Story Dependencies

| Story | Depends On | Independently Testable |
|-------|------------|------------------------|
| US1 - Quickstart | Foundational only | Yes - visit site, follow guide |
| US2 - Search | Foundational only (local search) | Yes - search for terms |
| US3 - Theming | Foundational only | Yes - create custom theme |
| US4 - Versioning | Foundational only | Yes - switch versions |
| US5 - AI | DEFERRED | N/A - requires external service |
| US6 - Contributing | Foundational only | Yes - local preview |

### Parallel Opportunities

**Phase 1 (Setup)**: T003, T004, T006 can run in parallel

**Phase 2 (Foundational)**: T008, T009 can run in parallel

**Phase 3 (US1)**: T014, T015, T016 can run in parallel

**Phase 5 (US3)**: T027, T028, T029 can run in parallel

**Phase 9 (histui docs)**: T049, T050, T051, T052 can run in parallel

**Phase 11 (Polish)**: T062, T063, T064 can run in parallel

---

## Parallel Example: Phase 9 (histui CLI Documentation)

```bash
# Launch all command reference pages together:
Task: "Create docs/docs/histui/commands/get.md"
Task: "Create docs/docs/histui/commands/prune.md"
Task: "Create docs/docs/histui/commands/status.md"
Task: "Create docs/docs/histui/commands/tui.md"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T007)
2. Complete Phase 2: Foundational (T008-T011)
3. Complete Phase 3: User Story 1 (T012-T019)
4. **STOP and VALIDATE**: Site is live, users can discover and install
5. Deploy to GitHub Pages

### Incremental Delivery

1. **MVP**: Setup + Foundational + US1 → Quickstart available
2. **+Search**: US2 (after Algolia approval) → Documentation searchable
3. **+Theming**: US3 → Theme customization documented
4. **+Versions**: US4 → Version-specific docs
5. **+Content**: Phases 9-10 → Full reference documentation
6. **+Polish**: Phase 11 → Production-ready

### External Dependencies

- **GitHub Pages**: Requires repository settings update (T010)
- ~~**Algolia DocSearch**: No longer required - using local search instead~~

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Content phases (9-10) can proceed in parallel with user story implementations
- Local search has no external blockers (unlike Algolia which required approval)
- Search only works on built site (`npm run serve`), not during development (`npm start`)
- Screenshots are manual but can be added incrementally
- AI assistant (US5) deferred - can revisit with Kapa.ai OSS program when site is established
