package panel

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
)

// fixture makes a host with its built-in monitors, without polling: a machine
// that reports the parts of a computer and a mouse beside it.
func fixture(t *testing.T) (*app.Host, *App) {
	t.Helper()
	h := app.NewHost(t.TempDir())
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	h.OnChange = func() {}
	h.Inject("mouse", core.Snapshot{
		Status: core.StatusOK, Title: "Rapoo Gaming Device",
		Subtitle: "无线 · 72%", Primary: "battery",
		Readings: []core.Reading{
			{ID: "battery", Label: "电量", Value: 72, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "2 秒前"},
			{ID: "link", Label: "连接", Text: "无线", Severity: core.SeverityOK},
			{ID: "dpi", Label: "DPI", Value: 1200, Severity: core.SeverityNone},
		},
	})
	h.Inject("system", core.Snapshot{
		Status: core.StatusOK, Title: "DESKTOP-4K1", Subtitle: "Windows 11 26H2",
		Primary: "cpu", Health: 90, HealthNote: "很好",
		Readings: []core.Reading{
			{ID: "cpu", Label: "CPU", Value: 5, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "16 线程"},
			{ID: "mem", Label: "内存", Value: 46, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "6.39 GB / 13.7 GB"},
			{ID: "battery", Label: "电池", Value: 100, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "已充满"},
		},
	})
	p := New(h)
	p.SetSize(Fit(1920, 1040, 1))
	return h, p
}

// TestThePanelOpensOnTheMachine checks the panel's page is the machine's: its
// score, its cards, and the way into the settings — with no title bar and no
// tabs above them.
func TestThePanelOpensOnTheMachine(t *testing.T) {
	_, p := fixture(t)
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	for _, want := range []string{"90", "很好", "CPU", "电池", "鼠标", "打开设置", "刷新"} {
		if _, ok := tt.Find(want); !ok {
			t.Errorf("the panel does not show %q: %v", want, tt.Texts())
		}
	}
	for _, gone := range []string{"SeeAll", "本机"} {
		if tt.HasText(gone) {
			t.Errorf("the panel still draws the old title bar's %q: %v", gone, tt.Texts())
		}
	}
}

// TestTheSettingsPageStartsAtTheTop checks the settings page is drawn from the
// top, under its own heading, rather than being pushed down the window.
func TestTheSettingsPageStartsAtTheTop(t *testing.T) {
	_, p := fixture(t)
	p.SetSelected(SettingsPage)
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	back, ok := tt.Find("返回")
	if !ok {
		t.Fatalf("the settings have no way back: %v", tt.Texts())
	}
	first, ok := tt.Find("应用")
	if !ok {
		t.Fatalf("the settings page is empty: %v", tt.Texts())
	}
	if first.Y <= back.Y {
		t.Errorf("the settings content at y=%v sits above the heading at y=%v", first.Y, back.Y)
	}
	if first.Y > float32(DesignHeight)/4 {
		t.Errorf("the settings content starts at y=%v, not under the heading at y=%v",
			first.Y, back.Y)
	}

	texts := strings.Join(tt.Texts(), " ")
	if !strings.Contains(texts, "本机") {
		t.Errorf("the settings page lists no monitor: %s", texts)
	}
	// The mouse has nothing to set — its receiver is found by itself — and a
	// manual vendor number is exactly what that replaced.
	if strings.Contains(texts, "厂商号") {
		t.Errorf("the settings page still asks for a vendor number: %s", texts)
	}
}

// TestTheSettingsPageOnlyOffersSettings checks the page holds what the panel
// can change — when to start, how solid it is, what each plugin watches — and
// nothing else: monitors are not added, removed or notified about from here.
func TestTheSettingsPageOnlyOffersSettings(t *testing.T) {
	_, p := fixture(t)
	p.SetSelected(SettingsPage)
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)
	texts := strings.Join(tt.Texts(), " ")

	for _, want := range []string{"开机自启", "不透明度", "面板不透明度", "城市", "留空按 IP 定位"} {
		if !strings.Contains(texts, want) {
			t.Errorf("the settings page has no %q: %s", want, texts)
		}
	}
	for _, gone := range []string{"通知", "主题", "移除", "添加"} {
		if strings.Contains(texts, gone) {
			t.Errorf("the settings page still offers %q: %s", gone, texts)
		}
	}
}

// TestATypedSettingSitsOnItsRow checks the city's field is beside its name and
// note rather than across the whole card: every control on the page ends on the
// card's inner edge, so a typed setting reads like the choice under it instead
// of turning the page into two layouts.
func TestATypedSettingSitsOnItsRow(t *testing.T) {
	_, p := fixture(t)
	p.SetSelected(SettingsPage)
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	name, ok := tt.Find("城市")
	if !ok {
		t.Fatalf("the city setting has no name: %v", tt.Texts())
	}
	note, ok := tt.Find("留空按 IP 定位，填城市名就固定用它")
	if !ok {
		t.Fatalf("the city setting has no note: %v", tt.Texts())
	}

	// Where the field has to be: on the line the note explains, at the end of
	// the card's inner edge. Pressing there types into it only if it is there.
	x := float32(DesignWidth-Gutter-cardPadX) - 30
	y := (name.Y + note.Y + note.H) / 2
	tt.ClickAt(x, y)
	active, _ := tt.TextInputActive()
	caret, hasCaret := tt.TextCaret()
	if !active || !hasCaret {
		t.Fatalf("pressing %.0f,%.0f types into nothing: the field is not on the right of its row",
			x, y)
	}
	if caret.X < DesignWidth/2 {
		t.Errorf("the field starts at x=%v on the left of the page, not at its end", caret.X)
	}
	// The row is the two lines the name and the note take; a field above or
	// below them is a layout of its own.
	if caret.Y < name.Y || caret.Y+caret.H > note.Y+note.H {
		t.Errorf("the field at y=%v..%v is not on the row the name at y=%v..%v and the note at y=%v..%v take",
			caret.Y, caret.Y+caret.H, name.Y, name.Y+name.H, note.Y, note.Y+note.H)
	}
}
