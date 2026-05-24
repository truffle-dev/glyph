package theme

import "sort"

// builtin is the canonical name → Theme registry. Apps that let users pick a
// theme by name (nook's settings file, story-mode previews) look up here so
// that adding a new theme is a one-line registration, not a switch statement
// hunt across the codebase.
var builtin = map[string]Theme{
	"default":          Default,
	"light":            Light,
	"tokyo-night":      TokyoNight,
	"catppuccin-mocha": CatppuccinMocha,
	"rose-pine":        RosePine,
}

// ByName returns the named theme and true when registered, or the zero Theme
// and false otherwise. Names are stable lowercase-with-hyphens identifiers.
// Callers that want a fallback should pair this with `Default`:
//
//	t, ok := theme.ByName(cfg.Theme)
//	if !ok {
//		t = theme.Default
//	}
func ByName(name string) (Theme, bool) {
	t, ok := builtin[name]
	return t, ok
}

// Names returns the registered theme identifiers in sorted order. Useful for
// surfacing the catalog in a settings UI or `--list-themes` flag without
// reaching into the registry directly.
func Names() []string {
	out := make([]string, 0, len(builtin))
	for n := range builtin {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
