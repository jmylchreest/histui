#!/usr/bin/env python3
"""Actual histuid D-Bus -> GTK -> layer-shell -> compositor screenshot tests.
Run only via run.sh, which starts a fresh private D-Bus session.
"""

import argparse
import hashlib
import json
import os
import select
import shutil
import subprocess
import tempfile
import time
from pathlib import Path

import dbus
from dbus.mainloop.glib import DBusGMainLoop
from gi.repository import GLib
from PIL import Image, ImageChops


def check(condition, message):
    if not condition:
        raise AssertionError(message)


def wait_until(predicate, message, timeout=12):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        while GLib.MainContext.default().iteration(False):
            pass
        if predicate():
            return
        time.sleep(0.05)
    raise AssertionError(message)


def stop(process):
    if process.poll() is None:
        process.terminate()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()


def pointer_reply(process):
    check(
        select.select([process.stdout], [], [], 3)[0],
        "Virtual pointer acknowledgement timed out",
    )
    line = process.stdout.readline().strip()
    check(line in ["ready", "ok"], f"Virtual pointer failed: {line!r}")


def move_pointer(process, x, y):
    process.stdin.write(f"move {x} {y}\n")
    process.stdin.flush()
    pointer_reply(process)


def capture(path):
    subprocess.run(["grim", "-o", "HEADLESS-1", str(path)], check=True, timeout=10)
    return Image.open(path).convert("RGB")


def difference(a, b):
    return ImageChops.difference(a, b).getbbox()


def main(args):
    DBusGMainLoop(set_as_default=True)
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    evidence = {
        "daemon_sha256": hashlib.sha256(args.daemon.read_bytes()).hexdigest(),
        "status": "failed",
        "tests": [],
        "runtime": subprocess.check_output(["sway", "--version"], text=True).strip(),
        "libraries": subprocess.check_output(
            (
                [
                    "dpkg-query",
                    "-W",
                    "libgtk-4-1",
                    "libadwaita-1-0",
                    "libgtk4-layer-shell0",
                    "fonts-dejavu-core",
                ]
                if shutil.which("dpkg-query")
                else [
                    "rpm",
                    "-q",
                    "gtk4",
                    "libadwaita",
                    "gtk4-layer-shell",
                    "dejavu-sans-fonts",
                ]
            ),
            text=True,
        ).strip(),
    }
    (out / "result.json").write_text(
        json.dumps({**evidence, "status": "running"}, indent=2)
    )
    with tempfile.TemporaryDirectory(prefix="histui-popup-") as temporary:
        root = Path(temporary)
        runtime = root / "runtime"
        runtime.mkdir(mode=0o700)
        os.environ.update(
            {
                "XDG_RUNTIME_DIR": str(runtime),
                "WLR_BACKENDS": "headless",
                "WLR_LIBINPUT_NO_DEVICES": "1",
                "WLR_RENDERER": "pixman",
                "GDK_BACKEND": "wayland",
                "GSK_RENDERER": "cairo",
                "GTK_A11Y": "none",
                "XDG_CURRENT_DESKTOP": "sway",
            }
        )
        with (out / "sway.log").open("wb") as log:
            sway = subprocess.Popen(
                [
                    "sway",
                    "--unsupported-gpu",
                    "-c",
                    str(Path(__file__).with_name("sway.conf")),
                ],
                stdout=log,
                stderr=log,
            )
            pointer = None
            try:
                wait_until(
                    lambda: (
                        list(runtime.glob("wayland-*.lock"))
                        and list(runtime.glob("sway-ipc.*.sock"))
                    ),
                    "Sway did not start",
                )
                socket = next(
                    p for p in runtime.glob("wayland-*") if not p.name.endswith(".lock")
                )
                os.environ["WAYLAND_DISPLAY"] = socket.name
                os.environ["SWAYSOCK"] = str(next(runtime.glob("sway-ipc.*.sock")))
                bus = dbus.SessionBus()
                actions = []
                closed = []
                bus.add_signal_receiver(
                    lambda notification_id, key: actions.append(
                        (int(notification_id), str(key))
                    ),
                    signal_name="ActionInvoked",
                    dbus_interface="org.freedesktop.Notifications",
                )
                bus.add_signal_receiver(
                    lambda notification_id, reason: closed.append(
                        (int(notification_id), int(reason))
                    ),
                    signal_name="NotificationClosed",
                    dbus_interface="org.freedesktop.Notifications",
                )
                check(
                    not bus.name_has_owner("org.freedesktop.Notifications"),
                    "Private bus already has notification service",
                )
                time.sleep(0.3)
                blank = capture(out / "blank.png")
                fixture = root / "image.png"
                im = Image.new("RGB", (160, 80))
                for y in range(im.height):
                    for x in range(im.width):
                        im.putpixel((x, y), (x * 255 // 160, y * 255 // 80, 128))
                im.save(fixture)
                themes = args.only or [
                    "default",
                    "minimal",
                    "compact",
                    "detailed",
                    "catppuccin",
                    "glass",
                ]
                for theme in themes:
                    for urgency, animated in (
                        [(0, False), (1, False), (2, False)]
                        + ([(2, True)] if theme in ["default", "glass"] else [])
                        + ([(1, True)] if theme == "glass" else [])
                    ):
                        # Animated default/glass contain intentional continuous critical effects.
                        if animated and theme not in ["default", "glass"]:
                            continue
                        case = (
                            theme
                            + f"-urgency-{urgency}"
                            + ("-animated" if animated else "-static")
                        )
                        home = root / case
                        home.mkdir()
                        env = {
                            **os.environ,
                            "HOME": str(home),
                            "XDG_CONFIG_HOME": str(home / "config"),
                            "XDG_DATA_HOME": str(home / "data"),
                            "XDG_CACHE_HOME": str(home / "cache"),
                            "XDG_STATE_HOME": str(home / "state"),
                        }
                        config = home / "config" / "histui"
                        config.mkdir(parents=True)
                        shutil.copytree(args.themes, config / "themes")
                        css = config / "themes" / theme / "theme.css"
                        if not animated:
                            with css.open("a") as f:
                                f.write(
                                    "\n/* Visual harness: freeze motion and hide wall-clock text. */\n.notification-popup.notification-popup.notification-popup, .notification-popup.notification-popup.notification-popup * { animation: none; transition: none; }\n.notification-timestamp { opacity: 0; }\n"
                                )
                        (config / "histuid.toml").write_text(
                            '[display]\nposition="top-right"\nmax_visible=3\nimage_data_preview_size="always"\n[theme]\ncolor_scheme="dark"\n[audio]\nenabled=false\n[timeouts]\nlow="never"\nnormal="0"\ncritical="never"\n'
                        )
                        with (out / (case + ".log")).open("wb") as log:
                            daemon = subprocess.Popen(
                                [
                                    str(args.daemon),
                                    "--theme",
                                    theme,
                                    "--no-audio",
                                    "--font",
                                    "DejaVu Sans",
                                    "--font-size",
                                    "14",
                                    "--log-level",
                                    "debug",
                                ],
                                env=env,
                                stdout=log,
                                stderr=log,
                            )
                            try:
                                wait_until(
                                    lambda daemon=daemon: (
                                        bus.name_has_owner(
                                            "org.freedesktop.Notifications"
                                        )
                                        or daemon.poll() is not None
                                    ),
                                    "Daemon did not claim private bus",
                                )
                                check(
                                    daemon.poll() is None,
                                    f"Daemon exited: inspect {case}.log",
                                )
                                remote = dbus.Interface(
                                    bus.get_object(
                                        "org.freedesktop.Notifications",
                                        "/org/freedesktop/Notifications",
                                    ),
                                    "org.freedesktop.Notifications",
                                )
                                remote.GetServerInformation(timeout=3)
                                time.sleep(0.2)
                                ids = []
                                actions.clear()
                                closed.clear()
                                interaction = False
                                timeout_verified = False
                                reload_verified = False
                                hints = {
                                    "urgency": dbus.Byte(urgency),
                                    "value": dbus.Int32(65),
                                    "image-path": dbus.String(str(fixture)),
                                }
                                ids.append(
                                    remote.Notify(
                                        "Visual fixture",
                                        dbus.UInt32(0),
                                        str(fixture),
                                        [
                                            "Low priority",
                                            "Build finished",
                                            "Critical alert",
                                        ][urgency],
                                        "Synthetic <b>formatted</b> body. A second sentence tests wrapping.",
                                        dbus.Array(
                                            [
                                                "default",
                                                "Open",
                                                "dismiss",
                                                "Dismiss",
                                            ],
                                            signature="s",
                                        ),
                                        dbus.Dictionary(hints, signature="sv"),
                                        dbus.Int32(0),
                                        timeout=3,
                                    )
                                )
                                frames = []
                                # Use actual elapsed times as evidence; compositor wall-clock frames
                                # cannot be treated as deterministic keyframe phase goldens.
                                start = time.monotonic()
                                for index in range(12 if animated else 3):
                                    time.sleep(0.2 if animated else 0.35)
                                    shot = capture(out / f"{case}-{index:02}.png")
                                    bounds = difference(blank, shot)
                                    check(
                                        bounds is not None, f"No popup visible: {case}"
                                    )
                                    check(
                                        0 < bounds[0] < bounds[2] < shot.width
                                        and 0 < bounds[1] < bounds[3] < shot.height,
                                        f"Popup clipped at output edge: {case}: {bounds}",
                                    )
                                    frames.append(
                                        (shot, round(time.monotonic() - start, 3))
                                    )
                                if animated:
                                    check(
                                        len(
                                            {
                                                hashlib.sha256(f[0].tobytes()).digest()
                                                for f in frames[4:]
                                            }
                                        )
                                        >= 3,
                                        f"Animation did not continue changing pixels after settling: {case}",
                                    )
                                    frames[0][0].save(
                                        out / (case + ".gif"),
                                        save_all=True,
                                        append_images=[f[0] for f in frames[1:]],
                                        duration=200,
                                        loop=0,
                                    )
                                else:
                                    check(
                                        difference(frames[-2][0], frames[-1][0])
                                        is None,
                                        f"Frozen screenshot not stable: {case}",
                                    )
                                    frames[-1][0].save(out / (case + ".png"))
                                    if args.baseline:
                                        expected = Image.open(
                                            args.baseline / (case + ".png")
                                        ).convert("RGB")
                                        check(
                                            expected.size == frames[-1][0].size,
                                            "Baseline dimensions differ",
                                        )
                                        diff = ImageChops.difference(
                                            expected, frames[-1][0]
                                        )
                                        if diff.getbbox():
                                            diff.save(out / (case + "-diff.png"))
                                        check(
                                            diff.getbbox() is None,
                                            f"Screenshot baseline differs: {case}",
                                        )
                                if not animated and urgency == 1:
                                    pointer = subprocess.Popen(
                                        [os.environ["HISTUI_TEST_POINTER"]],
                                        stdin=subprocess.PIPE,
                                        stdout=subprocess.PIPE,
                                        text=True,
                                    )
                                    pointer_reply(pointer)
                                    x = (bounds[0] + bounds[2]) // 2
                                    y = (bounds[1] + bounds[3]) // 2
                                    move_pointer(pointer, x, y)
                                    time.sleep(0.3)
                                    hover = capture(out / (case + "-hover.png"))
                                    if theme != "minimal":
                                        check(
                                            difference(frames[-1][0], hover)
                                            is not None,
                                            f"Hover did not reveal actions: {case}",
                                        )
                                        hover_bounds = difference(blank, hover)
                                        check(
                                            hover_bounds[3] > bounds[3],
                                            f"Action row did not expand popup: {case}",
                                        )
                                        move_pointer(
                                            pointer,
                                            hover_bounds[0] + 60,
                                            hover_bounds[3] - 35,
                                        )
                                        time.sleep(0.15)
                                        pointer.stdin.write("click\n")
                                        pointer.stdin.flush()
                                        pointer_reply(pointer)
                                        wait_until(
                                            lambda ids=tuple(ids): (
                                                (int(ids[0]), "dismiss") in actions
                                            ),
                                            f"Action button did not emit ActionInvoked: {case}",
                                        )
                                        wait_until(
                                            lambda ids=tuple(ids): any(
                                                i == int(ids[0]) for i, _ in closed
                                            ),
                                            f"Action did not close notification: {case}",
                                        )
                                        interaction = True
                                        ids = []
                                    stop(pointer)
                                    pointer = None
                                for notification_id in ids:
                                    remote.CloseNotification(notification_id, timeout=3)
                                time.sleep(0.4)
                                after = capture(out / (case + "-dismissed.png"))
                                check(
                                    difference(blank, after) is None,
                                    f"Popup left pixels after dismissal: {case}",
                                )
                                check(daemon.poll() is None, f"Daemon crashed: {case}")
                                if theme == "default" and urgency == 1 and not animated:
                                    timed_id = int(
                                        remote.Notify(
                                            "Visual fixture",
                                            dbus.UInt32(0),
                                            str(fixture),
                                            "Timeout pause fixture",
                                            "Keep this popup alive while hovered",
                                            dbus.Array(
                                                ["dismiss", "Dismiss"], signature="s"
                                            ),
                                            dbus.Dictionary(
                                                {
                                                    "urgency": dbus.Byte(1),
                                                    "image-path": dbus.String(
                                                        str(fixture)
                                                    ),
                                                },
                                                signature="sv",
                                            ),
                                            dbus.Int32(1200),
                                        )
                                    )
                                    time.sleep(0.15)
                                    timed = capture(out / (case + "-timeout-start.png"))
                                    timed_bounds = difference(blank, timed)
                                    check(
                                        timed_bounds is not None,
                                        "Timed popup did not appear",
                                    )
                                    pointer = subprocess.Popen(
                                        [os.environ["HISTUI_TEST_POINTER"]],
                                        stdin=subprocess.PIPE,
                                        stdout=subprocess.PIPE,
                                        text=True,
                                    )
                                    pointer_reply(pointer)
                                    move_pointer(
                                        pointer,
                                        (timed_bounds[0] + timed_bounds[2]) // 2,
                                        (timed_bounds[1] + timed_bounds[3]) // 2,
                                    )
                                    time.sleep(1.6)
                                    paused = capture(
                                        out / (case + "-timeout-paused.png")
                                    )
                                    check(
                                        difference(blank, paused) is not None,
                                        "Timed popup expired while hovered",
                                    )
                                    while GLib.MainContext.default().iteration(False):
                                        pass
                                    check(
                                        not any(i == timed_id for i, _ in closed),
                                        "Timed popup emitted close while hovered",
                                    )
                                    move_pointer(pointer, 1, 700)
                                    stop(pointer)
                                    pointer = None
                                    wait_until(
                                        lambda timed_id=timed_id: (
                                            (timed_id, 1) in closed
                                        ),
                                        "Timed popup did not expire after pointer left",
                                        timeout=4,
                                    )
                                    time.sleep(0.2)
                                    expired = capture(
                                        out / (case + "-timeout-expired.png")
                                    )
                                    check(
                                        difference(blank, expired) is None,
                                        "Expired popup left pixels",
                                    )
                                    timeout_verified = True
                                    reload_id = int(
                                        remote.Notify(
                                            "Visual fixture",
                                            dbus.UInt32(0),
                                            "",
                                            "Reload fixture",
                                            "Existing popup must change colour",
                                            dbus.Array([], signature="s"),
                                            dbus.Dictionary(
                                                {"urgency": dbus.Byte(1)},
                                                signature="sv",
                                            ),
                                            dbus.Int32(0),
                                        )
                                    )
                                    time.sleep(0.3)
                                    before_reload = capture(
                                        out / (case + "-reload-before.png")
                                    )
                                    reload_bounds = difference(blank, before_reload)
                                    check(
                                        reload_bounds is not None,
                                        "Reload fixture did not appear",
                                    )

                                    def magenta_pixels(shot, bounds=reload_bounds):
                                        return sum(
                                            count
                                            for count, colour in shot.crop(
                                                bounds
                                            ).getcolors(1280 * 720)
                                            if colour == (255, 0, 255)
                                        )

                                    check(
                                        magenta_pixels(before_reload) == 0,
                                        "Reload marker colour already present",
                                    )
                                    with css.open("a") as stylesheet:
                                        stylesheet.write(
                                            "\n.notification-popup.notification-popup { background-image: none; background-color: #ff00ff; }\n"
                                        )
                                    wait_until(
                                        lambda case=case, magenta_pixels=magenta_pixels: (
                                            magenta_pixels(
                                                capture(
                                                    out / (case + "-reload-after.png")
                                                )
                                            )
                                            > 500
                                        ),
                                        "Existing popup did not apply CSS hot reload",
                                        timeout=6,
                                    )
                                    check(
                                        not any(i == reload_id for i, _ in closed),
                                        "Reload replaced/closed the original notification",
                                    )
                                    control = dbus.Interface(
                                        bus.get_object(
                                            "org.freedesktop.Notifications",
                                            "/org/freedesktop/histui/Daemon",
                                        ),
                                        "org.freedesktop.histui.Daemon",
                                    )
                                    control.CloseAllNotifications(timeout=3)
                                    time.sleep(0.3)
                                    check(
                                        difference(
                                            blank,
                                            capture(
                                                out / (case + "-reload-dismissed.png")
                                            ),
                                        )
                                        is None,
                                        "Reload fixtures did not clear",
                                    )
                                    reload_verified = True
                                    if args.churn:
                                        # Exercise real D-Bus writers while GTK creates/removes
                                        # widgets and queue slots are promoted on the main thread.
                                        for wave in range(args.churn):
                                            wave_ids = []
                                            for item in range(12):
                                                wave_ids.append(int(remote.Notify(
                                                    "Churn fixture", dbus.UInt32(0), "",
                                                    f"Wave {wave} item {item}", "Queue and replacement",
                                                    dbus.Array(["open", "Open"], signature="s"),
                                                    dbus.Dictionary({"urgency": dbus.Byte(1)}, signature="sv"),
                                                    dbus.Int32(0), timeout=3,
                                                )))
                                            wait_until(lambda: len(control.GetActiveNotifications(timeout=3)) == 12,
                                                       "Churn notifications did not all become active/queued")
                                            previous_active = set(control.GetActiveNotifications(timeout=3))
                                            replacement = int(remote.Notify(
                                                "Churn fixture", dbus.UInt32(wave_ids[0]), "",
                                                f"Replaced wave {wave}", "Replacement",
                                                dbus.Array([], signature="s"),
                                                dbus.Dictionary({"urgency": dbus.Byte(1)}, signature="sv"),
                                                dbus.Int32(0), timeout=3,
                                            ))
                                            check(replacement == wave_ids[0], "Replacement changed D-Bus ID")
                                            # Notify acknowledges before the GTK idle applies the
                                            # replacement, which has a new history ID. Wait for that
                                            # transition before closing an ID from the active list.
                                            wait_until(
                                                lambda: len(ids := set(control.GetActiveNotifications(timeout=3))) == 12
                                                and ids != previous_active,
                                                "Replacement did not update active history IDs",
                                            )
                                            active = control.GetActiveNotifications(timeout=3)
                                            check(control.CloseNotification(active[0], timeout=3), "IPC close failed")
                                            control.CloseAllNotifications(timeout=3)
                                            wait_until(lambda: not control.GetActiveNotifications(timeout=3),
                                                       "IPC close-all left active/queued notifications")
                                            check(daemon.poll() is None, "Daemon died during churn")
                                        # Queue promotion from worker timeouts must also stay on GTK.
                                        timed = []
                                        for item in range(6):
                                            timed.append(int(remote.Notify(
                                                "Timeout churn", dbus.UInt32(0), "", f"Timed item {item}", "",
                                                dbus.Array([], signature="s"),
                                                dbus.Dictionary({"urgency": dbus.Byte(1)}, signature="sv"),
                                                dbus.Int32(150), timeout=3,
                                            )))
                                        wait_until(lambda: all(any(i == n and reason == 1 for i, reason in closed) for n in timed),
                                                   "Queued timeout notifications did not expire", timeout=10)
                                        time.sleep(0.2)
                                        check(difference(blank, capture(out / (case + "-churn-dismissed.png"))) is None,
                                              "Churn left visible popup pixels")
                                        # Exit with callbacks and widgets still pending, exercising
                                        # shutdown cancellation instead of only an empty daemon.
                                        for item in range(10):
                                            remote.Notify("Shutdown churn", dbus.UInt32(0), "", f"Pending {item}", "",
                                                          dbus.Array([], signature="s"),
                                                          dbus.Dictionary({"urgency": dbus.Byte(1)}, signature="sv"),
                                                          dbus.Int32(0), timeout=3)
                                        print(f"PASS D-Bus churn: {args.churn * 12} notifications, replacements, IPC and timeout queue", flush=True)
                            finally:
                                stop(daemon)
                                check(daemon.returncode == 0, f"Daemon shutdown failed: {case}: {daemon.returncode}")
                        wait_until(
                            lambda: (
                                not bus.name_has_owner("org.freedesktop.Notifications")
                            ),
                            "Daemon retained bus name",
                        )
                        text = (out / (case + ".log")).read_text(errors="replace")
                        check(
                            "loaded user theme (directory)" in text,
                            f"Requested user theme not loaded: {case}",
                        )
                        check(
                            not any(
                                s in text
                                for s in [
                                    "Theme parser error",
                                    "panic:",
                                    "CRITICAL",
                                    "WARNING: DATA RACE",
                                    "fatal error:",
                                    "failed to load theme",
                                ]
                            ),
                            f"Theme/runtime errors: {case}",
                        )
                        evidence["tests"].append(
                            {
                                "case": case,
                                "frames": len(frames),
                                "elapsed_seconds": [f[1] for f in frames],
                                "dismissed_to_blank": True,
                                "hover_and_action_verified": interaction,
                                "timeout_pause_resume_verified": timeout_verified,
                                "css_hot_reload_verified": reload_verified,
                                "churn_waves": args.churn if reload_verified else 0,
                            }
                        )
                        print("PASS", case, flush=True)
                evidence["status"] = "passed"
            finally:
                if pointer is not None:
                    stop(pointer)
                stop(sway)
                (out / "result.json").write_text(json.dumps(evidence, indent=2) + "\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--daemon", type=Path, required=True)
    parser.add_argument("--themes", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--baseline", type=Path)
    parser.add_argument("--only", nargs="+")
    parser.add_argument("--churn", type=int, default=0, help="12-notification queue/replacement waves in default normal case")
    main(parser.parse_args())
