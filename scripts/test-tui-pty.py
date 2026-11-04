#!/usr/bin/env python3
"""Exercise the actual histui event loop in a PTY with an isolated fixture DB.
Usage: python3 scripts/test-tui-pty.py HISTUI_BINARY ARTIFACT_DIRECTORY
No graphical display, live D-Bus session, user history or extra Python packages.
"""
import errno
import fcntl
import json
import os
from pathlib import Path
import pty
import re
import select
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time

ANSI = re.compile(rb"\x1b\][^\x07]*(?:\x07|\x1b\\)|\x1b\[[0-?]*[ -/]*[@-~]|\x1b[()][0-2A-Z]|\x1b[=>]")


def run(binary, artifacts):
    artifacts.mkdir(parents=True, exist_ok=True)
    evidence = {"binary": str(binary), "steps": []}
    with tempfile.TemporaryDirectory(prefix="histui-pty-") as temporary:
        root = Path(temporary)
        env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor",
                   XDG_CONFIG_HOME=str(root / "config"), XDG_DATA_HOME=str(root / "data"),
                   XDG_CACHE_HOME=str(root / "cache"), XDG_RUNTIME_DIR=str(root / "runtime"),
                   DBUS_SESSION_BUS_ADDRESS="unix:path=" + str(root / "no-session-bus"))
        (root / "runtime").mkdir(mode=0o700)
        fixtures = [{"app_name": "PTYFixture", "summary": "Alpha Fixture", "timestamp": 200,
                     "body": "\n".join(f"BodyLine {i:03d} alpha scrolling content" for i in range(80))},
                    {"app_name": "PTYFixture", "summary": "Beta Fixture", "timestamp": 100,
                     "body": "Beta detail navigation succeeded"}]
        subprocess.run([str(binary), "get", "--source", "stdin", "--format", "ids"],
                       input=json.dumps(fixtures).encode(), env=env, check=True,
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=15)
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 30, 100, 0, 0))
        process = subprocess.Popen([str(binary), "tui", "--source", "histuid"], stdin=slave,
                                   stdout=slave, stderr=slave, env=env, start_new_session=True)
        os.close(slave)
        transcript = bytearray()

        def collect(timeout=0.35):
            chunk = bytearray()
            deadline = time.monotonic() + timeout
            while time.monotonic() < deadline:
                ready, _, _ = select.select([master], [], [], min(.05, max(0, deadline-time.monotonic())))
                if ready:
                    try:
                        data = os.read(master, 65536)
                    except OSError as error:
                        if error.errno == errno.EIO:
                            break
                        raise
                    if not data:
                        break
                    transcript.extend(data)
                    chunk.extend(data)
            return bytes(chunk)

        def step(name, keys=b"", expected=None, resize=None):
            if keys:
                os.write(master, keys)
            if resize:
                rows, columns = resize
                fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", rows, columns, 0, 0))
                os.kill(process.pid, signal.SIGWINCH)
            chunk = bytearray()
            deadline = time.monotonic() + 8
            while time.monotonic() < deadline:
                chunk.extend(collect())
                plain = ANSI.sub(b"", bytes(chunk)).decode("utf-8", "replace")
                if expected is None or expected in plain:
                    break
                if process.poll() is not None:
                    break
            (artifacts / (name + ".ansi")).write_bytes(chunk)
            (artifacts / (name + ".txt")).write_text(plain)
            if expected is not None and expected not in plain:
                raise AssertionError(f"{name}: missing {expected!r}; see {artifacts / (name + '.txt')}")
            if process.poll() is not None and name != "quit":
                raise AssertionError(f"{name}: unexpected exit {process.returncode}")
            evidence["steps"].append({"name": name, "expected": expected, "bytes": len(chunk), "resize": resize})

        try:
            step("startup", expected="Alpha Fixture")
            step("preview-off", b"p")
            step("navigate-down", b"j")
            step("beta-detail", b"\r", expected="Beta detail navigation succeeded")
            step("back-to-list", b"q", expected="Notification History")
            step("open-search", b"/", expected="Search")
            step("search", b"Alpha", expected="Alpha")
            step("alpha-detail", b"\r", expected="BodyLine 000")
            step("scroll-detail", b"\x1b[B" * 15, expected="BodyLine 030")
            step("resize-detail", resize=(24, 72), expected="BodyLine")
            step("back-after-resize", b"q", expected="Notification History")
            step("refresh", b"r")
            step("quit", b"q")
            process.wait(timeout=5)
            if process.returncode != 0:
                raise AssertionError(f"unclean exit {process.returncode}")
            plain = ANSI.sub(b"", bytes(transcript)).decode("utf-8", "replace")
            for error in ("panic:", "fatal error:", "DATA RACE", "failed to refresh", "Error:"):
                if error.lower() in plain.lower():
                    raise AssertionError(f"runtime error marker {error!r}")
            evidence["exit_code"] = process.returncode
            evidence["passed"] = True
        finally:
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
            os.close(master)
            (artifacts / "session.ansi").write_bytes(transcript)
            (artifacts / "acceptance.json").write_text(json.dumps(evidence, indent=2) + "\n")
    print("PTY acceptance passed:", artifacts / "acceptance.json")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    run(Path(sys.argv[1]).resolve(), Path(sys.argv[2]).resolve())
