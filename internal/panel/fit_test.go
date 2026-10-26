package panel

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestFitKeepsTheDesignShape checks the panel is always the same shape: the
// largest window of the design's proportions that fits the room it gets, so
// every display shows the same layout at a different scale.
func TestFitKeepsTheDesignShape(t *testing.T) {
	for _, area := range []struct {
		name string
		w, h int
	}{
		{"desktop", 1698, 904},
		{"laptop", 1358, 720},
		{"tiny", 400, 300},
	} {
		m := Fit(area.w, area.h)
		if m.Width > area.w || m.Height > area.h {
			t.Errorf("%s: Fit(%d, %d) = %dx%d, bigger than the room",
				area.name, area.w, area.h, m.Width, m.Height)
		}
		// Within a pixel of the design's ratio, whatever the rounding.
		got := float64(m.Width) / float64(m.Height)
		want := float64(DesignWidth) / float64(DesignHeight)
		if diff := got - want; diff > 0.01 || diff < -0.01 {
			t.Errorf("%s: Fit(%d, %d) = %dx%d, ratio %v, want %v",
				area.name, area.w, area.h, m.Width, m.Height, got, want)
		}
	}
}

// TestFitFillsARoomyAreaAtFullSize keeps the design's own size on a display
// with room to spare, rather than growing into it: a tray panel the size of
// the screen is not what the layout is for.
func TestFitFillsARoomyAreaAtFullSize(t *testing.T) {
	if m := Fit(1698, 904); m.Width != DesignWidth || m.Height != DesignHeight {
		t.Errorf("Fit(1698, 904) = %dx%d, want %dx%d", m.Width, m.Height, DesignWidth, DesignHeight)
	}
}

// TestFitFitsEveryPage renders each page at the sizes Fit gives a display and
// checks that the status bar — the last thing in every window — and the tab
// row are inside it. A page whose content outgrows the panel clips it, which
// is what the fixed-size layout has to prevent.
func TestFitFitsEveryPage(t *testing.T) {
	for _, area := range []struct {
		name string
		w, h int
	}{
		{"desktop", 1698, 904},
		{"laptop", 1358, 720},
	} {
		m := Fit(area.w, area.h)
		for _, row := range []string{"mouse", "system", SettingsPage} {
			h, _ := fixture(t)
			p := New(h)
			p.SetSize(m)
			p.SetSelected(row)

			tt := ui.NewTester(p.View, m.Width, m.Height)
			r, ok := tt.Find("退出")
			if !ok {
				t.Fatalf("%s/%s: the status bar is missing", area.name, row)
			}
			if r.Y+r.H > float32(m.Height) {
				t.Errorf("%s/%s: the status bar is clipped (bottom %v of %d)",
					area.name, row, r.Y+r.H, m.Height)
			}
			if r.Y < float32(m.Height)*0.8 {
				t.Errorf("%s/%s: the status bar sits at y=%v, not at the foot of %d",
					area.name, row, r.Y, m.Height)
			}
		}
	}
}
