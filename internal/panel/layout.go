package panel

// The panel is one column at a fixed aspect ratio: a title bar, a row of tabs
// that choose the monitor, the page, and a status bar at the foot.
const (
	// DesignWidth and DesignHeight are the panel's proportions, not a fixed
	// size: a display too small for the design gets a smaller panel of the
	// same shape.
	DesignWidth, DesignHeight = 660, 840
)

// Metrics is the panel's size, worked out from the room it may fill.
type Metrics struct {
	// Width and Height of the window, in DIPs.
	Width, Height int
}

// Fit sizes the panel for the room it may fill: the largest window of the
// design's proportions that fits, so the layout is the same shape on every
// display and only its scale changes.
//
// The size is worked out once and kept: switching tabs or reopening the panel
// does not resize it, and a page longer than it scrolls.
func Fit(availWidth, availHeight int) Metrics {
	width, height := DesignWidth, DesignHeight
	if availHeight > 0 && height > availHeight {
		height = availHeight
		width = height * DesignWidth / DesignHeight
	}
	if availWidth > 0 && width > availWidth {
		width = availWidth
		height = width * DesignHeight / DesignWidth
	}
	return Metrics{Width: maxInt(width, 1), Height: maxInt(height, 1)}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
