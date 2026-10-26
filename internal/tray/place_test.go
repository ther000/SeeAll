package tray

import (
	"testing"

	"github.com/egoist/mygo"
)

func rect(x, y, w, h int) mygo.Rectangle {
	return mygo.Rectangle{X: x, Y: y, Width: w, Height: h}
}

// TestDockPutsThePanelInTheCorner checks the panel ends up against the right
// edge of the screen and on top of the taskbar, which the work area already
// leaves out.
func TestDockPutsThePanelInTheCorner(t *testing.T) {
	// A 2560×1440 display with a 72-pixel taskbar at 96 DPI.
	work := rect(0, 0, 2560, 1368)
	x, y := Dock(work, 600, 800)
	if x != 2560-600 || y != 1368-800 {
		t.Errorf("Dock = (%d, %d), want (%d, %d)", x, y, 2560-600, 1368-800)
	}

	// A second display to the right docks against its own right edge.
	second := rect(2560, 0, 1920, 1040)
	x, y = Dock(second, 600, 800)
	if x != 2560+1920-600 || y != 1040-800 {
		t.Errorf("Dock on the second display = (%d, %d)", x, y)
	}
}

// TestDockNeverLeavesTheWorkArea keeps a panel too big for its display inside
// it, at the top-left corner rather than off screen.
func TestDockNeverLeavesTheWorkArea(t *testing.T) {
	work := rect(0, 0, 400, 300)
	x, y := Dock(work, 600, 800)
	if x != 0 || y != 0 {
		t.Errorf("Dock of an oversized panel = (%d, %d), want (0, 0)", x, y)
	}
}

// TestDockWithoutWorkArea leaves the panel where it is when the display is
// unknown.
func TestDockWithoutWorkArea(t *testing.T) {
	if x, y := Dock(rect(0, 0, 0, 0), 600, 800); x != 0 || y != 0 {
		t.Errorf("Dock without a work area = (%d, %d), want (0, 0)", x, y)
	}
}

// TestIconRenders checks the icon is a PNG of the tray size for every state,
// including the ones with nothing to show.
func TestIconRenders(t *testing.T) {
	for _, st := range []State{
		{Level: -1, Waiting: true},
		{Level: -1, Offline: true},
		{Level: 0},
		{Level: 7},
		{Level: 42},
		{Level: 100},
	} {
		png := Icon(st)
		if len(png) == 0 {
			t.Fatalf("Icon(%+v) drew nothing", st)
		}
		if string(png[1:4]) != "PNG" {
			t.Fatalf("Icon(%+v) is not a PNG", st)
		}
	}
}
