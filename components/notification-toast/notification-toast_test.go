package notificationtoast

import (
	"strings"
	"testing"
	"time"

	"github.com/truffle-dev/glyph/components/theme"
)

func TestEmptyTrayRendersBlank(t *testing.T) {
	tr := New(theme.Default)
	if out := tr.View(); out != "" {
		t.Fatalf("empty tray should render empty, got %q", out)
	}
}

func TestPushRendersToast(t *testing.T) {
	tr := New(theme.Default).WithWidth(40)
	tr = tr.Push(Toast{ID: "1", Level: LevelInfo, Title: "Heads up", Message: "Something happened."})
	out := tr.View()
	if !strings.Contains(out, "Heads up") {
		t.Fatalf("title must render, got %q", out)
	}
	if !strings.Contains(out, "Something happened.") {
		t.Fatalf("message must render, got %q", out)
	}
}

func TestPushKicksOldestOverCapacity(t *testing.T) {
	tr := New(theme.Default).WithMaxItems(2).WithWidth(40)
	tr = tr.Push(Toast{ID: "a", Title: "first"})
	tr = tr.Push(Toast{ID: "b", Title: "second"})
	tr = tr.Push(Toast{ID: "c", Title: "third"})
	toasts := tr.Toasts()
	if len(toasts) != 2 {
		t.Fatalf("expected 2 toasts after capacity overflow, got %d", len(toasts))
	}
	if toasts[0].ID != "b" || toasts[1].ID != "c" {
		t.Fatalf("expected b,c, got %s,%s", toasts[0].ID, toasts[1].ID)
	}
}

func TestDismissRemovesByID(t *testing.T) {
	tr := New(theme.Default).WithWidth(40)
	tr = tr.Push(Toast{ID: "a", Title: "first"})
	tr = tr.Push(Toast{ID: "b", Title: "second"})
	tr = tr.Dismiss("a")
	toasts := tr.Toasts()
	if len(toasts) != 1 || toasts[0].ID != "b" {
		t.Fatalf("Dismiss('a') should leave only b, got %#v", toasts)
	}
}

func TestDismissMissingIDIsNoOp(t *testing.T) {
	tr := New(theme.Default).WithWidth(40)
	tr = tr.Push(Toast{ID: "a", Title: "first"})
	before := len(tr.Toasts())
	tr = tr.Dismiss("nonexistent")
	after := len(tr.Toasts())
	if before != after {
		t.Fatalf("Dismiss of missing ID changed length: %d → %d", before, after)
	}
}

func TestTickRemovesExpired(t *testing.T) {
	now := time.Unix(1_000, 0)
	tr := New(theme.Default).WithWidth(40)
	tr = tr.Push(Toast{ID: "old", Title: "expired", ExpiresAt: now.Add(-1 * time.Second)})
	tr = tr.Push(Toast{ID: "live", Title: "still here", ExpiresAt: now.Add(10 * time.Second)})
	tr = tr.Push(Toast{ID: "forever", Title: "no expiry"})
	tr = tr.Tick(now)
	toasts := tr.Toasts()
	if len(toasts) != 2 {
		t.Fatalf("Tick should remove only expired, got %d", len(toasts))
	}
	gotIDs := map[string]bool{}
	for _, t := range toasts {
		gotIDs[t.ID] = true
	}
	if !gotIDs["live"] || !gotIDs["forever"] {
		t.Fatalf("Tick removed the wrong toasts: %#v", toasts)
	}
}

func TestTickExactlyAtExpirationDismisses(t *testing.T) {
	now := time.Unix(2_000, 0)
	tr := New(theme.Default).Push(Toast{ID: "boundary", Title: "x", ExpiresAt: now})
	tr = tr.Tick(now)
	if len(tr.Toasts()) != 0 {
		t.Fatal("Tick at the exact expiration moment must dismiss the toast")
	}
}

func TestDismissAllEmpties(t *testing.T) {
	tr := New(theme.Default).WithWidth(40)
	tr = tr.Push(Toast{ID: "a"})
	tr = tr.Push(Toast{ID: "b"})
	tr = tr.DismissAll()
	if len(tr.Toasts()) != 0 {
		t.Fatal("DismissAll should empty the tray")
	}
}

func TestLevelIconsRender(t *testing.T) {
	tr := New(theme.Default).WithWidth(50)
	tr = tr.Push(Toast{ID: "i", Level: LevelInfo, Title: "info"})
	tr = tr.Push(Toast{ID: "s", Level: LevelSuccess, Title: "ok"})
	tr = tr.Push(Toast{ID: "w", Level: LevelWarning, Title: "warn"})
	tr = tr.Push(Toast{ID: "e", Level: LevelError, Title: "fail"})
	out := tr.View()
	for _, glyph := range []string{"i", "✓", "!", "✗"} {
		if !strings.Contains(out, glyph) {
			t.Fatalf("missing level glyph %q in output %q", glyph, out)
		}
	}
}

func TestStackedToastsAreSeparated(t *testing.T) {
	tr := New(theme.Default).WithWidth(40)
	tr = tr.Push(Toast{ID: "a", Title: "first"})
	tr = tr.Push(Toast{ID: "b", Title: "second"})
	out := tr.View()
	if !strings.Contains(out, "first") || !strings.Contains(out, "second") {
		t.Fatalf("both toasts must render, got %q", out)
	}
}
