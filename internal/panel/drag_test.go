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
			{ID: "gpu", Label: "GPU", Value: 78, Unit: core.UnitPercent, Severity: core.SeverityWarn},
		}})
	p.SetSize(Fit(1920, 1040, 1))
	p.SetSelected("system")
	tt := ui.NewTester(func(c *ui.Context) { p.View(c) }, DesignWidth, DesignHeight)

	cpu, ok := tt.Find("CPU")
	if !ok {
		t.Fatalf("the page shows no CPU card: %v", tt.Texts())
	}
	gpu, ok := tt.Find("GPU")
	if !ok {
		t.Fatalf("the page shows no GPU card: %v", tt.Texts())
	}

	// CPU is the first card; drag it onto the GPU card's place.
	dragCard(tt, cpu, gpu)

	// The page now leads with the GPU card and the CPU card is where the GPU
	// card was.
	again := ui.NewTester(func(c *ui.Context) { p.View(c) }, DesignWidth, DesignHeight)
	gpu2, ok := again.Find("GPU")
	if !ok {
		t.Fatalf("the page lost the GPU card: %v", again.Texts())
	}
	cpu2, ok := again.Find("CPU")
	if !ok {
		t.Fatalf("the page lost the CPU card: %v", again.Texts())
	}
	if gpu2.Y > cpu2.Y || (gpu2.Y == cpu2.Y && gpu2.X > cpu2.X) {
		t.Errorf("GPU is at (%v, %v) and CPU at (%v, %v): the drag did not reorder them",
			gpu2.X, gpu2.Y, cpu2.X, cpu2.Y)
	}

	// The pointer landed on the GPU card's centre, which is the right half of
	// its cell: the CPU card goes after it. [mem gpu cpu] would be the same
	// move read the other way round, and is not what a user dragging onto the
	// middle of a card sees.
	want := []string{"mem", "gpu", "cpu"}
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
