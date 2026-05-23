//go:build glyph_story || glyph_snap

package main

// engagement is a fake "scout queue" snapshot: repos the operator might
// hand off to an outreach agent, with light counts and a status flag.
type engagement struct {
	Repo, Owner, Issues, PRs, LastTouch, Status string
}

func sample() []engagement {
	return []engagement{
		{"glyph", "truffle-dev", "3", "1", "2026-05-23", "active"},
		{"vigil", "baudsmithstudios", "12", "4", "2026-05-22", "active"},
		{"voltagent", "VoltAgent", "27", "8", "2026-05-22", "merged"},
		{"clap", "clap-rs", "104", "16", "2026-05-19", "blocked"},
		{"jj", "jj-vcs", "412", "23", "2026-05-15", "merged"},
		{"auto-subs", "tmoroney", "8", "2", "2026-05-21", "merged"},
		{"opencli", "opencli-tools", "9", "2", "2026-05-19", "merged"},
		{"helix", "helix-editor", "319", "11", "2026-05-10", "off-limits"},
		{"sprocket", "stjude-rust-labs", "47", "5", "2026-05-12", "off-limits"},
		{"ratatui", "ratatui-org", "63", "9", "2026-05-22", "ready"},
		{"DuckDB", "duckdb", "892", "47", "2026-05-22", "active"},
		{"openclaw", "anthropics", "147", "12", "2026-05-22", "active"},
		{"NemoClaw", "nvidia", "284", "19", "2026-05-22", "active"},
		{"astro", "withastro", "0", "0", "2026-05-22", "off-limits"},
		{"turso", "tursodatabase", "0", "0", "2026-05-22", "off-limits"},
	}
}
