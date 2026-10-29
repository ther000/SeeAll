package panel

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
)

// dashFixture is the panel the dashboard is drawn for: a machine that scores
// itself, names its parts and lists what it runs, and a mouse beside it.
func dashFixture(t *testing.T) (*app.Host, *App) {
	t.Helper()
	h := app.NewHost(t.TempDir())
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	h.OnChange = func() {}
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
				Severity: core.SeverityOK, Badge: "RTX 3060", Detail: "50°C · 2.1 / 5.9 GB"},
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
	h.InjectHistory("system", "cpu", flat(5, 40))
	h.InjectHistory("system", "gpu", wave(4, 40, 3))
	h.InjectHistory("system", "mem", wave(46, 40, 3))
	h.InjectHistory("mouse", "battery", wave(72, 40, 6))

	p := New(h)
	p.SetSize(designSize())
	p.SetSelected("system")
	return h, p
}

// designSize is the panel at the design's own size: 660×1110 DIPs, which is
// what Fit gives a display with room to spare at 100%.
func designSize() Metrics { return Fit(4000, 4000, 1) }

// flat and wave make histories for the charts.
func flat(v float64, count int) []float64 { return wave(v, count, 0) }

func wave(v float64, count int, wobble float64) []float64 {
	out := make([]float64, count)
	for i := range out {
		out[i] = v + wobble*float64(i%3)/2
	}
	return out
}

// TestTheDashboardIsOneScreen checks the page is one window and not more: at
// the design's size and at the 440×740 DIPs a 150% display gives the panel,
// every part of it — the score, the four rows of cards, the last program — is
// inside the window, so nothing has to be scrolled to.
func TestTheDashboardIsOneScreen(t *testing.T) {
	for _, size := range []Metrics{
		designSize(),        // 660×1110
		Fit(1706, 912, 1.5), // 440×740, a 150% display
		Fit(1920, 1040, 1),  // 618×1040, a 100% display whose work area is short
	} {
		_, p := dashFixture(t)
		p.SetSize(size)
		tt := ui.NewTester(p.View, size.Width, size.Height)

		last := float32(0)
		for _, s := range tt.Texts() {
			r, _ := tt.Find(s)
			if r.X < -0.5 || r.X+r.W > float32(size.Width)+0.5 ||
				r.Y < -0.5 || r.Y+r.H > float32(size.Height)+0.5 {
				t.Errorf("at %d×%d, %q is at %+v, outside the panel",
					size.Width, size.Height, s, r)
			}
			last = max(last, r.Y+r.H)
		}
		// The last line of the page has room under it rather than running
		// into the window's edge: the page fits, it is not being cut.
		if last > float32(size.Height)-4 {
			t.Errorf("at %d×%d the page ends at y=%v, at the window's edge",
				size.Width, size.Height, last)
		}
	}
}

// TestEveryCardIsTheSameSize checks the battery and the mouse are cards of the
// machine's kind rather than strips across the page: one row, two columns, the
// same top as each other and the number a row under the title, as on the
// machine's own cards.
func TestEveryCardIsTheSameSize(t *testing.T) {
	_, p := dashFixture(t)
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	// Two columns: the left one starts at the gutter, the right one a gutter
	// and a card and a gap across.
	battery, ok := tt.Find("电池")
	if !ok {
		t.Fatalf("the battery has no card: %v", tt.Texts())
	}
	mouse, ok := tt.Find("鼠标")
	if !ok {
		t.Fatalf("the mouse has no card: %v", tt.Texts())
	}
	if battery.Y != mouse.Y {
		t.Errorf("the battery and the mouse are not on one row: y=%v and y=%v",
			battery.Y, mouse.Y)
	}
	if mouse.X <= battery.X {
		t.Errorf("the mouse at x=%v is not to the right of the battery at x=%v",
			mouse.X, battery.X)
	}
	cpu, _ := tt.Find("CPU")
	gpu, _ := tt.Find("GPU")
	if got, want := mouse.X-battery.X, gpu.X-cpu.X; got != want {
		t.Errorf("the battery and the mouse are %v apart, the machine's cards %v", got, want)
	}
	// Each card ends where its row's right column ends: a card as wide as the
	// page would put its badge past the panel's edge.
	for _, badge := range []string{"健康 78%", "无线"} {
		r, ok := tt.Find(badge)
		if !ok {
			t.Fatalf("the card has no badge %q: %v", badge, tt.Texts())
		}
		if r.X+r.W > DesignWidth-dashGutter+0.5 {
			t.Errorf("%q ends at x=%v, past the card that ends at %v",
				badge, r.X+r.W, DesignWidth-dashGutter)
		}
	}

	// The number of each card sits the same distance under its title as on
	// the machine's own cards, so all four rows are laid out alike.
	batteryValue, ok := tt.Find("100")
	if !ok {
		t.Fatalf("the battery shows no level: %v", tt.Texts())
	}
	mouseValue, ok := tt.Find("72")
	if !ok {
		t.Fatalf("the mouse shows no level: %v", tt.Texts())
	}
	cpuTitle, _ := tt.Find("CPU")
	cpuValue, _ := tt.Find("5")
	want := cpuValue.Y - cpuTitle.Y
	if got := batteryValue.Y - battery.Y; abs(got-want) > 1 {
		t.Errorf("the battery's number is %v under its title, the machine's %v", got, want)
	}
	if got := mouseValue.Y - mouse.Y; abs(got-want) > 1 {
		t.Errorf("the mouse's number is %v under its title, the machine's %v", got, want)
	}
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// TestTheMouseCardShowsTheDevice checks the mouse's card says whose it is, how
// it is connected, its level, and what the device calls itself.
func TestTheMouseCardShowsTheDevice(t *testing.T) {
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
	if !tt.HasText("Rapoo Gaming Device") {
		t.Errorf("the mouse's card does not say what the device calls itself: %v", tt.Texts())
	}
	if !tt.HasText("DPI 1200") {
		t.Errorf("the mouse's card does not say the rest of what it reports: %v", tt.Texts())
	}

	// The DPI sits at the far end of the level's own line, where it reads as a
	// property of that level, and the line under the card is the device's name
	// alone.
	level, ok := tt.Find("72")
	if !ok {
		t.Fatalf("the mouse's card has no level: %v", tt.Texts())
	}
	dpi, ok := tt.Find("DPI 1200")
	if !ok {
		t.Fatalf("the mouse's card has no DPI: %v", tt.Texts())
	}
	if dpi.X < level.X+level.W {
		t.Errorf("the DPI at x=%v is not beside the level at x=%v..%v",
			dpi.X, level.X, level.X+level.W)
	}
	if dpi.Y < level.Y-1 || dpi.Y+dpi.H > level.Y+level.H+1 {
		t.Errorf("the DPI at y=%v..%v is not on the level's line at y=%v..%v",
			dpi.Y, dpi.Y+dpi.H, level.Y, level.Y+level.H)
	}
	if tt.HasText("Rapoo Gaming Device · DPI 1200") {
		t.Errorf("the card still repeats the DPI in its line: %v", tt.Texts())
	}
}

// TestTheGPUModelDoesNotPushOutItsTitle checks a card's badge gives way to its
// title: the plugins name a graphics card in the badge, and a long name must be
// cut rather than push "GPU" off the card.
func TestTheGPUModelDoesNotPushOutItsTitle(t *testing.T) {
	h, p := dashFixture(t)
	snap := h.Instance("system").Snapshot()
	for i := range snap.Readings {
		if snap.Readings[i].ID == "gpu" {
			snap.Readings[i].Badge = "NVIDIA GeForce RTX 3060 Laptop GPU"
		}
	}
	h.Inject("system", snap)

	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)
	title, ok := tt.Find("GPU")
	if !ok {
		t.Fatalf("the card lost its title to its badge: %v", tt.Texts())
	}
	// The card has to name the card in less room than the whole model takes:
	// the badge says what it could, and ends at a word where it could.
	var badge ui.Rect
	var model string
	for _, s := range tt.Texts() {
		if strings.HasPrefix(s, "NVIDIA") {
			model = s
			badge, ok = tt.Find(s)
		}
	}
	if model == "" || !ok {
		t.Fatalf("the card does not show the model: %v", tt.Texts())
	}
	if !strings.HasSuffix(model, "…") || strings.HasSuffix(model, " …") {
		t.Errorf("the badge shows %q, want the model cut at a word", model)
	}
	// The badge gives way: it may take half a card, no more, and what is left
	// is the title's.
	cardW := float32(DesignWidth-2*dashGutter-dashGap) / 2
	half := (cardW - 2*dashCardPad) / 2
	if badge.W > half+1 {
		t.Errorf("the model's badge is %v wide, more than half a card (%v)", badge.W, half)
	}
	if title.W < half/2 {
		t.Errorf("the title is %v wide, too narrow to read next to the badge", title.W)
	}
	if badge.X < title.X+title.W {
		t.Errorf("the model at x=%v runs over the title at x=%v..%v",
			badge.X, title.X, title.X+title.W)
	}
	// The badge stays inside its card: the right column ends at the gutter.
	if badge.X+badge.W > DesignWidth-dashGutter+0.5 {
		t.Errorf("the model ends at x=%v, outside the card that ends at %v",
			badge.X+badge.W, DesignWidth-dashGutter)
	}
}

// TestTheFactsStayInsideTheirPills: a line of facts too wide for the panel is
// capped to fit, and capping a pill has to shorten its words. A pill whose box
// is capped while its text is drawn whole runs over the pill beside it — which
// is what a 150% display, where the design's 660 pixels become 440 DIP, showed.
func TestTheFactsStayInsideTheirPills(t *testing.T) {
	h, p := dashFixture(t)
	snap := h.Instance("system").Snapshot()
	snap.Facts = []core.Fact{
		{Value: "Ryzen 7 6800H"},
		{Value: "13.7 GB"},
		{Value: "Windows 11 26H2"},
		{Label: "已运行", Value: "2天18小时"},
	}
	h.Inject("system", snap)
	p.SetSize(Fit(2560, 1440, 1.5)) // 440×740: the design on a 150% display
	tt := ui.NewTester(p.View, p.Size.Width, p.Size.Height)

	// Two pills' words keep both paddings and the gap between the pills: a
	// text wider than the box it was given eats into that room.
	want := float32(2*dashFactPadX + dashFactGap)
	end := float32(0)
	for i, text := range []string{"Ryzen 7 6800H", "13.7 GB", "Windows 11 26H2", "已运行 2天18小时"} {
		r, ok := tt.Find(text)
		if !ok {
			t.Fatalf("the header lost the pill %q: %v", text, tt.Texts())
		}
		if i > 0 {
			if gap := r.X - end; gap < want-0.5 {
				t.Errorf("%q starts %v after the words before it, want %v: its text is drawn over its pill",
					text, gap, want)
			}
		}
		end = r.X + r.W
	}
	if end > float32(p.Size.Width-dashGutter)+0.5 {
		t.Errorf("the last pill's words end at x=%v, outside the header's room", end)
	}
}

// TestALongBadgeEndsAtAWholeWord: a model longer than the badge has room for is
// cut where a word ends, so the badge reads "RTX 3060…" rather than "RTX 3060
// La…", and the words it keeps stay inside its own surface.
func TestALongBadgeEndsAtAWholeWord(t *testing.T) {
	h, p := dashFixture(t)
	snap := h.Instance("system").Snapshot()
	for i := range snap.Readings {
		if snap.Readings[i].ID == "gpu" {
			snap.Readings[i].Badge = "RTX 3060 Laptop"
		}
	}
	h.Inject("system", snap)
	p.SetSize(Fit(2560, 1440, 1.5))
	tt := ui.NewTester(p.View, p.Size.Width, p.Size.Height)

	if !tt.HasText("RTX 3060…") {
		t.Errorf("a model too long for half a card was not ended at a word: %v", tt.Texts())
	}
	if tt.HasText("RTX 3060 Laptop") {
		t.Errorf("the whole model is still drawn, over a badge it does not fit: %v", tt.Texts())
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
// window's width rather than from the design's: the panel on a 150% display is
// narrower than the design, and a card that kept the design's width would hang
// off its edge.
func TestDashboardKeepsItsShapeWhenNarrow(t *testing.T) {
	for _, size := range []Metrics{
		designSize(),
		Fit(1706, 912, 1.5),
		Fit(400, 400, 1), // a small display, where the panel shrinks
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
// with less: no score means the page leads with the machine's name, no programs
// means no list, and no battery means no card — rather than an empty box where
// each would have been.
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

// TestTheGearOpensTheSettingsAndTheBackReturns checks the panel has two pages
// and one way between them: the header's gear opens the settings, and the
// settings' back returns to the machine's page.
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
	if !tt.HasText("开机自启") {
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
