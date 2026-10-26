package panel

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestFitFitsEveryPage renders each page at the size Fit gives a display and
// checks that the footer — the last thing on every monitor page — sits inside
// the window. A page whose content outgrows the panel clips it, which is what
// the fixed-size layout has to prevent.
func TestFitFitsEveryPage(t *testing.T) {
	sizes := []struct {
		name         string
		workW, workH int
	}{
		{"desktop", 2560, 1368},
		{"laptop", 1366, 728},
	}
	for _, size := range sizes {
		m := Fit(size.workW, size.workH)
		for _, row := range []string{"mouse", "system", "server:example.com", "agent"} {
			h, _ := fixture(t)
			p := New(h)
			p.SetSize(m)
			p.SetSelected(row)

			tt := ui.NewTester(p.View, m.Width, m.Height)
			r, ok := tt.Find("刷新")
			if !ok {
				t.Fatalf("%s/%s: the refresh button is missing", size.name, row)
			}
			if r.Y+r.H > float32(m.Height) {
				t.Errorf("%s/%s: the footer is clipped (bottom %v of %d)",
					size.name, row, r.Y+r.H, m.Height)
			}
		}
	}
}
