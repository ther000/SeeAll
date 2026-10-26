package panel

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"

	"seeall/internal/core"
)

// TestDraggingACardReordersThePage drags a card over another and checks the
// page takes the new order, and that it survives into what the host remembers:
// a reorder the user cannot keep is not a reorder.
func TestDraggingACardReordersThePage(t *testing.T) {
	h, p := fixture(t)
	h.Inject("system", core.Snapshot{Status: core.StatusOK, Title: "DESKTOP",
		Primary: "cpu", Readings: []core.Reading{
			{ID: "cpu", Label: "CPU", Value: 23.4, Unit: core.UnitPercent, Severity: core.SeverityOK},
			{ID: "mem", Label: "内存", Value: 61, Unit: core.UnitPercent, Severity: core.SeverityOK},
			{ID: "disk", Label: "磁盘 C:", Value: 78, Unit: core.UnitPercent, Severity: core.SeverityWarn},
		}})
	p.SetSize(Fit(1920, 1040, 1))
	p.SetSelected("system")
	tt := ui.NewTester(func(c *ui.Context) { p.View(c) }, DesignWidth, DesignHeight)

	cpu, ok := tt.Find("CPU")
	if !ok {
		t.Fatalf("the page shows no CPU card: %v", tt.Texts())
	}
	disk, ok := tt.Find("磁盘 C:")
	if !ok {
		t.Fatalf("the page shows no disk card: %v", tt.Texts())
	}

	// CPU is the first card; drag it onto the disk card's place.
	dragCard(tt, cpu, disk)

	// The page now leads with the disk card and the CPU card is where the
	// disk card was.
	again := ui.NewTester(func(c *ui.Context) { p.View(c) }, DesignWidth, DesignHeight)
	disk2, ok := again.Find("磁盘 C:")
	if !ok {
		t.Fatalf("the page lost the disk card: %v", again.Texts())
	}
	cpu2, ok := again.Find("CPU")
	if !ok {
		t.Fatalf("the page lost the CPU card: %v", again.Texts())
	}
	if disk2.Y > cpu2.Y || (disk2.Y == cpu2.Y && disk2.X > cpu2.X) {
		t.Errorf("磁盘 is at (%v, %v) and CPU at (%v, %v): the drag did not reorder them",
			disk2.X, disk2.Y, cpu2.X, cpu2.Y)
	}

	// The pointer landed on the disk card's centre, which is the right half of
	// its cell: the CPU card goes after it. [mem disk cpu] would be the same
	// move read the other way round, and is not what a user dragging onto the
	// middle of a card sees.
	want := []string{"mem", "disk", "cpu"}
	if got := h.CardOrder("system"); !reflect.DeepEqual(got, want) {
		t.Errorf("the host remembers %v, want %v", got, want)
	}
}

// dragCard presses a card, moves the pointer onto another's centre and lets go,
// as a user does.
func dragCard(tt *ui.Tester, from, to ui.Rect) {
	fromX, fromY := from.X+from.W/2, from.Y+from.H/2
	toX, toY := to.X+to.W/2, to.Y+to.H/2
	tt.Press(fromX, fromY)
	for k := 1; k <= 4; k++ {
		f := float32(k) / 4
		tt.Move(fromX+(toX-fromX)*f, fromY+(toY-fromY)*f)
	}
	tt.Release(toX, toY)
}
