#!/usr/bin/env python3
"""Capture synthetic View fixtures in Kitty on a private Xvfb display.
Run: xvfb-run -a -s '-screen 0 1280x800x24' python3 scripts/capture-tui.py ARTIFACT_DIR
Requires kitty, ImageMagick (import/identify), Xvfb, DejaVu Sans Mono.
ANSI goldens gate layout/colour changes; PNGs are review artifacts, not portable
pixel goldens across terminal/font/graphics versions. Never uses the live daemon.
"""
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time


def run(*args):
    return subprocess.check_output(args, stderr=subprocess.STDOUT, timeout=15)


def capture(directory):
    paths = sorted(directory.glob("*.ansi"))
    if not paths:
        raise SystemExit("No ANSI artifacts; run TestVisualFrames with HISTUI_VISUAL_ARTIFACTS first")
    evidence = {"kitty": run("kitty", "--version").decode().strip(), "frames": []}
    for path in paths:
        with tempfile.TemporaryDirectory(prefix="histui-frame-") as temporary:
            ready = Path(temporary) / "ready"
            # Hide the cursor and park the child after writing the complete frame.
            child = "import os,pathlib,signal,sys; os.write(1,b'\\x1b[2J\\x1b[H\\x1b[?25l'+pathlib.Path(sys.argv[1]).read_bytes()); pathlib.Path(sys.argv[2]).touch(); signal.pause()"
            command = ["kitty", "--config", "NONE", "--override", "linux_display_server=x11",
                       "--override", "font_family=DejaVu Sans Mono", "--override", "font_size=10",
                       "--override", "initial_window_width=1100", "--override", "initial_window_height=700",
                       "--override", "remember_window_size=no", "--override", "window_padding_width=0",
                       "--override", "background=#000000", "--override", "foreground=#dddddd",
                       "--override", "cursor_blink_interval=0", "--override", "enable_audio_bell=no",
                       "--override", "sync_to_monitor=no", "--title", "histui-visual-fixture",
                       sys.executable, "-c", child, str(path.resolve()), str(ready)]
            with (directory / (path.stem + ".terminal.log")).open("wb") as log:
                process = subprocess.Popen(command, stdout=log, stderr=log, start_new_session=True,
                                           env={**os.environ, "LIBGL_ALWAYS_SOFTWARE": "1"})
                try:
                    deadline = time.monotonic() + 15
                    while not ready.exists():
                        if process.poll() is not None or time.monotonic() > deadline:
                            raise RuntimeError(f"Terminal failed for {path}; inspect terminal log")
                        time.sleep(.05)
                    previous = None
                    stable = 0
                    while time.monotonic() < deadline:
                        time.sleep(.15)
                        pixels = run("import", "-window", "root", "-depth", "8", "rgb:-")
                        digest = hashlib.sha256(pixels).hexdigest()
                        stable = stable + 1 if digest == previous else 0
                        previous = digest
                        if stable >= 2:
                            break
                    else:
                        raise RuntimeError(f"Frame did not settle: {path}")
                    png = directory / (path.stem + ".png")
                    run("import", "-window", "root", str(png))
                    colours = int(run("identify", "-format", "%k", str(png)))
                    if colours < 8:
                        raise RuntimeError(f"Blank/near-blank capture: {png}")
                    evidence["frames"].append({"name": path.stem, "pixel_sha256": digest, "colours": colours})
                    print("Captured", png)
                finally:
                    if process.poll() is None:
                        os.killpg(process.pid, signal.SIGTERM)
                    try:
                        process.wait(timeout=5)
                    except subprocess.TimeoutExpired:
                        os.killpg(process.pid, signal.SIGKILL)
                        process.wait()
    (directory / "screenshots.json").write_text(json.dumps(evidence, indent=2) + "\n")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit(__doc__)
    capture(Path(sys.argv[1]).resolve())
