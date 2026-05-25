package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// version is the human-readable CLI version string. The source-only value
// always carries a "-dev" suffix; release binaries are built with goreleaser,
// which overrides this with the tag via -ldflags "-X main.version=...".
var version = "0.28.1-dev"

// versionString returns the human-readable CLI version. When the binary was
// built with VCS info embedded (typical for `go install` and `go build`), it
// includes a short commit SHA, the commit time, and a "-dirty" marker if the
// working tree had uncommitted changes. The Go runtime version is always
// included so bug reports name the toolchain.
func versionString() string {
	var b strings.Builder
	fmt.Fprintf(&b, "glyph %s", version)
	if bi, ok := debug.ReadBuildInfo(); ok {
		var rev, when string
		var dirty bool
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.time":
				when = s.Value
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
		if rev != "" {
			if len(rev) > 12 {
				rev = rev[:12]
			}
			if dirty {
				rev += "-dirty"
			}
			fmt.Fprintf(&b, " (commit %s", rev)
			if when != "" {
				fmt.Fprintf(&b, ", %s", when)
			}
			b.WriteString(")")
		}
	}
	fmt.Fprintf(&b, " %s", runtime.Version())
	return b.String()
}
