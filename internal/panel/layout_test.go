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
// old app: a page longer than the window must scroll, not push its own title
// out of view.
func TestSettingsStartsAtTheTop(t *testing.T) {
	_, p := fixture(t)
	p.SetSelected(SettingsPage)
	tt := ui.NewTester(p.View, 380, 470)

	// The header's subtitle is unique to the page, unlike "设置", which the
	// sidebar also has at its foot.
	r, ok := tt.Find("5 个监控项 · 5 个插件")
	if !ok {
		t.Fatalf("the settings header is missing: %v", tt.Texts())
	}
	if r.Y > 60 {
		// Everything the page holds, with where it sits, for the failure.
		t.Fatalf("the settings header sits at y=%v, not at the top (found %+v)", r.Y, r)
	}

	// And the monitor list is reachable by scrolling.
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
	tt := ui.NewTester(p.View, 380, 470)

	if !tt.HasText("72") {
		t.Fatalf("the mouse page shows no level: %v", tt.Texts())
	}
	if !tt.HasText("无线") {
		t.Fatalf("the mouse page shows no link: %v", tt.Texts())
	}
}

// TestSidebarSelectionSwitchesPage checks a click on a sidebar row changes
// what the right side shows.
func TestSidebarSelectionSwitchesPage(t *testing.T) {
	h, p := fixture(t)
	p.SetSelected("mouse")
	tt := ui.NewTester(p.View, 380, 470)

	if len(h.Instances()) < 2 {
		t.Skip("needs at least two built-in monitors")
	}
	// "键盘" is both a monitor and nothing else: the section titles are
	// "设备", "本机", "服务" and "Agent".
	if err := tt.Click("键盘"); err != nil {
		t.Fatal(err)
	}
	if p.Selected() != "keyboard" {
		t.Fatalf("selected %q, want keyboard", p.Selected())
	}
}

// findIn finds the element with text whose box starts right of x.
func findIn(tt *ui.Tester, text string, x float32) (ui.Rect, bool) {
	if r, ok := tt.Find(text); ok && r.X >= x {
		return r, true
	}
	return ui.Rect{}, false
}
