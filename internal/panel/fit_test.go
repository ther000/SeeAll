package panel

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestFitKeepsTheDesignShape checks the panel is always the same shape: the
// design's pixels in the display's scale, shrunk evenly when the room is
// smaller. Every display then shows the same layout.
func TestFitKeepsTheDesignShape(t *testing.T) {
	for _, area := range []struct {
		name  string
		w, h  int
		scale float64
	}{
		{"desktop", 1698, 904, 1.5},
		{"laptop", 1358, 720, 1},
		{"tiny", 400, 300, 1},
	} {
		m := Fit(area.w, area.h, area.scale)
		if m.Width > area.w || m.Height > area.h {
			t.Errorf("%s: Fit(%d, %d, %v) = %dx%d, bigger than the room",
				area.name, area.w, area.h, area.scale, m.Width, m.Height)
		}
		// Within a pixel of the design's ratio, whatever the rounding.
		got := float64(m.Width) / float64(m.Height)
		want := float64(DesignWidth) / float64(DesignHeight)
		if diff := got - want; diff > 0.01 || diff < -0.01 {
			t.Errorf("%s: Fit(%d, %d, %v) = %dx%d, ratio %v, want %v",
				area.name, area.w, area.h, area.scale, m.Width, m.Height, got, want)
		}
	}
}

// TestFitTurnsPixelsIntoDIPs checks the design's pixel size is what the window
// asks for in DIPs: 660×840 pixels is 660×840 DIPs at 100%, 440×560 at 150%
// and 330×420 at 200%, so the panel covers the same pixels of the screen.
func TestFitTurnsPixelsIntoDIPs(t *testing.T) {
	for _, c := range []struct {
		scale        float64
		wantW, wantH int
	}{
		{1, 660, 840},
		{1.5, 440, 560},
		{2, 330, 420},
	} {
		m := Fit(4000, 4000, c.scale)
		if m.Width != c.wantW || m.Height != c.wantH {
			t.Errorf("Fit at %vx = %dx%d DIPs, want %dx%d",
				c.scale, m.Width, m.Height, c.wantW, c.wantH)
		}
	}
}

// TestFitShrinksIntoASmallRoom checks a display too small for the design gets
// a smaller panel of the same shape, rather than a clipped one.
func TestFitShrinksIntoASmallRoom(t *testing.T) {
	m := Fit(440, 480, 1)
	if m.Height != 480 {
		t.Errorf("Fit(440, 480, 1) = %dx%d, want the height to fill 480", m.Width, m.Height)
	}
	if m.Width != 480*DesignWidth/DesignHeight {
		t.Errorf("Fit(440, 480, 1) = %dx%d, want the width in proportion", m.Width, m.Height)
	}

	// The width is the tighter of the two when a narrow display is tall.
	wantH := int(math.Round(float64(300) * DesignHeight / DesignWidth))
	if m := Fit(300, 4000, 1); m.Width != 300 || m.Height != wantH {
		t.Errorf("Fit(300, 4000, 1) = %dx%d, want 300x%d", m.Width, m.Height, wantH)
	}
}

// TestMarginDIP checks the pixel margin becomes DIPs: 8 pixels is 8 DIPs at
// 100%, 5 at 150%, 4 at 200%, and never nothing.
func TestMarginDIP(t *testing.T) {
	for _, c := range []struct {
		scale float64
		want  int
	}{{1, 8}, {1.5, 5}, {2, 4}, {0, 8}, {3, 3}} {
		if got := MarginDIP(c.scale); got != c.want {
			t.Errorf("MarginDIP(%v) = %d, want %d", c.scale, got, c.want)
		}
	}
}

// TestFitFitsEveryPage renders each page at the sizes Fit gives a display and
// checks that the status bar — the last thing in every window — and the tab
// row are inside it. A page whose content outgrows the panel clips it, which
// is what the fixed-size layout has to prevent.
func TestFitFitsEveryPage(t *testing.T) {
	for _, area := range []struct {
		name  string
		w, h  int
		scale float64
	}{
		{"desktop", 1698, 904, 1.5},
		{"laptop", 1358, 720, 1},
	} {
		m := Fit(area.w, area.h, area.scale)
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
