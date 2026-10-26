package panel

import (
	"sort"
	"testing"

	"github.com/egoist/mygo/ui"

	"seeall/internal/core"
)

// TestReadingsShareOneCardSize checks a page with one reading per card gives
// every reading the same box: the same width as its neighbour, and the same
// height whatever its reading has under the value. A card that grew with its
// detail line would leave the grid ragged.
func TestReadingsShareOneCardSize(t *testing.T) {
	h, p := fixture(t)
	h.Inject("system", core.Snapshot{Status: core.StatusOK, Title: "DESKTOP",
		Primary: "cpu", Readings: []core.Reading{
			// Two with a detail line under the value and one without, which is
			// what made the cards differ in height before.
			{ID: "cpu", Label: "CPU", Value: 23.4, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "16 核"},
			{ID: "mem", Label: "内存", Value: 61, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "19.4 GB / 31.8 GB"},
			{ID: "disk", Label: "磁盘 C:", Value: 78, Unit: core.UnitPercent,
				Severity: core.SeverityWarn, Detail: "212 GB 可用 / 953 GB"},
			{ID: "uptime", Label: "开机", Text: "2.4 h", Severity: core.SeverityNone},
		}})
	p.SetSize(Fit(1920, 1040, 1))
	p.SetSelected("system")
	tt := ui.NewTester(func(c *ui.Context) { p.View(c) }, DesignWidth, DesignHeight)

	// Each card's own top is its label's top; its bottom is the card's, which
	// the row below its label starts.
	labels := []string{"CPU", "内存", "磁盘 C:", "开机"}
	rows := map[float32][]float32{}
	for _, name := range labels {
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("the page shows no %q: %v", name, tt.Texts())
		}
		rows[r.Y] = append(rows[r.Y], r.X)
	}
	if len(rows) != 2 {
		t.Fatalf("the four cards sit on %d rows, want 2", len(rows))
	}

	var starts []float32
	for y, xs := range rows {
		sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
		if len(xs) != 2 {
			t.Errorf("the row at y=%v holds %d cards, want 2", y, len(xs))
		}
		starts = append(starts, y)
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })
	// The rows are one card and one row gap apart, so the cards are all one
	// height. GridView spaces its rows by two units of the theme's spacing.
	rowGap := float32(2) * gridGap
	if gap := starts[1] - starts[0]; gap != ReadingCardH+rowGap {
		t.Errorf("the rows are %v apart, want one card and one gap: %v",
			gap, float32(ReadingCardH)+rowGap)
	}
}
