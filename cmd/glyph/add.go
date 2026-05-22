package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func runAdd(args []string) int {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	yes := fs.Bool("y", false, "Skip the confirmation prompt")
	force := fs.Bool("f", false, "Overwrite existing files")
	registry := fs.String("registry", "", "Override registry URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "glyph add: requires a component name")
		return 2
	}

	c, cfgPath, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "glyph add:", err)
		return 1
	}
	projectRoot := filepath.Dir(cfgPath)
	regURL := c.registryURL()
	if *registry != "" {
		regURL = strings.TrimRight(*registry, "/")
	}

	// Resolve every requested component and recursively pull registryDependencies.
	resolved := map[string]*RegistryItem{}
	order := []string{}
	for _, name := range fs.Args() {
		if err := resolveItem(regURL, name, resolved, &order, map[string]bool{}); err != nil {
			fmt.Fprintln(os.Stderr, "glyph add:", err)
			return 1
		}
	}

	// Build install plan.
	planFiles := [][3]string{} // {item, target, fileType}
	planDeps := map[string]bool{}
	for _, name := range order {
		item := resolved[name]
		for _, dep := range item.Dependencies {
			planDeps[dep] = true
		}
		for _, f := range item.Files {
			target, err := c.resolveAlias(f.Target)
			if err != nil {
				fmt.Fprintln(os.Stderr, "glyph add:", err)
				return 1
			}
			planFiles = append(planFiles, [3]string{name, target, f.Type})
		}
	}

	// Print plan.
	fmt.Println("Plan:")
	for _, p := range planFiles {
		fmt.Printf("  + %-18s %s\n", p[0], p[1])
	}
	if len(planDeps) > 0 {
		fmt.Println()
		fmt.Println("Dependencies:")
		depList := make([]string, 0, len(planDeps))
		for d := range planDeps {
			depList = append(depList, d)
		}
		sort.Strings(depList)
		for _, d := range depList {
			fmt.Printf("  + %s\n", d)
		}
	}
	fmt.Println()

	if !*yes {
		fmt.Print("Proceed? [y/N] ")
		r := bufio.NewReader(os.Stdin)
		line, _ := r.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "y" && line != "yes" {
			fmt.Println("Aborted.")
			return 1
		}
	}

	// Refuse to overwrite unless -f.
	if !*force {
		for _, p := range planFiles {
			fullPath := filepath.Join(projectRoot, p[1])
			if _, err := os.Stat(fullPath); err == nil {
				fmt.Fprintf(os.Stderr, "glyph add: refusing to overwrite %s (use -f to force)\n", fullPath)
				return 1
			}
		}
	}

	// Install dependencies.
	if len(planDeps) > 0 {
		fmt.Println("Installing dependencies...")
		depList := make([]string, 0, len(planDeps))
		for d := range planDeps {
			depList = append(depList, d)
		}
		sort.Strings(depList)
		for _, d := range depList {
			if err := runGoGet(projectRoot, d); err != nil {
				fmt.Fprintf(os.Stderr, "glyph add: go get %s: %v\n", d, err)
				return 1
			}
		}
		fmt.Println("✓ Dependencies installed")
	}

	// Fetch and write files.
	fmt.Println("Writing files...")
	for _, name := range order {
		item := resolved[name]
		for _, f := range item.Files {
			target, _ := c.resolveAlias(f.Target)
			fullPath := filepath.Join(projectRoot, target)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				fmt.Fprintln(os.Stderr, "glyph add:", err)
				return 1
			}
			fileURL := f.URL
			if fileURL == "" {
				fileURL = regURL + "/" + item.Name + "/" + filepath.Base(f.Path)
			}
			body, err := fetchText(fileURL)
			if err != nil {
				fmt.Fprintln(os.Stderr, "glyph add:", err)
				return 1
			}
			body = rewriteImports(body, c)
			body = applyStyles(body, item.Styles)
			if err := os.WriteFile(fullPath, body, 0o644); err != nil {
				fmt.Fprintln(os.Stderr, "glyph add:", err)
				return 1
			}
		}
	}
	fmt.Println("✓ Files written")

	// Tidy modules so the consumer's go.sum and minimum-version selection are stable.
	if len(planDeps) > 0 {
		if err := runGoModTidy(projectRoot); err != nil {
			fmt.Fprintf(os.Stderr, "glyph add: go mod tidy: %v\n", err)
			return 1
		}
		fmt.Println("✓ Modules tidied")
	}
	fmt.Println()

	// Print docs from the last item (typically the user-requested one).
	last := resolved[order[len(order)-1]]
	if last.Docs != "" {
		fmt.Println(last.Docs)
	} else {
		fmt.Printf("%s installed.\n", last.Name)
	}
	return 0
}

func resolveItem(registryURL, name string, resolved map[string]*RegistryItem, order *[]string, inProgress map[string]bool) error {
	if _, ok := resolved[name]; ok {
		return nil
	}
	if inProgress[name] {
		return fmt.Errorf("dependency cycle through %s", name)
	}
	inProgress[name] = true
	item, err := fetchItem(registryURL, name)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", name, err)
	}
	for _, dep := range item.RegistryDependencies {
		if err := resolveItem(registryURL, dep, resolved, order, inProgress); err != nil {
			return err
		}
	}
	resolved[name] = item
	*order = append(*order, name)
	delete(inProgress, name)
	return nil
}

func runGoGet(dir, mod string) error {
	cmd := exec.Command("go", "get", mod)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runGoModTidy(dir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
