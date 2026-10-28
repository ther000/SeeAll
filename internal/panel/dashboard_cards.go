package panel

import (
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
)

// The dashboard draws a reading's shape by what the reading measures: a line
// is a thing that moves, an area is how full something is, and a bar is how
// far along it is. The three are drawn here rather than by a chart widget so
// that they all start and end on the same edges of a card, whatever their
// data.

// dashColor is the accent a reading's icon and chart are drawn in. The number
// itself stays the panel's brightest text: color says which card, not how bad
// the value is.
func dashColor(d dash, r core.Reading) ui.Color {
	switch r.ID {
	case "gpu":
		return d.amber
	case "mem":
		return d.gold
	case "disk", "net":
		return d.blue
	case "temp":
		return d.teal
	case "cpu":
		return d.teal
	}
	if r.Severity != core.SeverityNone {
		return d.palette().severityColor(r.Severity)
	}
	return d.teal
}

// dashIcon is the icon of a reading's card: what the reading watches.
func dashIcon(id string) *ui.SVG {
	switch id {
	case "gpu":
		return iconGPU
	case "mem":
		return iconMem
	case "disk":
		return iconDisk
	case "net":
		return iconNet
	case "temp":
		return iconTemp
	case "battery":
		return iconBattery
	case "cpu":
		return iconCPU
	}
	return iconGauge
}

// dashValue splits a reading into the number a card sets large and the unit
// it sets small. A reading made of words has no unit to split, and is set as
// it reads.
func dashValue(r core.Reading) (value, unit string) {
	if r.Text != "" {
		return splitUnit(r.Text)
	}
	if r.Unit == core.UnitPercent {
		return core.FormatNumber(r.Value), "%"
	}
	return splitUnit(r.Format())
}

// splitUnit cuts the unit off the end of a value: "54°C" is 54 and °C,
// "2.1 MB/s" is 2.1 and MB/s, "已连接" is all words. A value that ends in a
// digit has no unit, and is returned whole.
func splitUnit(s string) (value, unit string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	i := len(s)
	for i > 0 {
		c := s[i-1]
		if c >= '0' && c <= '9' || c == '.' || c == '-' {
			break
		}
		i--
	}
	if i == 0 || i == len(s) {
		return s, ""
	}
	value = strings.TrimSpace(s[:i])
	if value == "" {
		return s, ""
	}
	return value, strings.TrimSpace(s[i:])
}

// dashDetail is the line under a card's number: what the number does not say
// by itself. The parts of a machine that idle say how hard they are working
// before they say what else they know.
func dashDetail(snap core.Snapshot, r core.Reading) string {
	if r.ID != "cpu" && r.ID != "gpu" {
		return r.Detail
	}
	parts := make([]string, 0, 2)
	if r.Text == "" && r.Unit == core.UnitPercent {
		parts = append(parts, loadWord(r.Value))
	}
	switch {
	case r.ID == "gpu":
		// The card already draws the GPU's temperature and its load; what
		// the vram reading adds is how much of the card's memory those
		// frames took, which belongs on the same line.
		if vram, ok := snap.Reading("vram"); ok && vram.Detail != "" {
			parts = append(parts, vram.Detail)
		} else if r.Detail != "" {
			parts = append(parts, r.Detail)
		}
	case r.Detail != "":
		parts = append(parts, r.Detail)
	}
	return strings.Join(parts, " · ")
}

// loadWord turns a level into the two words a card says before its detail.
func loadWord(percent float64) string {
	switch {
	case percent < 30:
		return "低负载"
	case percent < 70:
		return "中负载"
	}
	return "高负载"
}

// usedFraction is how much of the disk is used, as its detail line says it:
// "已用 96 GB · 60%". The reading itself is the room that is left, which
// cannot say how much of the disk there was.
func usedFraction(r core.Reading) (float64, bool) {
	if v, ok := percentIn(r.Detail); ok {
		return clamp01(v / 100), true
	}
	if v, ok := percentIn(r.Badge); ok {
		return clamp01(v / 100), true
	}
	return 0, false
}

// percentIn finds a percentage written in a line of text, as the disk's and
// the battery's details write theirs.
func percentIn(s string) (float64, bool) {
	end := strings.IndexByte(s, '%')
	if end <= 0 {
		return 0, false
	}
	start := end
	for start > 0 {
		c := s[start-1]
		if c >= '0' && c <= '9' || c == '.' {
			start--
			continue
		}
		break
	}
	if start == end {
		return 0, false
	}
	v, err := strconv.ParseFloat(s[start:end], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// dashChart draws one reading's shape in a box every card keeps at the same
// height, so that the cards of a row line up whether their readings have a
// history or not.
func dashChart(c *ui.Context, d dash, in *app.Instance, r core.Reading, color ui.Color) ui.Element {
	history := in.History(r.ID)
	return ui.Box(c).Shrink(0).Height(dashChartH).Draw(func(pt *ui.Painter, rect ui.Rect) {
		drawDashChart(pt, rect, d, r, history, color)
	})
}

func drawDashChart(pt *ui.Painter, r ui.Rect, d dash, reading core.Reading, history []float64, color ui.Color) {
	switch reading.ID {
	case "disk":
		if used, ok := usedFraction(reading); ok {
			drawBar(pt, r, d.track, used, d.blue)
		}
		return
	case "temp":
		drawBar(pt, r, d.track, clamp01(reading.Value/100), d.teal)
		return
	case "mem":
		// Memory is a share of the whole, and what the area says is how
		// close to full it has been: the highest value of the window is the
		// top of the box and the fill under it says how much of it is used.
		if len(history) >= 2 {
			drawArea(pt, r, history, d.gold, 0, top(history))
			return
		}
	}
	// A reading the dashboard has no shape for, with no history to shape,
	// is not a chart of anything: "无线" gets an empty box rather than a
	// dashed line that means nothing. The parts of a machine always keep
	// their box, flat or not, so that a row of cards stays a row.
	if len(history) < 2 && !dashShaped(reading.ID) {
		return
	}
	if !varies(history) {
		drawDashed(pt, r, color)
		return
	}
	lo, hi := spanOf(history)
	drawLine(pt, r, history, color, lo, hi)
}

// dashShaped reports whether a reading's card draws a shape even when the
// reading has no history yet: a load and a rate are flat lines when nothing
// moves, which is worth saying.
func dashShaped(id string) bool {
	switch id {
	case "cpu", "gpu", "mem", "net":
		return true
	}
	return false
}

// top is the height a filled chart is drawn against: the highest value in it,
// with a little air above, so that the area fills the box and its line still
// has room to move.
func top(values []float64) float64 {
	hi := 0.0
	for _, v := range values {
		hi = math.Max(hi, v)
	}
	hi *= 1.05
	if hi <= 0 {
		return 1
	}
	return hi
}

// spanOf is the range a series is drawn against, grown a little at both ends
// so that its highest and lowest points are not on the card's edge.
func spanOf(values []float64) (lo, hi float64) {
	lo, hi = values[0], values[0]
	for _, v := range values {
		lo = math.Min(lo, v)
		hi = math.Max(hi, v)
	}
	if hi-lo < 1e-9 {
		hi = lo + 1
	}
	pad := (hi - lo) * 0.18
	return lo - pad, hi + pad
}

// point places a series' point in r, given the range it is drawn against.
func point(r ui.Rect, i, n int, v, lo, hi float64) (x, y float32) {
	x = r.X + r.W*float32(i)/float32(n-1)
	at := (v - lo) / (hi - lo)
	y = r.Y + r.H - r.H*float32(at)
	return x, y
}

// drawLine draws a series as a line, its newest value at the right edge.
func drawLine(pt *ui.Painter, r ui.Rect, values []float64, color ui.Color, lo, hi float64) {
	path := &ui.Path{}
	for i, v := range values {
		x, y := point(r, i, len(values), v, lo, hi)
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	pt.StrokePath(path, 1.5, color)
}

// drawArea draws a series as the area under its line, filled from the line
// down to the bottom of the box, with the line itself kept bright.
func drawArea(pt *ui.Painter, r ui.Rect, values []float64, color ui.Color, lo, hi float64) {
	area := &ui.Path{}
	area.MoveTo(r.X, r.Y+r.H)
	line := &ui.Path{}
	for i, v := range values {
		x, y := point(r, i, len(values), v, lo, hi)
		area.LineTo(x, y)
		if i == 0 {
			line.MoveTo(x, y)
		} else {
			line.LineTo(x, y)
		}
	}
	area.LineTo(r.X+r.W, r.Y+r.H)
	area.Close()
	pt.FillPathGradient(area, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
		{Color: color.Alpha(0.55), At: 0},
		{Color: color.Alpha(0.02), At: 1},
	}})
	pt.StrokePath(line, 1.5, color)
}

// drawBar draws how far along something is: the track it runs in, and the lit
// part of it.
func drawBar(pt *ui.Painter, r ui.Rect, track ui.Color, v float64, color ui.Color) {
	const h = 6
	y := r.Y + (r.H-h)/2
	pt.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: h}, track, h/2)
	if v <= 0 {
		return
	}
	pt.Fill(ui.Rect{X: r.X, Y: y, W: r.W * float32(clamp01(v)), H: h}, color, h/2)
}

// drawDashed draws a reading that is not changing as a dashed line: a flat
// series drawn as a solid line would look like a chart with the top cut off,
// and as nothing at all against the card's edge.
func drawDashed(pt *ui.Painter, r ui.Rect, color ui.Color) {
	const dash, gap, width = 5, 4, 1.5
	y := r.Y + r.H/2
	for x := r.X; x < r.X+r.W; x += dash + gap {
		w := min32(dash, r.X+r.W-x)
		pt.Fill(ui.Rect{X: x, Y: y - width/2, W: w, H: width}, color, width/2)
	}
}
