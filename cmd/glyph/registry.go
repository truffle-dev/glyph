package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RegistryFile is a single source file in a component.
type RegistryFile struct {
	Path   string `json:"path"`
	URL    string `json:"url,omitempty"`
	Type   string `json:"type"`
	Target string `json:"target"`
}

// RegistryItem mirrors registry-item.json. See research/04-architecture-spec.md.
type RegistryItem struct {
	Schema               string            `json:"$schema"`
	Name                 string            `json:"name"`
	Type                 string            `json:"type"`
	Title                string            `json:"title"`
	Description          string            `json:"description"`
	Author               string            `json:"author"`
	Version              string            `json:"version"`
	Frame                string            `json:"frame"`
	Dependencies         []string          `json:"dependencies,omitempty"`
	DevDependencies      []string          `json:"devDependencies,omitempty"`
	RegistryDependencies []string          `json:"registryDependencies,omitempty"`
	Files                []RegistryFile    `json:"files"`
	Styles               map[string]string `json:"styles,omitempty"`
	Docs                 string            `json:"docs,omitempty"`
	Categories           []string          `json:"categories,omitempty"`
	Meta                 map[string]any    `json:"meta,omitempty"`
}

// RegistryCatalog mirrors registry.json (the root manifest).
type RegistryCatalog struct {
	Schema   string         `json:"$schema"`
	Name     string         `json:"name"`
	Homepage string         `json:"homepage"`
	Items    []RegistryItem `json:"items"`
}

var httpClient = &http.Client{Timeout: 20 * time.Second}

func fetchJSON(target string, out any) error {
	if _, err := url.Parse(target); err != nil {
		return fmt.Errorf("invalid URL %q: %w", target, err)
	}
	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "glyph/"+version)
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return fmt.Errorf("fetching %s: HTTP %d %s", target, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func fetchText(target string) ([]byte, error) {
	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "glyph/"+version)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: HTTP %d", target, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func fetchCatalog(registryURL string) (*RegistryCatalog, error) {
	var cat RegistryCatalog
	if err := fetchJSON(registryURL+"/registry.json", &cat); err != nil {
		return nil, err
	}
	return &cat, nil
}

func fetchItem(registryURL, name string) (*RegistryItem, error) {
	var item RegistryItem
	if err := fetchJSON(registryURL+"/"+name+".json", &item); err != nil {
		return nil, err
	}
	return &item, nil
}
