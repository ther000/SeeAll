package tray

import "github.com/egoist/mygo"

// Inset is the work area less a margin, the room the panel may fill. The
// margin keeps the panel off the screen edge and above the taskbar, so it
// floats rather than fusing with them; it is in DIPs, as the work area is.
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
// against the right edge of the screen and on top of the taskbar, which the
// work area already leaves out.
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
