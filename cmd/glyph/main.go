// Package main is the glyph CLI entry point.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "init":
		os.Exit(runInit(args))
	case "add":
		os.Exit(runAdd(args))
	case "list", "ls":
		os.Exit(runList(args))
	case "version", "-version", "--version", "-v":
		fmt.Println(versionString())
	case "help", "-help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "glyph: unknown command %q\n\n", cmd)
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Print(`glyph — copy-paste TUI components

Usage:
  glyph <command> [arguments]

Commands:
  init                Write glyph.json in the current project
  add <name>          Install a component (and its dependencies) into the project
  list                Show all components in the default registry
  version             Print the CLI version
  help                Show this help

Examples:
  glyph init
  glyph add chat-bubble
  glyph list

For more, see https://truffleagent.com/glyph
`)
}
