package panel

import "math"

// The panel is one column of a fixed shape: a title bar, a row of tabs that
// choose the monitor, the page, and a status bar at the foot.
const (
	// DesignWidth and DesignHeight are the panel's size in physical pixels,
	// the unit a design is drawn in. A window is sized in DIPs, so the panel
	// converts them with the display's scale: 660×840 is 440×560 DIPs at
	// 150%, and the same 660×840 pixels on screen.
	DesignWidth, DesignHeight = 660, 840
	// MarginPixels is the room the panel keeps from the screen edge and the
	// taskbar, also in physical pixels.
	MarginPixels = 8
)

// Metrics is the panel's size, worked out from the room it may fill.
type Metrics struct {
	// Width and Height of the window, in DIPs.
	Width, Height int
}

// Fit sizes the panel for the room it may fill, in DIPs, on a display with the
// given scale: the design's pixel size in that scale, shrunk evenly when the
// room is smaller.
//
// The size is worked out once and kept: switching tabs or reopening the panel
// does not resize it, and a page longer than it scrolls.
func Fit(availWidth, availHeight int, scale float64) Metrics {
	if scale <= 0 {
		scale = 1
	}
	width := int(math.Round(float64(DesignWidth) / scale))
	height := int(math.Round(float64(DesignHeight) / scale))

	if availHeight > 0 && height > availHeight {
		height = availHeight
		width = int(math.Round(float64(height) * DesignWidth / DesignHeight))
	}
	if availWidth > 0 && width > availWidth {
		width = availWidth
		height = int(math.Round(float64(width) * DesignHeight / DesignWidth))
	}
	return Metrics{Width: maxInt(width, 1), Height: maxInt(height, 1)}
}

// MarginDIP is the margin the panel keeps from the screen edge, in DIPs on a
// display with the given scale.
func MarginDIP(scale float64) int {
	if scale <= 0 {
		scale = 1
	}
	return maxInt(int(math.Round(float64(MarginPixels)/scale)), 1)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
