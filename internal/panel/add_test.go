package panel

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
)

// TestAddBringsBackARemovedMonitor is the bug the settings page had: removing a
// monitor left no way to add it back, because the add card only knew plugins
// that take several instances, and none of the built-in ones do.
func TestAddBringsBackARemovedMonitor(t *testing.T) {
	h, p := fixture(t)
	p.SetSelected(SettingsPage)
	p.SetSize(Fit(1920, 1040, 1))

	// Remove the mouse, with a setting changed first: it has to come back
	// with that setting, not with the default.
	mouse := h.Instance("mouse")
	if mouse == nil {
		t.Fatalf("no mouse instance: %v", h.Instances())
	}
	mouse.Settings().Set("vid", "1234")
	h.Remove("mouse")
	if h.Instance("mouse") != nil {
		t.Fatal("the mouse was not removed")
	}

	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)
	if !tt.HasText("添加") {
		t.Fatalf("the add card is missing after removing a monitor: %v", tt.Texts())
	}
	if err := tt.Click("添加监控项"); err != nil {
		t.Fatal(err)
	}

	back := h.Instance("mouse")
	if back == nil {
		t.Fatalf("the mouse was not added back: %v", h.Instances())
	}
	if got := back.Settings().Get("vid"); got != "1234" {
		t.Errorf("the mouse came back with vid %q, want 1234", got)
	}
	if p.Selected() != "mouse" {
		t.Errorf("selected %q after adding, want mouse", p.Selected())
	}
}

// TestAddCardHidesWhatAlreadyRuns keeps the add card offering only the plugins
// without a monitor: a second mouse would replace the first one's reading.
func TestAddCardHidesWhatAlreadyRuns(t *testing.T) {
	h, p := fixture(t)
	p.SetSelected(SettingsPage)
	p.SetSize(Fit(1920, 1040, 1))

	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)
	if tt.HasText("添加") {
		t.Fatalf("the add card offers a monitor while every plugin runs: %v", tt.Texts())
	}

	h.Remove("system")
	tt = ui.NewTester(p.View, DesignWidth, DesignHeight)
	names := p.missingPlugins()
	if len(names) != 1 || names[0].Manifest().ID != "system" {
		t.Fatalf("missing plugins = %v, want just the system monitor", names)
	}
}

// TestRemovedMonitorStaysRemovedAcrossRestart checks the settings file records
// that a monitor was taken away: starting again must not put it back behind
// the user's back.
func TestRemovedMonitorStaysRemovedAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	h := app.NewHost(dir)
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	h.Remove("system")
	if h.Instance("system") != nil {
		t.Fatal("the system monitor was not removed")
	}

	again := app.NewHost(dir)
	if err := again.Load(); err != nil {
		t.Fatal(err)
	}
	again.Stop()
	if again.Instance("system") != nil {
		t.Fatal("the removed system monitor came back after a restart")
	}
	if again.Instance("mouse") == nil {
		t.Fatal("the mouse monitor was lost across a restart")
	}
}

// TestEmptyPanelStaysEmptyAcrossRestart is the same bug at its worst: with
// every monitor removed, the settings file holds no instance, and seeding the
// defaults from an empty file brought them all back at the next launch.
func TestEmptyPanelStaysEmptyAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	h := app.NewHost(dir)
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	for _, in := range h.Instances() {
		h.Remove(in.ID)
	}
	if len(h.Instances()) != 0 {
		t.Fatalf("instances left after removing them all: %v", h.Instances())
	}

	again := app.NewHost(dir)
	if err := again.Load(); err != nil {
		t.Fatal(err)
	}
	again.Stop()
	if len(again.Instances()) != 0 {
		t.Fatalf("a panel emptied on purpose came back with %v", again.Instances())
	}
}

// TestRemovedSettingsComeBack checks removing a monitor keeps its settings on
// disk, so that adding it back restores how it was set up rather than a fresh
// one.
func TestRemovedSettingsComeBack(t *testing.T) {
	dir := t.TempDir()
	h := app.NewHost(dir)
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	h.Instance("mouse").Settings().Set("vid", "ABCD")
	h.Remove("mouse")

	again := app.NewHost(dir)
	if err := again.Load(); err != nil {
		t.Fatal(err)
	}
	again.Stop()
	back := again.Add("mouse", "", nil)
	if back == nil {
		t.Fatal("the mouse could not be added back")
	}
	if got := back.Settings().Get("vid"); got != "ABCD" {
		t.Errorf("the mouse came back with vid %q, want ABCD", got)
	}

	// The settings are no longer waiting to be restored: a second removal
	// saves them afresh rather than keeping a stale copy.
	if again.Instance("mouse") == nil {
		t.Fatal("the mouse is not running after being added back")
	}
	again.Instance("mouse").Settings().Set("vid", "EFGH")
	again.Remove("mouse")

	third := app.NewHost(dir)
	if err := third.Load(); err != nil {
		t.Fatal(err)
	}
	third.Stop()
	yetanother := third.Add("mouse", "", nil)
	if got := yetanother.Settings().Get("vid"); got != "EFGH" {
		t.Errorf("the mouse came back with vid %q, want EFGH", got)
	}
}

// TestEveryMonitorCanBeAddedBack removes each built-in monitor in turn and
// checks the settings page offers it again.
func TestEveryMonitorCanBeAddedBack(t *testing.T) {
	for _, id := range []string{"mouse", "system"} {
		h, p := fixture(t)
		if h.Instance(id) == nil {
			t.Fatalf("%s is not a built-in monitor", id)
		}
		h.Remove(id)
		p.SetSelected(SettingsPage)
		p.SetSize(Fit(1920, 1040, 1))

		missing := p.missingPlugins()
		found := false
		for _, pl := range missing {
			if pl.Manifest().ID == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s cannot be added back: missing = %v", id, missing)
		}

		tt := ui.NewTester(p.View, DesignWidth, DesignHeight)
		if err := tt.Click("添加监控项"); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if h.Instance(id) == nil {
			t.Fatalf("%s was not added back", id)
		}
	}
}
