package panel

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
)

// TestOpacitySliderFadesTheWindowAsItIsDragged checks the settings page's
// transparency slider: dragging it down asks the window to fade on the way,
// and the value is written to the settings file only once the pointer is let
// go — a settings file rewritten sixty times a second is a disk problem, not a
// feature.
func TestOpacitySliderFadesTheWindowAsItIsDragged(t *testing.T) {
	h, p := fixture(t)
	if got := p.Opacity; got != 1 {
		t.Fatalf("a fresh panel is %v solid, want 1", got)
	}
	var asked []float64
	p.OnOpacity = func(v float64) { asked = append(asked, v) }
	p.SetSelected(SettingsPage)
	p.SetSize(Fit(1920, 1040, 1))
	tt := ui.NewTester(func(c *ui.Context) { p.View(c) }, DesignWidth, DesignHeight)

	slider, ok := tt.Find("面板不透明度")
	if !ok {
		t.Fatalf("the settings page has no transparency slider: %v", tt.Texts())
	}
	// Drag the knob to about half way.
	fromX := slider.X + slider.W - 6
	toX := slider.X + slider.W*0.5
	mid := slider.Y + slider.H/2
	tt.Press(fromX, mid)
	tt.Move(fromX+(toX-fromX)*0.5, mid)
	tt.Move(toX, mid)
	if len(asked) == 0 {
		t.Fatal("dragging the slider did not ask the window to fade")
	}
	if p.Opacity >= 1 || p.Opacity < app.MinPanelOpacity {
		t.Errorf("the panel is %v solid mid drag, want between %v and 1", p.Opacity, app.MinPanelOpacity)
	}
	if got := h.PanelOpacity(); got != 1 {
		t.Errorf("the settings file was written mid drag (%v), want it left alone", got)
	}
	dragged := p.Opacity

	tt.Release(toX, mid)
	if got := h.PanelOpacity(); got != dragged {
		t.Errorf("after letting go the file holds %v, want %v", got, dragged)
	}
	// The next frame shows the value it settled on.
	tt = ui.NewTester(func(c *ui.Context) { p.View(c) }, DesignWidth, DesignHeight)
	if want := fmt.Sprintf("%d%%", int(dragged*100+0.5)); !tt.HasText(want) {
		t.Errorf("the row does not say %q: %v", want, tt.Texts())
	}
}

// TestThePanelCannotGoInvisible checks the slider's floor: a panel faded to
// nothing can be neither read nor found again, since it has no window frame to
// grab. The floor holds even against a saved value from an older file.
func TestThePanelCannotGoInvisible(t *testing.T) {
	if got := app.ClampOpacityForTest(0.01); got != app.MinPanelOpacity {
		t.Errorf("a barely visible panel is clamped to %v, want %v", got, app.MinPanelOpacity)
	}
	if got := app.ClampOpacityForTest(2); got != 1 {
		t.Errorf("an over solid panel is clamped to %v, want 1", got)
	}

	h, p := fixture(t)
	slider := func(tt *ui.Tester) ui.Rect {
		r, ok := tt.Find("面板不透明度")
		if !ok {
			t.Fatalf("no slider: %v", tt.Texts())
		}
		return r
	}
	p.SetSelected(SettingsPage)
	p.SetSize(Fit(1920, 1040, 1))
	tt := ui.NewTester(func(c *ui.Context) { p.View(c) }, DesignWidth, DesignHeight)
	r := slider(tt)
	mid := r.Y + r.H/2
	// Drag past the left end of the slider.
	tt.Press(r.X+r.W-6, mid)
	tt.Move(r.X+r.W/2, mid)
	tt.Move(r.X-40, mid)
	tt.Release(r.X-40, mid)
	if p.Opacity < app.MinPanelOpacity {
		t.Errorf("the panel faded to %v, below the floor %v", p.Opacity, app.MinPanelOpacity)
	}
	if got := h.PanelOpacity(); got < app.MinPanelOpacity {
		t.Errorf("the settings file holds %v, below the floor %v", got, app.MinPanelOpacity)
	}
}

// TestOpacitySurvivesARestart checks the transparency is kept in the settings
// file: a panel that forgets how faint it was at the next launch is one the
// user has to set every time.
func TestOpacitySurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	h := app.NewHost(dir)
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	if got := h.PanelOpacity(); got != 1 {
		t.Fatalf("a panel that never set it is %v solid, want 1", got)
	}
	h.SetPanelOpacity(0.7)

	again := app.NewHost(dir)
	if err := again.Load(); err != nil {
		t.Fatal(err)
	}
	again.Stop()
	if got := again.PanelOpacity(); got != 0.7 {
		t.Errorf("after a restart the panel is %v solid, want 0.7", got)
	}

	// The panel takes the value from the host once the file is read: the
	// window follows the panel, so a panel that kept its default would open
	// solid however faint the file says it should be.
	var applied []float64
	again2 := app.NewHost(dir)
	if err := again2.Load(); err != nil {
		t.Fatal(err)
	}
	again2.Stop()
	p := New(again2)
	p.OnOpacity = func(v float64) { applied = append(applied, v) }
	p.SyncOpacity()
	if p.Opacity != 0.7 {
		t.Errorf("the panel is %v solid after reading the file, want 0.7", p.Opacity)
	}
	if len(applied) == 0 || applied[len(applied)-1] != 0.7 {
		t.Errorf("the window was told %v, want 0.7", applied)
	}

	// A file from before the slider existed has no value, and means opaque.
	h2 := app.NewHost(t.TempDir())
	if err := h2.Load(); err != nil {
		t.Fatal(err)
	}
	h2.Stop()
	if got := h2.PanelOpacity(); got != 1 {
		t.Errorf("a settings file with no opacity gives %v, want 1", got)
	}
}
