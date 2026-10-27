package panel

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
)

// TestCardOrderSurvivesARestart checks the order a user drags cards into is
// kept in the settings file: a page that forgets its arrangement at the next
// launch is not arrangeable.
func TestCardOrderSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	h := app.NewHost(dir)
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	h.SetCardOrder("system", []string{"gpu", "cpu", "mem"})

	again := app.NewHost(dir)
	if err := again.Load(); err != nil {
		t.Fatal(err)
	}
	again.Stop()
	if got := again.CardOrder("system"); !reflect.DeepEqual(got, []string{"gpu", "cpu", "mem"}) {
		t.Errorf("after a restart the order is %v, want [gpu cpu mem]", got)
	}
}

// TestDroppingACardKeepsTheOtherPagesAlone checks the arranged order belongs to
// the instance it was dragged on: the mouse page must not follow the system
// page's arrangement.
func TestDroppingACardKeepsTheOtherPagesAlone(t *testing.T) {
	h, p := fixture(t)
	p.SetSize(Fit(1920, 1040, 1))
	h.SetCardOrder("system", []string{"gpu", "cpu"})
	p.SetSelected("mouse")
	tt := ui.NewTester(func(c *ui.Context) { p.View(c) }, DesignWidth, DesignHeight)
	if _, ok := tt.Find("Rapoo Gaming Device"); !ok {
		t.Fatalf("the mouse page is not the one shown: %v", tt.Texts())
	}
	if got := h.CardOrder("mouse"); len(got) != 0 {
		t.Errorf("the mouse page took the system page's order: %v", got)
	}
}
