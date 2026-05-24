package tasks

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPathMatchesRoot(t *testing.T) {
	got := Path("/tmp/proj")
	want := filepath.Join("/tmp/proj", ".nook", "tasks.toml")
	if got != want {
		t.Fatalf("Path: got %q, want %q", got, want)
	}
}

func TestLoadReturnsErrNotFoundWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(Path(dir))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLoadParsesTaskArrayOfTables(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".nook"))
	body := `
[[task]]
name = "test"
command = "go"
args = ["test", "./..."]
description = "Run tests"

[[task]]
name = "build"
command = "go"
args = ["build", "./..."]
cwd = "subpkg"
[task.env]
CGO_ENABLED = "0"
`
	mustWrite(t, filepath.Join(dir, ".nook", "tasks.toml"), body)
	cfg, err := Load(Path(dir))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tasks := cfg.All()
	if len(tasks) != 2 {
		t.Fatalf("len(tasks)=%d, want 2", len(tasks))
	}
	if tasks[0].Name != "test" || tasks[0].Command != "go" || len(tasks[0].Args) != 2 {
		t.Fatalf("task[0]=%+v", tasks[0])
	}
	if tasks[1].Cwd != "subpkg" {
		t.Fatalf("task[1].Cwd=%q, want %q", tasks[1].Cwd, "subpkg")
	}
	if tasks[1].Env["CGO_ENABLED"] != "0" {
		t.Fatalf("task[1].Env[CGO_ENABLED]=%q, want %q", tasks[1].Env["CGO_ENABLED"], "0")
	}
}

func TestLoadAcceptsTasksArrayAlias(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".nook"))
	body := `
tasks = [
  { name = "fmt",  command = "gofmt", args = ["-w", "."] },
  { name = "vet",  command = "go",    args = ["vet", "./..."] },
]
`
	mustWrite(t, filepath.Join(dir, ".nook", "tasks.toml"), body)
	cfg, err := Load(Path(dir))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tasks := cfg.All()
	if len(tasks) != 2 {
		t.Fatalf("len(tasks)=%d, want 2", len(tasks))
	}
	if tasks[0].Name != "fmt" || tasks[1].Name != "vet" {
		t.Fatalf("unexpected: %+v", tasks)
	}
}

func TestLoadMalformedReturnsError(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".nook"))
	mustWrite(t, filepath.Join(dir, ".nook", "tasks.toml"), `[[task\nname = oops`)
	_, err := Load(Path(dir))
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatal("malformed should not be ErrNotFound")
	}
}

func TestDefaultsForGoProject(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.21\n")
	defaults := Defaults(dir)
	if len(defaults) != 4 {
		t.Fatalf("len(defaults)=%d, want 4", len(defaults))
	}
	names := []string{}
	for _, d := range defaults {
		names = append(names, d.Name)
	}
	want := []string{"go test", "go build", "go vet", "go mod tidy"}
	for i, n := range want {
		if names[i] != n {
			t.Fatalf("defaults[%d].Name=%q, want %q", i, names[i], n)
		}
	}
}

func TestDefaultsForNonGoProjectReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	defaults := Defaults(dir)
	if len(defaults) != 0 {
		t.Fatalf("len(defaults)=%d, want 0", len(defaults))
	}
}

func TestLoadOrDefaultsPrefersConfigWhenPresent(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module x\n")
	mustMkdir(t, filepath.Join(dir, ".nook"))
	mustWrite(t, filepath.Join(dir, ".nook", "tasks.toml"), `
[[task]]
name = "custom"
command = "echo"
args = ["hello"]
`)
	tasks, err := LoadOrDefaults(dir)
	if err != nil {
		t.Fatalf("LoadOrDefaults: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Name != "custom" {
		t.Fatalf("got %+v, want one task named custom", tasks)
	}
}

func TestLoadOrDefaultsFallsBackToGoDefaults(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module x\n")
	tasks, err := LoadOrDefaults(dir)
	if err != nil {
		t.Fatalf("LoadOrDefaults: %v", err)
	}
	if len(tasks) != 4 {
		t.Fatalf("got %d tasks, want 4 go defaults", len(tasks))
	}
}

func TestLoadOrDefaultsMalformedSurfacesErrorButStillReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module x\n")
	mustMkdir(t, filepath.Join(dir, ".nook"))
	mustWrite(t, filepath.Join(dir, ".nook", "tasks.toml"), "this isnt toml = at all\nbroken{")
	tasks, err := LoadOrDefaults(dir)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if len(tasks) != 4 {
		t.Fatalf("got %d tasks alongside the error, want 4 (go defaults)", len(tasks))
	}
}

func TestLoadOrDefaultsEmptyConfigFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module x\n")
	mustMkdir(t, filepath.Join(dir, ".nook"))
	mustWrite(t, filepath.Join(dir, ".nook", "tasks.toml"), "# empty\n")
	tasks, err := LoadOrDefaults(dir)
	if err != nil {
		t.Fatalf("LoadOrDefaults: %v", err)
	}
	if len(tasks) != 4 {
		t.Fatalf("empty config should fall through to defaults; got %d", len(tasks))
	}
}

func TestTaskIsValid(t *testing.T) {
	cases := []struct {
		t    Task
		want bool
	}{
		{Task{Name: "x", Command: "echo"}, true},
		{Task{Name: "", Command: "echo"}, false},
		{Task{Name: "x", Command: ""}, false},
		{Task{}, false},
	}
	for i, c := range cases {
		if got := c.t.IsValid(); got != c.want {
			t.Errorf("case %d: IsValid=%v, want %v (task=%+v)", i, got, c.want, c.t)
		}
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
