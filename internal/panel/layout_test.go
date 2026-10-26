package panel

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
)

// fixture makes a host with its built-in instances, without polling.
func fixture(t *testing.T) (*app.Host, *App) {
	t.Helper()
	h := app.NewHost(t.TempDir())
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	h.OnChange = func() {}
	h.Inject("mouse", core.Snapshot{Status: core.StatusOK, Subtitle: "无线 · 72%",
		Primary: "battery", Readings: []core.Reading{
			{ID: "battery", Label: "电量", Value: 72, Unit: core.UnitPercent, Severity: core.SeverityOK},
			{ID: "link", Label: "连接", Text: "无线", Severity: core.SeverityOK},
		}})
	return h, New(h)
}

// TestSettingsStartsAtTheTop guards the bug the fixed-size panel had in the
// old app: a page longer than the window must scroll, not push its own
// content out of view. The tab row stays where it is above it.
func TestSettingsStartsAtTheTop(t *testing.T) {
	_, p := fixture(t)
	p.SetSelected(SettingsPage)
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	tab, ok := tt.Find("鼠标")
	if !ok {
		t.Fatalf("the tab row is missing: %v", tt.Texts())
	}
	first, ok := tt.Find("应用")
	if !ok {
		t.Fatalf("the settings page is empty: %v", tt.Texts())
	}
	if first.Y <= tab.Y {
		t.Fatalf("the settings content sits at y=%v, above the tabs at y=%v", first.Y, tab.Y)
	}
	// Directly under the tabs, not pushed to the middle or the foot by a
	// page that is longer than the window.
	if first.Y > float32(DesignHeight)/4 {
		t.Fatalf("the settings content starts at y=%v, not under the tabs at y=%v", first.Y, tab.Y)
	}

	texts := strings.Join(tt.Texts(), " ")
	if !strings.Contains(texts, "鼠标") {
		t.Fatalf("the settings page lists no monitor: %s", texts)
	}
}

// TestMonitorPageLeadsWithItsReading checks the detail page leads with the
// reading the plugin named, in the big value.
func TestMonitorPageLeadsWithItsReading(t *testing.T) {
	_, p := fixture(t)
	p.SetSelected("mouse")
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	if !tt.HasText("72") {
		t.Fatalf("the mouse page shows no level: %v", tt.Texts())
	}
	if !tt.HasText("无线") {
		t.Fatalf("the mouse page shows no link: %v", tt.Texts())
	}
}

// TestTabsChooseThePage checks a click on a tab shows that monitor, and that
// the tab row follows a page chosen from elsewhere, as the tray menu does.
func TestTabsChooseThePage(t *testing.T) {
	h, p := fixture(t)
	if len(h.Instances()) < 2 {
		t.Skip("needs at least two built-in monitors")
	}
	p.SetSelected("system")
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	// "鼠标" is the tab of the other monitor and the title of its page, so
	// the click has one row to land on here: the page shows the system.
	if err := tt.Click("鼠标"); err != nil {
		t.Fatal(err)
	}
	if p.Selected() != "mouse" {
		t.Fatalf("selected %q, want mouse", p.Selected())
	}

	if err := tt.Click("打开设置"); err != nil {
		t.Fatal(err)
	}
	if p.Selected() != SettingsPage {
		t.Fatalf("selected %q, want the settings", p.Selected())
	}

	// Chosen from the tray menu, the tabs must point at the page shown.
	p.SetSelected("system")
	tt2 := ui.NewTester(p.View, DesignWidth, DesignHeight)
	tab, ok := tt2.Find("本机")
	if !ok {
		t.Fatalf("the system tab is missing: %v", tt2.Texts())
	}
	body, ok := tt2.Find("CPU")
	if !ok {
		t.Fatalf("the system page is missing: %v", tt2.Texts())
	}
	if body.Y <= tab.Y {
		t.Fatalf("the system page sits above the tabs: y=%v, tabs at %v", body.Y, tab.Y)
	}
}
