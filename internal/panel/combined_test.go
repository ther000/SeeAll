package panel

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
)

// combinedFixture makes a host whose mouse reports the readings a combined
// page shows: battery, link and DPI.
func combinedFixture(t *testing.T) (*app.Host, *App) {
	t.Helper()
	h, p := fixture(t)
	h.Inject("mouse", core.Snapshot{Status: core.StatusOK, Title: "Rapoo Gaming Device",
		Subtitle: "无线 · 72%", Primary: "battery", Readings: []core.Reading{
			{ID: "battery", Label: "电量", Value: 72, Unit: core.UnitPercent, Severity: core.SeverityOK},
			{ID: "link", Label: "连接", Text: "无线", Severity: core.SeverityOK},
			{ID: "dpi", Label: "DPI", Value: 1200, Severity: core.SeverityNone},
		}})
	return h, p
}

// TestCombinedPageIsOneCard checks a plugin whose readings describe one device
// shows them in a single card: the device's name, the headline reading, and
// link and DPI together under it, rather than as cards of their own.
func TestCombinedPageIsOneCard(t *testing.T) {
	_, p := combinedFixture(t)
	p.SetSelected("mouse")
	p.SetSize(Fit(1920, 1040, 1))
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	// The device is named at the top of the card.
	name, ok := tt.Find("Rapoo Gaming Device")
	if !ok {
		t.Fatalf("the page does not name the device: %v", tt.Texts())
	}
	battery, ok := tt.Find("72")
	if !ok {
		t.Fatalf("the page shows no battery level: %v", tt.Texts())
	}
	if battery.Y <= name.Y {
		t.Errorf("the battery sits above the device name: %v vs %v", battery.Y, name.Y)
	}

	// The other readings are in the same card, under the level, so they must
	// sit below it and above the card's bottom edge.
	for _, want := range []string{"连接", "无线", "DPI", "1200"} {
		r, ok := tt.Find(want)
		if !ok {
			t.Fatalf("the card shows no %q: %v", want, tt.Texts())
		}
		if r.Y <= battery.Y {
			t.Errorf("%q sits at y=%v, above the level at y=%v", want, r.Y, battery.Y)
		}
	}

	// One card means no gap of a card between the level and the readings
	// under it: the readings are in the same box, close below it.
	link, _ := tt.Find("连接")
	if link.Y-battery.Y > 160 {
		t.Errorf("the readings under the level are %v away: a second card", link.Y-battery.Y)
	}
}

// TestEveryOtherPageKeepsItsCards checks the combined page is the plugin's
// choice: a plugin whose readings measure different things still gets a card
// each.
func TestEveryOtherPageKeepsItsCards(t *testing.T) {
	h, p := fixture(t)
	h.Inject("system", core.Snapshot{Status: core.StatusOK, Title: "DESKTOP",
		Primary: "cpu", Readings: []core.Reading{
			{ID: "cpu", Label: "CPU", Value: 23, Unit: core.UnitPercent, Severity: core.SeverityOK},
			{ID: "mem", Label: "内存", Value: 61, Unit: core.UnitPercent, Severity: core.SeverityOK},
			{ID: "disk", Label: "磁盘 C:", Value: 78, Unit: core.UnitPercent, Severity: core.SeverityWarn},
		}})
	p.SetSelected("system")
	p.SetSize(Fit(1920, 1040, 1))
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	mem, ok := tt.Find("内存")
	if !ok {
		t.Fatalf("the system page shows no memory reading: %v", tt.Texts())
	}
	disk, ok := tt.Find("磁盘 C:")
	if !ok {
		t.Fatalf("the system page shows no disk reading: %v", tt.Texts())
	}
	// Two cards in a row sit side by side; one card would stack them.
	if mem.Y != disk.Y {
		t.Errorf("内存 at y=%v and 磁盘 at y=%v are not a row of cards", mem.Y, disk.Y)
	}
	if disk.X <= mem.X {
		t.Errorf("磁盘 at x=%v is not beside 内存 at x=%v", disk.X, mem.X)
	}
}
