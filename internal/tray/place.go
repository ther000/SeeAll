package tray

import "github.com/egoist/mygo"

// Edge is the room the panel keeps from the screen edges it does not sit on.
// It sits flush against the right edge and on top of the taskbar, as the
// system's own flyouts do.
const Edge = 0

// Dock puts the panel in the bottom-right corner of the work area: against the
// right edge of the screen and just above the taskbar, which the work area
// already leaves out.
func Dock(work mygo.Rectangle, width, height int) (int, int) {
	if work.Width <= 0 || work.Height <= 0 {
		return 0, 0
	}
	x := work.X + maxInt(Edge, work.Width-width-Edge)
	y := work.Y + maxInt(Edge, work.Height-height-Edge)
	return x, y
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
