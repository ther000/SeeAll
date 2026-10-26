package tray

import (
	"testing"

	"github.com/egoist/mygo"
)

func rect(x, y, w, h int) mygo.Rectangle {
	return mygo.Rectangle{X: x, Y: y, Width: w, Height: h}
}

// TestInsetKeepsThePanelOffTheEdges checks the room the panel may fill is the
// work area less the margin: the panel floats above the taskbar and off the
// right edge instead of fusing with them.
func TestInsetKeepsThePanelOffTheEdges(t *testing.T) {
	work := rect(0, 0, 2560, 1368)
	area := Inset(work, 8)
	if area.Width != 2552 || area.Height != 1360 || area.X != 0 || area.Y != 0 {
		t.Errorf("Inset = %+v, want 2552×1360 at (0, 0)", area)
	}
}

// TestInsetWithoutWorkArea leaves an unknown display alone, rather than making
// a negative area out of it.
func TestInsetWithoutWorkArea(t *testing.T) {
	if got := Inset(rect(0, 0, 0, 0), 8); got.Width != 0 || got.Height != 0 {
		t.Errorf("Inset of an unknown work area = %+v, want it unchanged", got)
	}
	if got := Inset(rect(0, 0, 4, 4), 8); got.Width < 1 || got.Height < 1 {
		t.Errorf("Inset smaller than the margin = %+v, want at least one pixel", got)
	}
}

// TestDockPutsThePanelInTheCorner checks the panel ends up against the right
// edge of the area it may fill and at its bottom.
func TestDockPutsThePanelInTheCorner(t *testing.T) {
	// A 2560×1440 display with a 72-pixel taskbar at 96 DPI, less the margin.
	area := rect(0, 0, 2552, 1360)
	x, y := Dock(area, 660, 840)
	if x != 2552-660 || y != 1360-840 {
		t.Errorf("Dock = (%d, %d), want (%d, %d)", x, y, 2552-660, 1360-840)
	}

	// A second display to the right docks against its own right edge.
	second := rect(2560, 0, 1912, 1032)
	x, y = Dock(second, 660, 840)
	if x != 2560+1912-660 || y != 1032-840 {
		t.Errorf("Dock on the second display = (%d, %d)", x, y)
	}
}

// TestDockNeverLeavesTheArea keeps a panel too big for its display inside it,
// at the top-left corner rather than off screen.
func TestDockNeverLeavesTheArea(t *testing.T) {
	area := rect(0, 0, 400, 300)
	x, y := Dock(area, 660, 840)
	if x != 0 || y != 0 {
		t.Errorf("Dock of an oversized panel = (%d, %d), want (0, 0)", x, y)
	}
}

// TestDockWithoutArea leaves the panel where it is when the display is
// unknown.
func TestDockWithoutArea(t *testing.T) {
	if x, y := Dock(rect(0, 0, 0, 0), 660, 840); x != 0 || y != 0 {
		t.Errorf("Dock without an area = (%d, %d), want (0, 0)", x, y)
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
