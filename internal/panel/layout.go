package panel

// Metrics is the panel's size, worked out from the work area it docks into.
type Metrics struct {
	// Width and Height of the window, in DIPs.
	Width, Height int
	// Sidebar is the width of the monitor list.
	Sidebar int
}

// Fit sizes the panel for a work area.
//
// The panel is docked against the right edge of the screen, on top of the
// taskbar, and it is a dashboard rather than a popover: wide enough for two
// columns of readings on a desktop display, narrower on a laptop.
func Fit(workWidth, workHeight int) Metrics {
	const (
		minWidth, maxWidth   = 560, 920
		minHeight, maxHeight = 560, 680
	)
	m := Metrics{
		Width:   clampInt(workWidth*42/100, minWidth, maxWidth),
		Height:  clampInt(workHeight*58/100, minHeight, maxHeight),
		Sidebar: 184,
	}
	if m.Width > workWidth {
		m.Width = workWidth
	}
	if m.Height > workHeight {
		m.Height = workHeight
	}
	// A narrow panel gives up some of the list before the page.
	if m.Width < 660 {
		m.Sidebar = clampInt(m.Width*31/100, 128, 184)
	}
	return m
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
