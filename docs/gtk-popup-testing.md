# GTK popup screenshot tests

The harness runs the actual histuid executable in a private D-Bus session and a
headless Sway compositor. It sends synthetic Freedesktop notifications through
D-Bus, uses the production theme loader and XML widget builder, then captures the
compositor output with grim. This is not the standalone puregotk widget probe.
It never connects to the user's compositor, notification bus, config or history.
Audio is disabled. Each case gets fresh XDG config/data/cache directories.

## Coverage

- All six bundled themes at low, normal and critical urgency: 18 static cases.
- Synthetic icon and body image, formatted body, progress and action hints.
- Verified normal-priority hover expansion and actual action-button clicks on all
  five layouts with action rows. Matching D-Bus ActionInvoked and close signals
  are required. Minimal deliberately omits the action row.
- Default-theme timed notification stays visible while hovered past its deadline,
  then expires with D-Bus reason 1 after the pointer leaves.
- Default-theme CSS hot reload changes the background of an already-open popup;
  a unique colour is checked inside its original bounds and its ID must not close.
- Critical glow on default and glass, and normal glass shimmer: 3 animation cases.
- Each static frame must settle exactly; popup bounds must remain inside the
  output; closing the notification must return the output to the empty baseline.
- Animated cases sample 12 frames and require at least three distinct later
  frames, so initial widget realization cannot alone satisfy the animation check.
- Daemon liveness, actual requested theme load and GTK theme warnings are checked.

Static cases append a high-specificity test-only CSS override disabling animation
and transitions and hiding wall-clock timestamp text. Real theme CSS/XML, colours,
variables and dimensions otherwise remain in use. Animated cases use unmodified
source themes. GIF timings are illustrative; result.json records actual capture
elapsed times. This tests continuing motion, not precise frame pacing or every
animation in the shared effects library.

## Run

Build the production daemon (CGO-free Codeberg GTK bindings):

```sh
sh scripts/puregotk/go.sh build -o /tmp/histuid-visual ./cmd/histuid
podman build -t histui-popup-tests -f scripts/gtk-popup/Containerfile scripts/gtk-popup
mkdir -p /tmp/histui-popup-results
podman run --rm --network=none --userns=keep-id --user "$(id -u):$(id -g)" \
  -v /tmp/histuid-visual:/daemon:ro \
  -v "$PWD/embed/themes:/themes:ro" \
  -v "$PWD/scripts/gtk-popup:/tests:ro" \
  -v /tmp/histui-popup-results:/artifacts \
  histui-popup-tests sh /tests/run.sh --daemon /daemon --themes /themes --out /artifacts
```

The binary must be compatible with the container's native libraries. The supplied
runtime uses Debian 13, GTK 4.18 and libadwaita 1.7. Record/pin an image digest for
cross-run exact pixel baselines; rebuilding a floating distro image may update
fonts/rendering libraries. result.json records native versions and daemon SHA256.

On a machine with the dependencies already installed, run as an unprivileged user:

```sh
sh scripts/gtk-popup/run.sh --daemon /absolute/path/histuid --themes "$PWD/embed/themes" --out /tmp/histui-popup-results
```

Dependencies: Sway, swaybg, grim, dbus-run-session, Python dbus/Pillow/GObject, DejaVu Sans,
C compiler, pkg-config, wayland-scanner and Wayland client development headers,
and the daemon's GTK/libadwaita/layer-shell runtime dependencies. The run script
always starts a new D-Bus session; capture.py starts its own Wayland compositor.

## Baselines and artifacts

Output includes full-frame PNGs, hover/dismissal PNGs, animated GIFs, per-frame
animation PNGs, daemon/compositor logs and machine-readable result.json.

The checked-in Debian baselines are in `scripts/gtk-popup/baselines/debian13-gtk4.18`.
All 18 static frames reproduced exactly in a second integration run. A one-pixel
reference corruption correctly failed and produced a diff image.

For exact regression comparison, mount a reviewed previous output directory at
/baseline and pass `--baseline /baseline`. Static PNG differences fail the run and
produce a `*-diff.png`. Baselines are never updated automatically. Compare only
matching runtime/font/scale environments. `--only default glass` narrows a run.

The existing CI build job now includes a private popup capture step and uploads
artifacts for 14 days. It uses the existing Fedora runtime, so it gates within-run
stability/motion/dismissal and warning checks; it does not compare Fedora pixels
against Debian PNGs. The original gotk4 CI step passed; the updated puregotk workflow needs a remote CI run.

## Remaining limits

No compositor blur is present in standard Sway, so glass transparency is exercised
but Hyprland/SwayFX blur is not. No ARM64 runtime, HiDPI/fractional scale, monitor hotplug, keyboard focus or all
action types are covered. Timeout and CSS hot-reload checks currently exercise
the default theme only; config/layout-file reload is not covered. One software rendering backend/output is tested. This is
not a guarantee of native memory/callback safety on every runtime. Native lifetime
and race tests complement the screenshot cases.

## Finding from real-widget coverage

The prior parse-only probe missed deferred CSS validation: the glass background
used `calc()` inside GTK's legacy `alpha()` function. GTK 4.18 rejected the gradient
when applied to widgets. Four expressions now use relative `rgb()` with an alpha
calculation, retaining the same custom properties and arithmetic (including the
source colour's alpha multiplier). The harness rejects the old form via daemon
GTK warning logs. See https://docs.gtk.org/gtk4/css-properties.html for supported
standard/legacy colour forms. The separate theme experiment contains this fix.

## Executed integration validation (2026-11-03)

Before rebasing onto newer main dependencies, all 21 cases passed with the integration daemon on Debian 13 (GTK 4.18.6,
libadwaita 1.7.6, Sway 1.10.1). The complete matrix passed again with exact static
baseline comparison. The initial older dist binary was used only to develop the
harness; the final evidence records the new executable's hash.

The successful local build used CGO_CFLAGS/CGO_CXXFLAGS='-O0 -g0' and
-gcflags='github.com/diamondburned/...=-l' to reduce binding compilation cost,
with Go 1.27.3. This is a functional integration test build, not a release size
or performance measurement. The normal optimized build did not complete in the
initial attempt. The running desktop daemon was not replaced.

## Persistent input helper

`run.sh` builds `pointer.c` in a temporary directory and deletes it on exit. It
uses the vendored MIT-licensed wlr virtual-pointer protocol from
https://github.com/swaywm/wlr-protocols/blob/master/unstable/wlr-virtual-pointer-unstable-v1.xml
(retrieved 2026-11-03). This is test tooling only; no new binding or native-library
requirement is added to the production application. Coordinates target the fixed
1280x720 test output. Each helper command has an acknowledgement and a timeout.

A persistent device matters: one-shot tools destroy their virtual pointer before
the screenshot and GTK loses hover. Automatic cursor hiding also ends hover.
The helper exists only during interaction; static captures retain their original
cursor-free pixel baselines. Hover captures may show the cursor.

## Updated main dependency runtime (2026-11-03)

The optimized build after rebasing onto main completed with gotk4 v0.4.1.
That executable cannot start in the Debian 13 fixture runtime: the loader reports
undefined symbol `g_ecn_code_point_get_type`. The older Debian screenshot evidence
does not validate this updated binary. A GTK4 installation alone is insufficient;
the GLib/GTK versions must satisfy the bindings and build environment.

`Containerfile.fedora` supplies the current Fedora runtime used by CI. Use it
in the build command instead of `Containerfile` for the updated native bindings.
Keep the Debian image/baselines for compatible builds; do not compare pixels
across these two runtimes. Broad distribution compatibility remains a separate
release requirement and must not be inferred from passing Fedora tests.

The optimized current-PR executable subsequently passed all 21 cases on Fedora
(GTK 4.24.1, libadwaita 1.10.0), including hover/actions, timeout pause/resume
and CSS hot reload. Daemon SHA256:
`4917de8a48600ad04dfea311a400115dbb8dc41bdb7ac3abcb8b07e7d66a0c25`.
This confirms the current build on that runtime, not Debian compatibility.

## CGO-free production port (2026-11-04)

All 21 cases passed on both Fedora and Debian 13. On the same Fedora image, all
18 static frames matched the running gotk4 integration build byte-for-byte at the
pixel level. The port also passes 25 waves of 12 notifications with queueing,
replacement, IPC close/close-all, timeout-driven queue promotion and shutdown with
pending notifications (`--churn 25`). Nonzero daemon shutdown and native criticals
now fail the harness. The default-theme/churn cases also pass with a race build.

`HISTUI_TEST_GTK=1 GDK_BACKEND=x11 xvfb-run -a sh scripts/puregotk/go.sh test
./internal/display ./internal/theme` separately exercises 2,100 popup lifetimes,
checks all 25,200 tracked native objects finalize, and tests cancellation/reload.
Use `CGO_ENABLED=1 ... test -race` for the race-instrumented variant.
