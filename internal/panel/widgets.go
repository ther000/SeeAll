package panel

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// ring draws a percentage as an arc: the track first, then the filled part,
// from twelve o'clock clockwise.
func ring(c *ui.Context, p palette, value float64, color ui.Color, width float32) ui.Element {
	v := clamp01(value / 100)
	return ui.Box(c).Fill().Draw(func(pt *ui.Painter, r ui.Rect) {
		drawRing(pt, r, p.Track, v, color, width)
	})
}

// drawRing paints the track and the lit part of a ring in r.
func drawRing(pt *ui.Painter, r ui.Rect, track ui.Color, v float64, color ui.Color, width float32) {
	size := min32(r.W, r.H)
	radius := (size - width) / 2
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	pt.StrokePath(circlePath(cx, cy, radius), width, track)
	switch {
	case v <= 0:
	case v >= 1:
		pt.StrokePath(circlePath(cx, cy, radius), width, color)
	default:
		pt.StrokePath(arcPath(cx, cy, radius, 0, 360*float32(v)), width, color)
	}
}

// arcPath draws part of a circle clockwise from startDeg to endDeg, in
// degrees from twelve o'clock.
func arcPath(cx, cy, radius, startDeg, endDeg float32) *ui.Path {
	path := &ui.Path{}
	steps := 48
	if sweep := endDeg - startDeg; sweep > 0 && sweep < 24 {
		steps = int(max32(sweep/2, 4))
	}
	for i := 0; i <= steps; i++ {
		angle := float64(startDeg+float32(i)/float32(steps)*(endDeg-startDeg)) * math.Pi / 180
		x := cx + radius*float32(math.Sin(angle))
		y := cy - radius*float32(math.Cos(angle))
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	return path
}

// circlePath is a full circle as a path.
func circlePath(cx, cy, radius float32) *ui.Path {
	return arcPath(cx, cy, radius, 0, 359.999)
}

// dot draws a small filled circle, the sidebar's status light.
func dot(c *ui.Context, color ui.Color, size float32) ui.Element {
	return ui.Box(c).Size(size, size).Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Fill(r, color, size/2)
	})
}

// bigValue is the large number of the detail page.
func bigValue(c *ui.Context, text string, size float32, color ui.Color) ui.Element {
	return ui.Text(c, text).FontSize(size).Bold().TextColor(color).NoWrap()
}

// label is the small, muted text of a card's heading.
func label(c *ui.Context, p palette, text string) ui.Element {
	return ui.Text(c, text).FontSize(11).TextColor(p.TextMuted).NoWrap()
}

// card is the panel's surface with a faint outline.
func card(c *ui.Context, p palette) ui.Element {
	return ui.Column(c).Padding(14).Gap(10).Radius(12).
		Background(p.Card).Border(1, p.CardBorder)
}

// panelCard is a card that fills the room its column gives it, for the pages
// that hold two of them side by side.
func panelCard(c *ui.Context, p palette) ui.Element {
	return card(c, p).FillHeight().Grow(1)
}

// sparkline draws the recent values of a reading as a small line, with the
// newest point marked.
func sparkline(c *ui.Context, p palette, values []float64, color ui.Color, height float32) ui.Element {
	return ui.Box(c).Height(height).Draw(func(pt *ui.Painter, r ui.Rect) {
		if len(values) < 2 {
			return
		}
		lo, hi := values[0], values[0]
		for _, v := range values {
			lo = math.Min(lo, v)
			hi = math.Max(hi, v)
		}
		if hi-lo < 1e-9 {
			hi = lo + 1
		}
		path := &ui.Path{}
		for i, v := range values {
			x := r.X + r.W*float32(i)/float32(len(values)-1)
			y := r.Y + r.H - r.H*float32((v-lo)/(hi-lo))
			if i == 0 {
				path.MoveTo(x, y)
			} else {
				path.LineTo(x, y)
			}
		}
		pt.StrokePath(path, 1.5, color)
		last := values[len(values)-1]
		x := r.X + r.W
		y := r.Y + r.H - r.H*float32((last-lo)/(hi-lo))
		pt.Fill(ui.Rect{X: x - 2, Y: y - 2, W: 4, H: 4}, color, 2)
	})
}

// meter is a rounded bar filled to a share of its width.
func meter(c *ui.Context, p palette, share float64, color ui.Color, height float32) ui.Element {
	return ui.Box(c).Height(height).Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Fill(r, p.Track, height/2)
		w := float32(clamp01(share)) * r.W
		if w <= 0 {
			return
		}
		if w < height {
			w = height
		}
		pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: w, H: r.H}, color, height/2)
	})
}

// badge is a small pill, used for a source's status word.
func badge(c *ui.Context, text string, color ui.Color) ui.Element {
	return ui.Box(c).Padding(2, 7).Radius(999).Background(color.Alpha(0.14)).
		Children(func() {
			ui.Text(c, text).FontSize(11).TextColor(color).NoWrap()
		})
}

// hline is a hairline between two groups of content.
func hline(c *ui.Context, p palette) ui.Element {
	return ui.Box(c).Height(1).MarginY(2).Background(p.CardBorder)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
