package tray

import "github.com/egoist/mygo"

// Margin is the room the panel keeps from the screen edges it docks against:
// it floats above the taskbar and off the right edge, rather than fusing with
// them.
const Margin = 8

// Inset is the work area less the margin, the room the panel may fill.
func Inset(work mygo.Rectangle, margin int) mygo.Rectangle {
	if work.Width <= 0 || work.Height <= 0 {
		return work
	}
	return mygo.Rectangle{
		X:      work.X,
		Y:      work.Y,
		Width:  maxInt(work.Width-margin, 1),
		Height: maxInt(work.Height-margin, 1),
	}
}

// Dock puts the panel in the bottom-right corner of the area it may fill:
// against the right edge of the screen and on top of the taskbar, less the
// margin the app insets the work area by.
func Dock(area mygo.Rectangle, width, height int) (int, int) {
	if area.Width <= 0 || area.Height <= 0 {
		return 0, 0
	}
	x := area.X + maxInt(0, area.Width-width)
	y := area.Y + maxInt(0, area.Height-height)
	return x, y
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
