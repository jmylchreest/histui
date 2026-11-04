# TUI visual regression tests

`TestVisualFrames` calls the production `Model.View()` with synthetic notifications
and fixed viewport sizes. Exact ANSI baselines gate text, layout and colour changes.
No user history, live daemon, database, D-Bus, clipboard or audio is opened.

Twelve fixtures cover empty/list/preview/narrow views, dismissed entries, empty
search, help, detail, detail scrolling, and halfblock images before/after scrolling
and cycling. Scroll/cycle cases exercise `Model.Update` with key messages.
Timestamp fixtures intentionally use 2100 so relative-time labels remain stable;
the resulting "in the future" text is expected. Colour profile is forced to true
colour and restored after the test. These tests must not run in parallel with
other tests that change Lip Gloss's global renderer.

Run the portable baseline checks:

```sh
CGO_ENABLED=0 HISTUI_VISUAL_ARTIFACTS=/tmp/histui-visual go test ./internal/tui -run '^TestVisualFrames$' -count=1
```

Capture actual ANSI frames in a real Kitty terminal on an isolated virtual display:

```sh
xvfb-run -a -s '-screen 0 1280x800x24' python3 scripts/capture-tui.py /tmp/histui-visual
```

Requires Python 3, Kitty, ImageMagick `import`/`identify`, Xvfb, xauth and DejaVu Sans
Mono. The script waits for the child to write its frame and for pixels to settle,
fails on missing/blank captures or timeouts, and cleans up its terminal processes.
Keep this invocation on the private Xvfb display, not the user's desktop.
PNGs, terminal logs and pixel hashes are written beside the actual ANSI files.
The terminal is larger than the fixture viewport to preserve all output for review.

CI has a separate visual job and uploads actual ANSI, PNGs and logs for 14 days,
including on baseline failures. UPDATE_VISUAL is never enabled by CI. The local
capture path has been executed; the new GitHub Actions job has not yet run.
Fedora rawhide follows the existing workflow; this is not a pinned pixel environment.

To deliberately update a baseline:

```sh
CGO_ENABLED=0 UPDATE_VISUAL=1 HISTUI_VISUAL_ARTIFACTS=/tmp/histui-visual go test ./internal/tui -run '^TestVisualFrames$' -count=1
```

Review the ANSI changes and generated PNGs before committing. Normal tests fail
when a baseline is missing or different. A deliberate mismatch was tested and
failed as expected. PNG hashes are evidence only: pixel comparisons across Kitty,
font and graphics-library versions are not portable, so CI gates exact ANSI.

## Live PTY acceptance

Exercise the real Bubble Tea input loop with a disposable SQLite database:

```sh
CGO_ENABLED=0 go build -o /tmp/histui-pty-bin ./cmd/histui
python3 scripts/test-tui-pty.py /tmp/histui-pty-bin /tmp/histui-visual/pty
```

The Python standard-library harness imports synthetic notifications through the
CLI, launches the TUI in a real PTY, navigates to a second notification, searches
for the first, opens detail, scrolls, resizes the terminal, refreshes, and quits.
It checks expected terminal output at each stage, a successful exit, and absence
of runtime error markers. XDG directories and the D-Bus address are isolated;
the harness cannot access the user's notification database or session bus.
Per-step ANSI/text logs, the complete transcript and acceptance.json are saved.
The same path runs in CI and its evidence is included in the visual artifact.

This acceptance test caught two bugs not exposed by View snapshots: a cursor
left beyond a filtered list's last item, and detail content lost on resize. Both
also have focused Go regression tests. GTK notification CSS/XML, focus, hover,
scale and output hotplug remain separate acceptance tests. Halfblock images are
covered by the deterministic View tests; native Kitty/Sixel images are not
introduced or claimed.

## Headless race tests

```sh
CGO_ENABLED=1 go test -tags headless -race ./internal/tui ./internal/theme ./internal/imagecapture
```

The opt-in `headless` tag excludes only the GTK CSS loader, allowing shared theme
metadata and TUI tests to run under Go's race detector without GTK build headers.
It is not a renderer backend or a flag for building histuid. Normal daemon builds
still include the GTK loader. The focused tests cover coalesced refreshes, stale
result rejection, worker/model ownership, totals and selection, watcher shutdown,
and image availability after persistence.
