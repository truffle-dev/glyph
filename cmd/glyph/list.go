package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

func runList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "Output as JSON")
	registry := fs.String("registry", "", "Override registry URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	url := *registry
	if url == "" {
		if c, _, err := loadConfig(); err == nil {
			url = c.registryURL()
		} else {
			url = defaultRegistryURL
		}
	}

	cat, err := fetchCatalog(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "glyph list:", err)
		return 1
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(cat)
		return 0
	}

	sort.SliceStable(cat.Items, func(i, j int) bool { return cat.Items[i].Name < cat.Items[j].Name })

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tCATEGORIES\tFRAME")
	for _, it := range cat.Items {
		cats := strings.Join(it.Categories, ", ")
		if cats == "" {
			cats = "-"
		}
		frame := it.Frame
		if frame == "" {
			frame = "bubbletea"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", it.Name, it.Type, cats, frame)
	}
	_ = w.Flush()
	return 0
}
