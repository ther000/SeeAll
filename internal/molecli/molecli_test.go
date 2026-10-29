package molecli

import (
	"context"
	"errors"
	"testing"
)

// TestNotInstalled pins the "mo is not installed" path: it only looks, and all
// six entry points return an ErrNotInstalled that errors.Is can spot, rather
// than each inventing its own error.
func TestNotInstalled(t *testing.T) {
	c := New()
	c.look = func() (string, bool) { return "", false }

	if got := c.Path(); got != "" {
		t.Errorf("Path gave %q with mo absent, want the empty string", got)
	}
	if c.Available() {
		t.Error("Available said true with mo absent")
	}

	ctx := context.Background()
	entries := []struct {
		name string
		call func() error
	}{
		{"PreviewClean", func() error { _, err := c.PreviewClean(ctx); return err }},
		{"Clean", func() error { _, err := c.Clean(ctx); return err }},
		{"PreviewOptimize", func() error { _, err := c.PreviewOptimize(ctx); return err }},
		{"Optimize", func() error { _, err := c.Optimize(ctx); return err }},
		{"Analyze", func() error { _, err := c.Analyze(ctx); return err }},
		{"History", func() error { _, err := c.History(ctx, 5); return err }},
	}
	for _, entry := range entries {
		if err := entry.call(); !errors.Is(err, ErrNotInstalled) {
			t.Errorf("%s returned %v, want ErrNotInstalled", entry.name, err)
		}
	}
}

// TestPathIsProbedOnce pins the cache: one lookup is remembered, and Forget
// makes it look again. The panel asks Available from more than one place, and
// walking PATH every time would be wasted work.
func TestPathIsProbedOnce(t *testing.T) {
	c := New()
	calls := 0
	c.look = func() (string, bool) {
		calls++
		return `C:\fake\mole-cli\mo.cmd`, true
	}

	if got := c.Path(); got != `C:\fake\mole-cli\mo.cmd` {
		t.Fatalf("Path gave %q", got)
	}
	if !c.Available() {
		t.Error("Available said false")
	}
	if calls != 1 {
		t.Errorf("looked %d times, want 1 (the result should be remembered)", calls)
	}

	c.Forget()
	if !c.Available() {
		t.Error("Available said false after Forget")
	}
	if calls != 2 {
		t.Errorf("looked %d times after Forget, want 2", calls)
	}
}

// TestForgetAfterUninstall pins what Forget is for: once mo is uninstalled,
// one Forget turns the state back into "not installed" without a restart.
func TestForgetAfterUninstall(t *testing.T) {
	installed := true
	c := New()
	c.look = func() (string, bool) {
		if !installed {
			return "", false
		}
		return `C:\fake\mo.cmd`, true
	}
	if !c.Available() {
		t.Fatal("says not installed right after being installed")
	}
	installed = false
	if !c.Available() {
		t.Fatal("the cached answer expired early; that is what Forget is for")
	}
	c.Forget()
	if c.Available() {
		t.Error("still says available after mo was uninstalled and forgotten")
	}
	if _, err := c.Analyze(context.Background()); !errors.Is(err, ErrNotInstalled) {
		t.Errorf("after the uninstall the error is %v, want ErrNotInstalled", err)
	}
}
