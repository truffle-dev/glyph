"""Drive nook to show LSP red dot, output asciinema v2 cast."""
import fcntl, json, os, pty, re, select, struct, sys, tempfile, termios, time

COLS, ROWS = 120, 32
OSC_BG = re.compile(rb"\x1b\]11;\?\x1b\\")
OSC_FG = re.compile(rb"\x1b\]10;\?\x1b\\")
CSI_CUR = re.compile(rb"\x1b\[6n")
DA = re.compile(rb"\x1b\[c|\x1b\[>c")

def respond(fd, c):
    if OSC_BG.search(c): os.write(fd, b"\x1b]11;rgb:1212/1414/1818\x1b\\")
    if OSC_FG.search(c): os.write(fd, b"\x1b]10;rgb:e6e6/e6e6/e6e6\x1b\\")
    if CSI_CUR.search(c): os.write(fd, b"\x1b[1;1R")
    if DA.search(c): os.write(fd, b"\x1b[?1;0c")

out_path = sys.argv[1]

# Fixture: a Go file with one error
project = tempfile.mkdtemp(prefix="nook-lsp-cast-")
with open(os.path.join(project, "go.mod"), "w") as f:
    f.write("module lspdemo\n\ngo 1.23\n")
with open(os.path.join(project, "main.go"), "w") as f:
    f.write("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(messag)\n}\n")
# init git so git pane doesn't complain
import subprocess
subprocess.run(["git", "init", "-q", "-b", "main"], cwd=project)
subprocess.run(["git", "add", "."], cwd=project)
subprocess.run(["git", "-c", "user.email=nook@e.com", "-c", "user.name=nook",
                "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init"], cwd=project)

pid, fd = pty.fork()
if pid == 0:
    os.environ["TERM"] = "xterm-256color"
    os.environ["COLUMNS"] = str(COLS); os.environ["LINES"] = str(ROWS)
    os.environ.pop("ANTHROPIC_API_KEY", None)
    os.environ["NOOK_GHOST_DEMO"] = ""
    os.execvp("/tmp/nookbin", ["nookbin", project])
fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", ROWS, COLS, 0, 0))

events = []
t0 = time.time()

def drain(secs):
    end = time.time() + secs
    while time.time() < end:
        r,_,_ = select.select([fd],[],[],0.02)
        if not r: continue
        try: chunk = os.read(fd, 65536)
        except OSError: return
        if not chunk: return
        respond(fd, chunk)
        events.append((time.time() - t0, chunk))

def stage(data, wait):
    if data: os.write(fd, data)
    drain(wait)

stage(None, 1.6)          # boot
stage(b"\x10", 0.5)       # Ctrl+P
stage(b"main.go", 0.4)    # filter
stage(b"\r", 0.6)         # Enter — open file, kicks off gopls

# Wait for gopls (up to 12s) — drain until red ● appears
deadline = time.time() + 12.0
saw_dot = False
while time.time() < deadline:
    before = len(events)
    drain(0.5)
    new = b"".join(c for _, c in events[before:])
    if b"\xe2\x97\x8f" in new:
        saw_dot = True
        # hold for 1.2s so the viewer sees it
        drain(1.2)
        break

stage(b"\x11", 0.3)       # Ctrl+Q

try: os.kill(pid, 9)
except: pass
os.close(fd)

header = {
    "version": 2, "width": COLS, "height": ROWS,
    "timestamp": int(t0),
    "env": {"SHELL": "/bin/bash", "TERM": "xterm-256color"},
    "title": "nook — LSP diagnostics (gopls)",
}
with open(out_path, "w") as f:
    f.write(json.dumps(header) + "\n")
    for ts, chunk in events:
        f.write(json.dumps([round(ts, 4), "o", chunk.decode("utf-8", errors="replace")], ensure_ascii=False) + "\n")

print(f"events={len(events)} duration={events[-1][0]:.2f}s saw_dot={saw_dot}")
