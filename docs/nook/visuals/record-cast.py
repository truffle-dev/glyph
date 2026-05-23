#!/usr/bin/env python3
"""Drive nook in a PTY and emit an asciinema v2 cast.

Run from the repo root once nook is built:

    go build -o cmd/nook/nookbin ./cmd/nook
    mkdir -p /tmp/nook-fixture && cat > /tmp/nook-fixture/fixture.go <<'EOF'
    package main

    import "fmt"

    func main() {
    }
    EOF
    (cd /tmp/nook-fixture && git init -q && git add . && git commit -q -m init)
    python3 docs/nook/visuals/record-cast.py docs/nook/visuals/tour.cast \\
        cmd/nook/nookbin /tmp/nook-fixture

The cast records the ghost-text wedge: open file, type "fmt.Pri", idle past the
debounce so the demo proposal lands, Tab to accept. NOOK_GHOST_DEMO is set so
no API key is consumed.

Render to GIF with agg (one-shot via docker):

    docker run --rm -v $(pwd):/work ghcr.io/asciinema/agg:latest \\
        --theme github-dark --speed 1.4 --font-size 14 \\
        /work/docs/nook/visuals/tour.cast /work/docs/nook/visuals/tour.gif
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
ROWS = 32

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
        os.environ["NOOK_GHOST_DEMO"] = "ntln(\"hello, world\")"
        os.execvp(binary, [binary, root])

    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", ROWS, COLS, 0, 0))

    events = []
    t0 = time.time()

    def stage(data, wait):
        if data:
            os.write(fd, data)
        drain(fd, events, t0, wait)

    stage(None, 1.6)
    stage(b"\x10", 0.6)        # Ctrl+P opens the picker
    stage(b"fixture", 0.5)     # filter
    stage(b"\r", 0.9)          # open file
    stage(b"\x05", 0.4)        # Ctrl+E to end of line 1
    stage(b"\nfmt.Pri", 0.5)   # type prefix
    stage(None, 1.1)           # idle past debounce so ghost lands
    stage(b"\t", 0.6)          # Tab accepts
    stage(None, 0.6)
    stage(b"\x11", 0.5)        # Ctrl+Q

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
        "title": "nook — ghost-text autocomplete (Tab to accept)",
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
