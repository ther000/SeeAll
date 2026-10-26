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
// shows them in a single card: the device's name across the top, the level on
// the left, and link and DPI down the right, one to a line.
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

	// The other readings are in the same card, down its right side, one
	// reading to a line: a label and its value share a row, and the rows run
	// down the card.
	rows := []struct{ label, value string }{
		{"连接", "无线"},
		{"DPI", "1200"},
	}
	var previous float32
	for i, row := range rows {
		label, ok := tt.Find(row.label)
		if !ok {
			t.Fatalf("the card shows no %q: %v", row.label, tt.Texts())
		}
		value, ok := tt.Find(row.value)
		if !ok {
			t.Fatalf("the card shows no %q: %v", row.value, tt.Texts())
		}
		if label.X <= battery.X || value.X <= battery.X {
			t.Errorf("%q and %q sit at x=%v and x=%v, not to the right of the level at x=%v",
				row.label, row.value, label.X, value.X, battery.X)
		}
		if value.X <= label.X {
			t.Errorf("%q at x=%v is not to the right of %q at x=%v", row.value, value.X, row.label, label.X)
		}
		// Baseline alignment leaves a pixel or two between the two.
		if diff := label.Y - value.Y; diff > 6 || diff < -6 {
			t.Errorf("%q and %q are not on one row: y=%v vs y=%v", row.label, row.value, label.Y, value.Y)
		}
		if i > 0 && label.Y <= previous {
			t.Errorf("%q sits at y=%v, not below the reading before it at y=%v", row.label, label.Y, previous)
		}
		previous = label.Y
	}
}

// TestCombinedPageShowsNoTrendOrWording checks the combined card keeps to the
// number and the device: a battery page has no trend line and no "电量充足"
// beside the ring, which a glance at the ring already says.
func TestCombinedPageShowsNoTrendOrWording(t *testing.T) {
	_, p := combinedFixture(t)
	// A history long enough that the separate page would draw a trend.
	p.Host.InjectHistory("mouse", "battery", []float64{72, 70, 74, 71, 73, 72})
	p.SetSelected("mouse")
	p.SetSize(Fit(1920, 1040, 1))
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	for _, unwanted := range []string{"电量充足", "还够用", "该充电了"} {
		if tt.HasText(unwanted) {
			t.Errorf("the card says %q beside the ring: %v", unwanted, tt.Texts())
		}
	}
	if tt.HasText("最近") {
		t.Errorf("the card draws a trend: %v", tt.Texts())
	}
	// The value itself is still there.
	if !tt.HasText("72") {
		t.Errorf("the card shows no level: %v", tt.Texts())
	}
}

// TestEveryOtherPageKeepsItsCards checks the combined page is the plugin's
// choice: a plugin whose readings measure different things gets a card each,
// two to a row, all of them the same size.
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

	// The led reading comes first, so it and the one after it share the first
	// row, and the third starts the second.
	cpu, ok := tt.Find("CPU")
	if !ok {
		t.Fatalf("the system page shows no CPU reading: %v", tt.Texts())
	}
	mem, ok := tt.Find("内存")
	if !ok {
		t.Fatalf("the system page shows no memory reading: %v", tt.Texts())
	}
	disk, ok := tt.Find("磁盘 C:")
	if !ok {
		t.Fatalf("the system page shows no disk reading: %v", tt.Texts())
	}
	if cpu.Y != mem.Y {
		t.Errorf("CPU at y=%v and 内存 at y=%v are not a row of cards", cpu.Y, mem.Y)
	}
	if mem.X <= cpu.X {
		t.Errorf("内存 at x=%v is not beside CPU at x=%v", mem.X, cpu.X)
	}
	if disk.Y <= cpu.Y {
		t.Errorf("磁盘 at y=%v did not start a new row under CPU at y=%v", disk.Y, cpu.Y)
	}
	if disk.X != cpu.X {
		t.Errorf("磁盘 at x=%v does not start the row at CPU's x=%v", disk.X, cpu.X)
	}
}
