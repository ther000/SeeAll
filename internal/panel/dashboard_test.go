package panel

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
)

// dashFixture is the panel the dashboard is drawn for: a machine that scores
// itself, names its parts and lists what it runs, and a mouse beside it. It is
// the data the plugins report, injected, so that the tests of the dashboard's
// layout do not wait for a machine or a device.
func dashFixture(t *testing.T) (*app.Host, *App) {
	t.Helper()
	h := app.NewHost(t.TempDir())
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	h.OnChange = func() {}
	h.SetTheme(app.ThemeDashboard)
	h.ClearHistory()

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
		Status: core.StatusOK, Title: "DESKTOP-4K1", Subtitle: "Windows 11 26H2", Primary: "cpu",
		Health: 90, HealthNote: "很好",
		Facts: []core.Fact{
			{Value: "Ryzen 7 6800H"},
			{Value: "13.7 GB"},
			{Value: "Windows 11 26H2"},
			{Label: "已运行", Value: "2天2小时"},
		},
		Processes: []core.Process{
			{Name: "QQ", Mem: 391 * 1024 * 1024},
			{Name: "DeepSeek Harness", Mem: 254 * 1024 * 1024},
			{Name: "Windows 服务主进程", Mem: 222 * 1024 * 1024, CPU: 3.3},
			{Name: "Everything", Mem: 143 * 1024 * 1024},
			{Name: "Quicker", Mem: 128 * 1024 * 1024},
		},
		Readings: []core.Reading{
			{ID: "cpu", Label: "CPU", Value: 5, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Badge: "2.24 GHz", Detail: "16 线程"},
			{ID: "gpu", Label: "GPU", Value: 4, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Badge: "50°C", Detail: "GeForce RTX 3060"},
			{ID: "mem", Label: "内存", Value: 46, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Badge: "压力正常", Detail: "6.39 GB / 13.7 GB"},
			{ID: "disk", Label: "可用空间", Value: 63 * 1024 * 1024 * 1024, Unit: core.UnitBytes,
				Severity: core.SeverityOK, Badge: "C: · 159 GB", Detail: "已用 96 GB · 60%"},
			{ID: "net", Label: "网络", Text: "<1 KB/s",
				Severity: core.SeverityOK, Badge: "Wi-Fi", Detail: "上传 <1 KB/s"},
			{ID: "temp", Label: "温度", Value: 54, Text: "54°C",
				Severity: core.SeverityOK, Badge: "正常", Detail: "系统 54° · GPU 50°"},
			{ID: "vram", Label: "显存", Value: 36, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "2.1 GB / 5.9 GB"},
			{ID: "battery", Label: "电池", Value: 100, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Badge: "健康 78%", Detail: "已充满"},
			{ID: "uptime", Label: "开机", Text: "2天2小时", Severity: core.SeverityNone},
		},
	})
	h.InjectHistory("system", "cpu", dashWave(5, 40, 0))
	h.InjectHistory("system", "gpu", dashWave(4, 40, 3))
	h.InjectHistory("system", "mem", dashWave(46, 40, 3))
	h.InjectHistory("mouse", "battery", dashWave(72, 40, 6))

	p := New(h)
	p.SetSize(Fit(1920, 1040, 1))
	p.SetSelected("system")
	return h, p
}

// dashWave is a short history used to give a card a chart to draw.
func dashWave(v float64, count int, wobble float64) []float64 {
	out := make([]float64, count)
	for i := range out {
		out[i] = v + wobble*float64(i%3)/2
	}
	return out
}

// TestDashboardShowsTheMachineOnOnePage checks the dashboard's parts are where
// it says they are: the score and the facts at the top, the six cards in the
// order of the machine, the mouse's card under them, the battery, the programs
// and the machine's own foot — all of them inside the window, with nothing
// hung off its edge and nothing of the old foot bar left.
func TestDashboardShowsTheMachineOnOnePage(t *testing.T) {
	_, p := dashFixture(t)
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	inWindow := func(name string) ui.Rect {
		t.Helper()
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("the dashboard does not show %q: %v", name, tt.Texts())
		}
		if r.X < 0 || r.Y < 0 || r.X+r.W > DesignWidth || r.Y+r.H > DesignHeight {
			t.Fatalf("%q is at %+v, outside the %d×%d panel", name, r, DesignWidth, DesignHeight)
		}
		return r
	}

	score := inWindow("90")
	note := inWindow("很好")
	if note.Y < score.Y || note.Y+note.H > score.Y+score.H+8 {
		t.Errorf("the note at y=%v does not sit on the score's line at y=%v", note.Y, score.Y)
	}
	fact := inWindow("Ryzen 7 6800H")
	if fact.Y <= score.Y {
		t.Errorf("the facts at y=%v sit above the score at y=%v", fact.Y, score.Y)
	}

	// The six parts of the machine, two to a row, in the dashboard's order.
	for _, row := range [][2]string{{"CPU", "GPU"}, {"内存", "可用空间"}, {"网络", "温度"}} {
		left, right := inWindow(row[0]), inWindow(row[1])
		if left.Y != right.Y {
			t.Errorf("%q and %q are not on one row: %v and %v", row[0], row[1], left.Y, right.Y)
		}
		if right.X <= left.X {
			t.Errorf("%q is not to the right of %q", row[1], row[0])
		}
	}

	// The mouse is not a tab and not another page: its card follows the
	// machine's own readings and comes before the machine's battery.
	mouse := inWindow("鼠标")
	if mouse.Y <= inWindow("温度").Y {
		t.Errorf("the mouse's card at y=%v sits above the machine's last cards at y=%v",
			mouse.Y, inWindow("温度").Y)
	}
	if inWindow("电池").Y <= mouse.Y {
		t.Errorf("the battery at y=%v sits above the mouse's card at y=%v",
			inWindow("电池").Y, mouse.Y)
	}
	if inWindow("高占用进程").Y <= inWindow("电池").Y {
		t.Error("the programs sit no lower than the battery")
	}

	// The foot of the panel is the content's own edge: the bar that counted
	// the window's monitors is gone.
	for _, absent := range []string{"异常项", "监控项"} {
		if tt.HasText(absent) {
			t.Errorf("the page still shows the foot bar's %q: %v", absent, tt.Texts())
		}
	}
	if last := tt.Texts()[len(tt.Texts())-1]; last == "" {
		t.Error("the page ends with an empty label")
	}
}

// TestDashboardShowsTheMouseOnTheSamePage checks the mouse's card says what a
// device card says: whose it is, how it is connected, its level, and the rest
// of what it reports.
func TestDashboardShowsTheMouseOnTheSamePage(t *testing.T) {
	_, p := dashFixture(t)
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	if !tt.HasText("鼠标") {
		t.Fatalf("the machine's page does not name the mouse: %v", tt.Texts())
	}
	if !tt.HasText("72") {
		t.Errorf("the mouse's level is not on the page: %v", tt.Texts())
	}
	if !tt.HasText("无线") {
		t.Errorf("the mouse's card does not say how it is connected: %v", tt.Texts())
	}
	if !tt.HasText("DPI 1200") {
		t.Errorf("the mouse's card does not say the rest of what it reports: %v", tt.Texts())
	}
}

// TestADeviceWithoutDataKeepsItsCard checks a device that reports nothing is
// still on the page, saying so: a card that disappears is a device the user
// forgets the panel watches.
func TestADeviceWithoutDataKeepsItsCard(t *testing.T) {
	h, p := dashFixture(t)
	h.Inject("mouse", core.Snapshot{Status: core.StatusOffline})
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	if !tt.HasText("鼠标") {
		t.Fatalf("a device that reports nothing lost its card: %v", tt.Texts())
	}
	if !tt.HasText("未连接") {
		t.Errorf("the card does not say the device is not answering: %v", tt.Texts())
	}
	if !tt.HasText("—") {
		t.Errorf("the card shows no level at all: %v", tt.Texts())
	}
}

// TestDashboardKeepsItsShapeWhenNarrow checks the page is laid out from the
// window's width rather than from the design's: the panel on a 150% display
// is narrower than the design, and a card that kept the design's width would
// hang off its edge.
func TestDashboardKeepsItsShapeWhenNarrow(t *testing.T) {
	for _, size := range []Metrics{
		Fit(1920, 1040, 1),  // 660×840, the design's own size
		Fit(1706, 912, 1.5), // 440×560, what a 150% display gives it
		Fit(400, 400, 1),    // a small display, where the panel shrinks
	} {
		_, p := dashFixture(t)
		p.SetSize(size)
		tt := ui.NewTester(p.View, size.Width, size.Height)
		for _, s := range tt.Texts() {
			r, _ := tt.Find(s)
			if r.X < -0.5 || r.X+r.W > float32(size.Width)+0.5 {
				t.Errorf("at %d×%d, %q spans %v..%v, outside the window",
					size.Width, size.Height, s, r.X, r.X+r.W)
			}
		}
	}
}

// TestDashboardDegradesWithoutData checks a machine that reports less is drawn
// with less: no score means the page leads with the machine's name, no
// programs means no list, and no battery means no card — rather than an empty
// box where each would have been.
func TestDashboardDegradesWithoutData(t *testing.T) {
	h, p := dashFixture(t)
	h.Inject("system", core.Snapshot{
		Status: core.StatusOK, Title: "旧电脑", Subtitle: "Windows 10",
		Readings: []core.Reading{
			{ID: "cpu", Label: "CPU", Value: 71, Unit: core.UnitPercent,
				Severity: core.SeverityWarn, Detail: "8 线程"},
		},
	})
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)
	texts := strings.Join(tt.Texts(), " ")

	if !tt.HasText("旧电脑") {
		t.Errorf("a page without a score does not name the machine: %s", texts)
	}
	if !tt.HasText("CPU") {
		t.Errorf("the one reading it reports is not shown: %s", texts)
	}
	for _, absent := range []string{"高占用进程", "电池", "显存", "开机"} {
		if tt.HasText(absent) {
			t.Errorf("the page shows %q although the machine reports no such thing: %s", absent, texts)
		}
	}
	if !tt.HasText("高负载") {
		t.Errorf("a busy processor is not called one: %s", texts)
	}
}

// TestTheGearOpensTheSettingsAndTheBackReturns checks the dashboard has two
// pages and one way between them: the header's gear opens the settings, and
// the settings' back returns to the machine's page.
func TestTheGearOpensTheSettingsAndTheBackReturns(t *testing.T) {
	h, p := dashFixture(t)
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)
	if err := tt.Click("打开设置"); err != nil {
		t.Fatalf("the header offers no settings: %v", tt.Texts())
	}
	if p.Selected() != SettingsPage {
		t.Fatalf("the gear showed %q, want the settings", p.Selected())
	}

	tt = ui.NewTester(p.View, DesignWidth, DesignHeight)
	if _, ok := tt.Find("返回"); !ok {
		t.Fatalf("the settings have no way back: %v", tt.Texts())
	}
	if !tt.HasText("主题") {
		t.Errorf("the settings page lost its own rows: %v", tt.Texts())
	}
	if err := tt.Click("返回"); err != nil {
		t.Fatal(err)
	}
	if p.Selected() == SettingsPage {
		t.Fatal("back stayed in the settings")
	}
	if got := p.Selected(); got != h.LastInstance() {
		t.Errorf("back chose %q, want the monitor the panel was on, %q", got, h.LastInstance())
	}
	tt = ui.NewTester(p.View, DesignWidth, DesignHeight)
	if !tt.HasText("高占用进程") {
		t.Errorf("back did not return to the machine's page: %v", tt.Texts())
	}
}

// TestTheThemeIsThePanelsOwn checks the two looks are one setting: switching
// it changes what the frame draws at once, and the host keeps the choice.
func TestTheThemeIsThePanelsOwn(t *testing.T) {
	h, p := fixture(t) // the classic look
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)
	if _, ok := tt.Find("本机"); !ok {
		t.Fatalf("the classic look has no tab for the machine: %v", tt.Texts())
	}

	h.SetTheme(app.ThemeDashboard)
	tt = ui.NewTester(p.View, DesignWidth, DesignHeight)
	if _, ok := tt.Find("本机"); ok {
		t.Errorf("the dashboard still draws the classic's tabs: %v", tt.Texts())
	}
	if _, ok := tt.Find("打开设置"); !ok {
		t.Errorf("the dashboard draws no way into the settings: %v", tt.Texts())
	}

	h.SetTheme(app.ThemeClassic)
	tt = ui.NewTester(p.View, DesignWidth, DesignHeight)
	if _, ok := tt.Find("本机"); !ok {
		t.Errorf("switching back does not bring the tabs back: %v", tt.Texts())
	}
}

// TestTheSettingsChooseTheLook checks the settings page's theme row: it is
// the way a user picks the look, so the choice must come from the row and not
// only from the settings file.
func TestTheSettingsChooseTheLook(t *testing.T) {
	h, p := fixture(t) // the classic look
	p.SetSelected(SettingsPage)
	tt := ui.NewTester(func(c *ui.Context) { p.View(c) }, DesignWidth, DesignHeight)

	if _, ok := tt.Find("主题"); !ok {
		t.Fatalf("the settings page offers no theme row: %v", tt.Texts())
	}
	// The row's trigger shows the look in use ("经典"); clicking it opens the
	// two the user can pick.
	if err := tt.Click("经典"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("深蓝仪表盘"); err != nil {
		t.Fatalf("the theme row offers no dashboard: %v", tt.Texts())
	}
	if got := h.Theme(); got != app.ThemeDashboard {
		t.Fatalf("after choosing the dashboard the settings hold %q", got)
	}

	// The frame after the choice is the dashboard's, and it stays on the
	// settings page: choosing a look is not choosing a monitor. The settings
	// list every monitor by name in both looks, so the way back is what says
	// the page is the dashboard's.
	tt = ui.NewTester(func(c *ui.Context) { p.View(c) }, DesignWidth, DesignHeight)
	if _, ok := tt.Find("返回"); !ok {
		t.Errorf("the settings page is not the dashboard's: %v", tt.Texts())
	}
	if p.Selected() != SettingsPage {
		t.Errorf("choosing a look switched the page to %q", p.Selected())
	}
	if !tt.HasText("主题") {
		t.Errorf("the dashboard's settings page lost the theme row: %v", tt.Texts())
	}
}
