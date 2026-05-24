#!/usr/bin/env python3
"""Drive nook in a PTY to record the first-run welcome experience.

The cast shows what a brand-new user sees:

    1. Land on the welcome card (wordmark, project info, AI/LSP status, quick-start keys)
    2. Press ? to open the full keymap overlay
    3. Esc to dismiss
    4. Ctrl+P to open the file picker
    5. Type a filter, Enter to open, type a line, Ctrl+S to save
    6. Ctrl+Q to quit

Run from the repo root once nook is built:

    go build -o cmd/nook/nookbin ./cmd/nook
    mkdir -p /tmp/nook-welcome-fixture
    cat > /tmp/nook-welcome-fixture/hello.go <<'EOF'
    package main

    import "fmt"

    func main() {
        fmt.Println("hello, nook")
    }
    EOF
    (cd /tmp/nook-welcome-fixture && git init -q && git add . && git commit -q -m init)
    python3 docs/nook/visuals/record-welcome-cast.py \\
        docs/nook/visuals/welcome.cast \\
        cmd/nook/nookbin /tmp/nook-welcome-fixture

Render to GIF with agg:

    docker run --rm -v $(pwd):/work ghcr.io/asciinema/agg:latest \\
        --theme github-dark --speed 1.2 --font-size 14 \\
        /work/docs/nook/visuals/welcome.cast /work/docs/nook/visuals/welcome.gif
"""
import fcntl
import json
import os
import pty
import re
import select
import struct
import sys
import termios
import time

COLS = 120
ROWS = 34

OSC_BG_QUERY = re.compile(rb"\x1b\]11;\?\x1b\\")
OSC_FG_QUERY = re.compile(rb"\x1b\]10;\?\x1b\\")
CSI_CURSOR_QUERY = re.compile(rb"\x1b\[6n")
DA_QUERY = re.compile(rb"\x1b\[c|\x1b\[>c")


def respond(fd, chunk):
    if OSC_BG_QUERY.search(chunk):
        os.write(fd, b"\x1b]11;rgb:1212/1414/1818\x1b\\")
    if OSC_FG_QUERY.search(chunk):
        os.write(fd, b"\x1b]10;rgb:e6e6/e6e6/e6e6\x1b\\")
    if CSI_CURSOR_QUERY.search(chunk):
        os.write(fd, b"\x1b[1;1R")
    if DA_QUERY.search(chunk):
        os.write(fd, b"\x1b[?1;0c")


def drain(fd, events, t0, seconds):
    end = time.time() + seconds
    while time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.02)
        if not r:
            continue
        try:
            chunk = os.read(fd, 65536)
        except OSError:
            return
        if not chunk:
            return
        ts = time.time() - t0
        events.append((ts, chunk))
        respond(fd, chunk)


def main():
    out_path = sys.argv[1]
    binary = sys.argv[2]
    root = sys.argv[3]

    pid, fd = pty.fork()
    if pid == 0:
        os.environ["TERM"] = "xterm-256color"
        os.environ["COLUMNS"] = str(COLS)
        os.environ["LINES"] = str(ROWS)
        os.environ.pop("ANTHROPIC_API_KEY", None)
        os.execvp(binary, [binary, root])

    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", ROWS, COLS, 0, 0))

    events = []
    t0 = time.time()

    def stage(data, wait):
        if data:
            os.write(fd, data)
        drain(fd, events, t0, wait)

    # 1. Welcome card. Hold long enough for the viewer to read the rows.
    stage(None, 2.6)

    # 2. ? opens the full keymap overlay (only routed when no file is open).
    stage(b"?", 2.4)

    # 3. Dismiss the help overlay.
    stage(b"\x1b", 0.8)  # Esc

    # 4. Ctrl+P to open the picker, filter, Enter.
    stage(b"\x10", 0.5)
    stage(b"hello", 0.6)
    stage(b"\r", 1.0)

    # 5. Move to end of file, add a line, save.
    stage(b"\x05", 0.4)        # Ctrl+E (end of line)
    stage(b"\n\tfmt.Println(\"saved by nook\")", 0.8)
    stage(b"\x13", 1.0)        # Ctrl+S

    # 6. Quit.
    stage(b"\x11", 0.5)

    try:
        os.kill(pid, 9)
    except ProcessLookupError:
        pass
    os.close(fd)

    header = {
        "version": 2,
        "width": COLS,
        "height": ROWS,
        "timestamp": int(t0),
        "env": {"SHELL": "/bin/bash", "TERM": "xterm-256color"},
        "title": "nook — first-run welcome → keymap → open → save",
    }

    with open(out_path, "w") as f:
        f.write(json.dumps(header) + "\n")
        for ts, chunk in events:
            line = [round(ts, 4), "o", chunk.decode("utf-8", errors="replace")]
            f.write(json.dumps(line, ensure_ascii=False) + "\n")

    total_bytes = sum(len(c) for _, c in events)
    print(f"wrote {out_path}: {len(events)} events, {total_bytes} bytes, "
          f"{events[-1][0]:.2f}s duration")


if __name__ == "__main__":
    main()
