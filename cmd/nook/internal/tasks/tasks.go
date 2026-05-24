// Package tasks owns nook's project-task surface: the TOML loader at
// `.nook/tasks.toml`, the default task set inferred from a Go project's
// go.mod, the process supervisor that streams a running task's stdout
// and stderr into Bubble Tea messages, and the overlay pane that lets a
// user pick a task, watch its output, and kill it.
//
// The shape is small on purpose. A `Task` is a name + a `command` and
// `args` slice. No shell quoting, no env-substitution; if a user needs
// a shell they spell it out (`command = "sh", args = ["-c", "..."]`).
// The pane has two modes — list and output — and the host wires `alt+t`
// to open the overlay. Esc closes (and kills any running process).
//
// Defaults: when no `.nook/tasks.toml` exists, a Go project gets four
// tasks (`go test`, `go build`, `go vet`, `go mod tidy`). Other
// languages currently fall back to no defaults; adding `package.json`
// detection (npm test / npm run build) is a planned follow-up.
package tasks

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Task describes one runnable command. Cwd is optional; when blank the
// workspace root is used. Env merges over the parent process env.
type Task struct {
	// Name is the human-readable label shown in the picker. Must be
	// non-empty for the task to be selectable.
	Name string `toml:"name"`
	// Description is an optional one-line hint shown beneath the name
	// in the picker.
	Description string `toml:"description"`
	// Command is the executable name (resolved via $PATH) or absolute
	// path. Required.
	Command string `toml:"command"`
	// Args are the arguments passed to Command. Optional.
	Args []string `toml:"args"`
	// Cwd is the working directory for the spawned process. When
	// blank the workspace root is used. Relative paths are resolved
	// against the workspace root.
	Cwd string `toml:"cwd"`
	// Env supplies process-environment overrides. Merged over the
	// parent process env; later writes win on duplicate keys.
	Env map[string]string `toml:"env"`
}

// Config is the deserialized `.nook/tasks.toml`. Tasks may also be
// listed inline as a TOML array of tables under `[[task]]`; the
// `tasks_alias` field is the equivalent `tasks = [...]` form.
type Config struct {
	Tasks      []Task `toml:"task"`
	TasksAlias []Task `toml:"tasks"`
}

// All returns the merged task list, preferring `[[task]]` entries over
// `tasks = [...]` aliases when both exist. Empty when neither set was
// populated.
func (c Config) All() []Task {
	if len(c.Tasks) > 0 && len(c.TasksAlias) > 0 {
		out := make([]Task, 0, len(c.Tasks)+len(c.TasksAlias))
		out = append(out, c.Tasks...)
		out = append(out, c.TasksAlias...)
		return out
	}
	if len(c.Tasks) > 0 {
		return c.Tasks
	}
	return c.TasksAlias
}

// ErrNotFound is returned by Load when the tasks file is absent. The
// host treats this as "no project config" and falls back to Defaults
// without surfacing an error.
var ErrNotFound = errors.New("tasks: file not found")

// Path returns the canonical tasks file location for a workspace root:
// `<root>/.nook/tasks.toml`. Mirrors how vscode keeps `tasks.json`
// under `.vscode/`.
func Path(root string) string {
	return filepath.Join(root, ".nook", "tasks.toml")
}

// Load reads and parses the tasks file at path. Returns ErrNotFound
// when the file doesn't exist (the host should call Defaults for that
// root); any other error means the file exists but is malformed.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, ErrNotFound
		}
		return Config{}, err
	}
	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Defaults returns the auto-generated task list for a project root.
// Currently Go-aware: when `<root>/go.mod` exists, returns the four
// standard go tooling tasks. Returns an empty slice when no language
// is recognized.
func Defaults(root string) []Task {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		return []Task{
			{
				Name:        "go test",
				Description: "Run all Go tests in this module",
				Command:     "go",
				Args:        []string{"test", "./..."},
			},
			{
				Name:        "go build",
				Description: "Build every package in this module",
				Command:     "go",
				Args:        []string{"build", "./..."},
			},
			{
				Name:        "go vet",
				Description: "Vet every package in this module",
				Command:     "go",
				Args:        []string{"vet", "./..."},
			},
			{
				Name:        "go mod tidy",
				Description: "Sync go.mod / go.sum with imports",
				Command:     "go",
				Args:        []string{"mod", "tidy"},
			},
		}
	}
	return nil
}

// LoadOrDefaults reads `<root>/.nook/tasks.toml` and returns its task
// list. When the file is absent, returns Defaults(root). When the file
// is malformed, returns the parse error plus Defaults(root) so the host
// can still offer something useful while surfacing the message.
func LoadOrDefaults(root string) ([]Task, error) {
	cfg, err := Load(Path(root))
	if errors.Is(err, ErrNotFound) {
		return Defaults(root), nil
	}
	if err != nil {
		return Defaults(root), err
	}
	tasks := cfg.All()
	if len(tasks) == 0 {
		return Defaults(root), nil
	}
	return tasks, nil
}

// IsValid reports whether the task is runnable: non-empty Name and
// Command. Tasks failing this check are filtered out of the picker.
func (t Task) IsValid() bool {
	return t.Name != "" && t.Command != ""
}
