// Build flattens components/*/*.json manifests into a registry tree under ./r.
//
// Output:
//
//	r/registry.json       — catalog of all items (no source URLs embedded)
//	r/<name>.json         — full item manifest with absolute file URLs
//	r/<name>/<file>       — copy of every file referenced by the item
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type registryFile struct {
	Path   string `json:"path"`
	URL    string `json:"url,omitempty"`
	Type   string `json:"type"`
	Target string `json:"target"`
}

type registryItem struct {
	Schema               string            `json:"$schema"`
	Name                 string            `json:"name"`
	Type                 string            `json:"type"`
	Title                string            `json:"title"`
	Description          string            `json:"description"`
	Author               string            `json:"author,omitempty"`
	Version              string            `json:"version"`
	Frame                string            `json:"frame"`
	Dependencies         []string          `json:"dependencies,omitempty"`
	DevDependencies      []string          `json:"devDependencies,omitempty"`
	RegistryDependencies []string          `json:"registryDependencies,omitempty"`
	Files                []registryFile    `json:"files,omitempty"`
	Styles               map[string]string `json:"styles,omitempty"`
	Docs                 string            `json:"docs,omitempty"`
	Categories           []string          `json:"categories,omitempty"`
	Meta                 map[string]any    `json:"meta,omitempty"`
}

type catalogEntry struct {
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Frame       string         `json:"frame"`
	Categories  []string       `json:"categories,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

type catalog struct {
	Schema   string         `json:"$schema"`
	Name     string         `json:"name"`
	Homepage string         `json:"homepage"`
	Items    []catalogEntry `json:"items"`
}

func main() {
	srcDir := flag.String("src", "components", "source directory containing components/<name>/<name>.json")
	outDir := flag.String("out", "r", "output directory (will be created fresh)")
	baseURL := flag.String("base-url", "https://truffleagent.com/glyph/r", "absolute base URL embedded into file URLs")
	flag.Parse()

	if err := run(*srcDir, *outDir, strings.TrimRight(*baseURL, "/")); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
}

func run(srcDir, outDir, baseURL string) error {
	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	manifests, err := findManifests(srcDir)
	if err != nil {
		return err
	}
	if len(manifests) == 0 {
		return fmt.Errorf("no manifests found under %s", srcDir)
	}

	cat := catalog{
		Schema:   "https://truffleagent.com/glyph/schema/registry.json",
		Name:     "glyph",
		Homepage: "https://truffleagent.com/glyph",
	}

	for _, m := range manifests {
		item, err := loadManifest(m)
		if err != nil {
			return fmt.Errorf("loading %s: %w", m, err)
		}
		if err := validate(item); err != nil {
			return fmt.Errorf("validating %s: %w", item.Name, err)
		}
		if err := emitItem(item, srcDir, outDir, baseURL); err != nil {
			return fmt.Errorf("emitting %s: %w", item.Name, err)
		}
		cat.Items = append(cat.Items, catalogEntry{
			Name:        item.Name,
			Type:        item.Type,
			Title:       item.Title,
			Description: item.Description,
			Version:     item.Version,
			Frame:       item.Frame,
			Categories:  item.Categories,
			Meta:        item.Meta,
		})
	}

	sort.SliceStable(cat.Items, func(i, j int) bool { return cat.Items[i].Name < cat.Items[j].Name })
	catPath := filepath.Join(outDir, "registry.json")
	if err := writeJSON(catPath, cat); err != nil {
		return err
	}

	fmt.Printf("built %d items into %s\n", len(cat.Items), outDir)
	for _, it := range cat.Items {
		fmt.Printf("  %s (%s)\n", it.Name, it.Type)
	}
	return nil
}

func findManifests(srcDir string) ([]string, error) {
	var out []string
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifest := filepath.Join(srcDir, e.Name(), e.Name()+".json")
		if _, err := os.Stat(manifest); err == nil {
			out = append(out, manifest)
		}
	}
	sort.Strings(out)
	return out, nil
}

func loadManifest(path string) (*registryItem, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var item registryItem
	if err := json.Unmarshal(b, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func validate(it *registryItem) error {
	if it.Name == "" {
		return fmt.Errorf("missing name")
	}
	if it.Type == "" {
		return fmt.Errorf("missing type")
	}
	if it.Version == "" {
		return fmt.Errorf("missing version")
	}
	if len(it.Files) == 0 {
		return fmt.Errorf("no files declared")
	}
	for _, f := range it.Files {
		if f.Path == "" {
			return fmt.Errorf("file missing path")
		}
		if f.Target == "" {
			return fmt.Errorf("file %s missing target", f.Path)
		}
		if f.Type == "" {
			return fmt.Errorf("file %s missing type", f.Path)
		}
	}
	return nil
}

func emitItem(it *registryItem, srcDir, outDir, baseURL string) error {
	itemDir := filepath.Join(outDir, it.Name)
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		return err
	}
	// Copy each file and set URL on a copy of the item used for the manifest.
	emitted := *it
	emitted.Files = make([]registryFile, len(it.Files))
	for i, f := range it.Files {
		src := filepath.Join(".", f.Path) // f.Path is relative to repo root
		dst := filepath.Join(itemDir, filepath.Base(f.Path))
		if _, err := os.Stat(src); err != nil {
			// Try inside srcDir as fallback.
			alt := filepath.Join(srcDir, it.Name, filepath.Base(f.Path))
			if _, err := os.Stat(alt); err != nil {
				return fmt.Errorf("source not found: %s (also tried %s)", src, alt)
			}
			src = alt
		}
		if err := copyFile(src, dst); err != nil {
			return err
		}
		emitted.Files[i] = registryFile{
			Path:   f.Path,
			URL:    baseURL + "/" + it.Name + "/" + filepath.Base(f.Path),
			Type:   f.Type,
			Target: f.Target,
		}
	}
	manifestPath := filepath.Join(outDir, it.Name+".json")
	return writeJSON(manifestPath, emitted)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func writeJSON(path string, v any) error {
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(buf.String()), 0o644)
}
