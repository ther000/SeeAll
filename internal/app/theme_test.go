package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestThemeIsRemembered checks the one thing a look that lives in the settings
// file can get wrong: the panel opens in the other theme, because the value
// was written and never read back — or read back as "not classic" on a file
// that says classic.
func TestThemeIsRemembered(t *testing.T) {
	dir := t.TempDir()

	h := NewHost(dir)
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	if got := h.Theme(); got != ThemeDashboard {
		t.Fatalf("a fresh install opens in %q, want %q", got, ThemeDashboard)
	}

	h.SetTheme(ThemeClassic)

	// A second run of the app, reading the file the first one wrote.
	again := NewHost(dir)
	if err := again.Load(); err != nil {
		t.Fatal(err)
	}
	again.Stop()
	if got := again.Theme(); got != ThemeClassic {
		t.Errorf("after a restart the panel drew %q, want %q", got, ThemeClassic)
	}
	// The look shares the file with the other choices, and must not take
	// them with it.
	if got := again.PanelOpacity(); got != 1 {
		t.Errorf("opacity = %v, want the untouched 1", got)
	}
	if len(again.Instances()) == 0 {
		t.Errorf("the monitors were lost: %v", again.Instances())
	}
}

// TestUnknownThemeFallsBackToTheDashboard keeps a hand-edited or older file
// from drawing something no code can render.
func TestUnknownThemeFallsBackToTheDashboard(t *testing.T) {
	dir := t.TempDir()
	data, err := json.Marshal(fileState{Version: 1, UI: savedUI{Theme: ptr("neon")}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "seeall.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	h := NewHost(dir)
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	if got := h.Theme(); got != ThemeDashboard {
		t.Errorf("a file saying %q drew %q, want the dashboard", "neon", got)
	}
}

func ptr[T any](v T) *T { return &v }
