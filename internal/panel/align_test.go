package panel

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"seeall/internal/core"
)

// alignedFixture makes a host with both monitors reporting the readings this
// test expects, so that the page it measures is the page it described.
func alignedFixture(t *testing.T) (*App, func(row string) *ui.Tester) {
	t.Helper()
	h, p := fixture(t)
	h.Inject("system", core.Snapshot{Status: core.StatusOK, Title: "DESKTOP",
		Primary: "cpu", Readings: []core.Reading{
			{ID: "cpu", Label: "CPU", Value: 23.4, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "16 核"},
			{ID: "mem", Label: "内存", Value: 61, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "19.4 GB / 31.8 GB"},
		}})
	p.SetSize(Fit(1920, 1040, 1))
	return p, func(row string) *ui.Tester {
		p.SetSelected(row)
		return ui.NewTester(func(c *ui.Context) { p.View(c) }, DesignWidth, DesignHeight)
	}
}

// TestPagesShareOneGutter checks the panel's pages line up with one another:
// the text at the left edge of a card and the text at its right edge sit on the
// same two vertical lines on every page. A button a few pixels in from the
// card's edge, or a page whose gutter differs, is what this catches.
func TestPagesShareOneGutter(t *testing.T) {
	pages := []struct {
		row string
		// left and right name a text at the card's left edge and one at its
		// right: a title on the left, a value or a control on the right.
		left, right string
	}{
		{"mouse", "Rapoo Gaming Device", "无线"},
		{"system", "CPU", "19.4 GB / 31.8 GB"},
		{SettingsPage, "应用", "全部刷新"},
	}

	// The first page sets the lines; the others must land on them.
	var wantLeft, wantRight float32
	for i, page := range pages {
		p, tester := alignedFixture(t)
		_ = p
		tt := tester(page.row)

		left, ok := tt.Find(page.left)
		if !ok {
			t.Fatalf("%s: the card's left edge is missing (%q): %v", page.row, page.left, tt.Texts())
		}
		right, ok := tt.Find(page.right)
		if !ok {
			t.Fatalf("%s: the card's right edge is missing (%q): %v", page.row, page.right, tt.Texts())
		}
		if i == 0 {
			wantLeft, wantRight = left.X, right.X+right.W
		}
		if left.X != wantLeft {
			t.Errorf("%s: %q starts at x=%v, but the pages before it start at %v",
				page.row, page.left, left.X, wantLeft)
		}
		if got := right.X + right.W; got != wantRight {
			t.Errorf("%s: %q ends at x=%v, but the pages before it end at %v",
				page.row, page.right, got, wantRight)
		}
	}

	// And the lines are the panel's gutter plus a card's padding and border,
	// not a number that happens to agree.
	const border = 1
	if want := float32(Gutter + cardPadX + border); wantLeft != want {
		t.Errorf("the cards start their text at x=%v, want %v", wantLeft, want)
	}
	if want := float32(DesignWidth - Gutter - cardPadX - border); wantRight != want {
		t.Errorf("the cards end their text at x=%v, want %v", wantRight, want)
	}
}
