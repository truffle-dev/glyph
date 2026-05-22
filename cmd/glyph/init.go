package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	module := fs.String("module", "", "Go module path (e.g., github.com/me/myapp)")
	componentsDir := fs.String("components", "internal/ui", "Components target directory")
	libDir := fs.String("lib", "internal/uilib", "Lib target directory")
	frame := fs.String("frame", "bubbletea", "TUI framework: bubbletea (v0.1 only)")
	theme := fs.String("theme", "default", "Theme name")
	yes := fs.Bool("y", false, "Skip interactive prompts; use flag defaults")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "glyph init: %v\n", err)
		return 1
	}
	target := filepath.Join(wd, "glyph.json")
	if _, err := os.Stat(target); err == nil {
		fmt.Fprintf(os.Stderr, "glyph init: %s already exists. Refusing to overwrite.\n", target)
		return 1
	}

	c := defaultConfig()
	c.Frame = *frame
	c.Theme = *theme
	c.Aliases["components"] = *componentsDir
	c.Aliases["lib"] = *libDir

	if *module != "" {
		c.Module = *module
	} else {
		c.Module = guessModulePath(wd)
	}

	if !*yes {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Initializing glyph in", wd)
		c.Module = ask(reader, "Go module path", c.Module)
		c.Frame = ask(reader, "TUI framework", c.Frame)
		c.Aliases["components"] = ask(reader, "Components directory", c.Aliases["components"])
		c.Aliases["lib"] = ask(reader, "Lib directory", c.Aliases["lib"])
		c.Theme = ask(reader, "Theme", c.Theme)
		fmt.Println()
	}

	if err := writeConfig(c, target); err != nil {
		fmt.Fprintf(os.Stderr, "glyph init: %v\n", err)
		return 1
	}
	for _, k := range []string{"components", "lib"} {
		dir := filepath.Join(wd, c.Aliases[k])
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "glyph init: creating %s: %v\n", dir, err)
			return 1
		}
	}

	fmt.Println("✓ Wrote", target)
	fmt.Println("✓ Created", c.Aliases["components"]+"/")
	fmt.Println("✓ Created", c.Aliases["lib"]+"/")
	fmt.Println()
	fmt.Println("Next:")
	fmt.Println("  glyph add theme")
	fmt.Println("  glyph add chat-bubble")
	return 0
}

func ask(r *bufio.Reader, prompt, def string) string {
	if def != "" {
		fmt.Printf("? %s [%s] ", prompt, def)
	} else {
		fmt.Printf("? %s ", prompt)
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

// guessModulePath reads go.mod in cwd if present, otherwise returns "".
func guessModulePath(wd string) string {
	b, err := os.ReadFile(filepath.Join(wd, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}
	return ""
}
