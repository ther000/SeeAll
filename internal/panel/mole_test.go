package panel

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/molecli"
	"seeall/internal/plugins/mole"
)

// moleFixture is the Mole page of a machine that has run all four commands:
// a preview that found 128 MB, a disk whose largest folders Mole knows, a
// maintenance pass with twelve things to do, and two cleanups in its log.
//
// The state is seeded rather than run: a test must not clean a disk, and a
// preview of one takes about a minute.
func moleFixture(t *testing.T) (*app.Host, *App, *mole.Instance) {
	t.Helper()
	h := app.NewHost(t.TempDir())
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	h.OnChange = func() {}

	in := h.Instance("mole")
	if in == nil {
		t.Fatalf("the app has no Mole instance")
	}
	m, ok := in.Monitor().(*mole.Instance)
	if !ok {
		t.Fatalf("the Mole instance is a %T", in.Monitor())
	}

	now := time.Now()
	m.Seed(mole.Data{
		Ready: true,
		Clean: &molecli.Clean{
			DryRun: true, Size: "128 MB", Bytes: 128 << 20, Items: 3, Free: "64.5 GB",
			Groups: []molecli.Group{
				{Name: "开发工具", Paths: []string{`C:\Users\me\AppData\Local\go-build`}},
				{Name: "应用缓存", Paths: []string{`C:\Users\me\AppData\Local\Temp`}},
				{Name: "浏览器缓存", Paths: []string{`C:\Users\me\AppData\Local\Chrome`}},
				{Name: "游戏", Paths: []string{`F:\Games`}},
			},
		},
		CleanAt:  now.Add(-8 * time.Minute),
		Optimize: &molecli.Optimize{DryRun: true, Applied: 12},
		OptAt:    now.Add(-3 * time.Minute),
		Disk: &molecli.Analyze{
			Total: 826 << 30,
			Entries: []molecli.Entry{
				{Name: "游戏 (F:)", Path: `F:\`, Bytes: 425 << 30, Dir: true},
				{Name: "软件 (E:)", Path: `E:\`, Bytes: 250 << 30, Dir: true},
				{Name: "Win11Pro X64 (C:)", Path: `C:\`, Bytes: 95 << 30, Dir: true},
				{Name: "Program Files", Path: `C:\Program Files`, Bytes: 20 << 30, Dir: true},
				{Name: "User AppData", Path: `C:\Users\me\AppData`, Bytes: 17 << 30,
					Dir: true, Insight: true},
				{Name: "Temp Files", Path: `C:\Users\me\AppData\Local\Temp`,
					Bytes: 1 << 30, Dir: true, Cleanable: true, Insight: true},
				{Name: "一个不会被画出来的更小的目录", Path: `C:\Small`, Bytes: 1 << 20, Dir: true},
			},
		},
		DiskAt: now.Add(-15 * time.Minute),
		History: &molecli.History{Sessions: []molecli.Session{
			{Command: "clean", Started: now.Add(-26 * time.Hour), Count: 12},
			{Command: "optimize", Started: now.Add(-50 * time.Hour), Count: 4},
		}},
		HistAt: now.Add(-2 * time.Minute),
	})

	p := New(h)
	p.SetSize(designSize())
	p.SetSelected(in.ID)
	return h, p, m
}

// emptyMole is the same page before anything has been run: Mole is installed
// and has answered nothing yet.
func emptyMole(t *testing.T) (*app.Host, *App, *mole.Instance) {
	t.Helper()
	h, p, m := moleFixture(t)
	m.Seed(mole.Data{Ready: true})
	return h, p, m
}

// TestMolePageSaysWhatItFound checks the page reports the four commands where
// the eye looks for them: what the cleanup would free, where the disk went, how
// many optimizations wait, and what was cleaned before.
func TestMolePageSaysWhatItFound(t *testing.T) {
	_, p, _ := moleFixture(t)
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	for _, want := range []string{
		"Mole",   // the page's heading
		"128 MB", // what the preview found
		"3 项",    // how many things that is
		"开发工具 · 应用缓存 · 浏览器缓存", // what kind of thing it is
		"磁盘占用",
		"游戏 (F:)", // the largest entry, as Mole names it
		"425 GB",
		"12", // the optimizations waiting
		"最近清理",
		"清理", // the log's first session, in words
	} {
		if !tt.HasText(want) {
			t.Errorf("the Mole page does not say %q: %v", want, tt.Texts())
		}
	}

	// The disk list is a panel of six lines: the seventh entry would only push
	// the cards under it further down.
	if tt.HasText("一个不会被画出来的更小的目录") {
		t.Errorf("the disk list drew more entries than it has rows: %v", tt.Texts())
	}

	// Nothing has been run in this frame, so nothing may be running.
	if tt.HasText("取消") {
		t.Errorf("the page offers to cancel a command nobody started: %v", tt.Texts())
	}
}

// TestMolePageFitsItsWidth checks the narrowest panel the design allows — 440
// DIPs, which is a 150% display — keeps every word of the page inside its own
// edge. A row of buttons beside a title is the way this page would overflow.
func TestMolePageFitsItsWidth(t *testing.T) {
	for _, size := range []Metrics{
		designSize(),        // 660×1110
		Fit(1706, 912, 1.5), // 440×740
	} {
		_, p, _ := moleFixture(t)
		p.SetSize(size)
		tt := ui.NewTester(p.View, size.Width, size.Height)
		for _, s := range tt.Texts() {
			r, ok := tt.Find(s)
			if !ok {
				continue
			}
			if r.X < -0.5 || r.X+r.W > float32(size.Width)+0.5 {
				t.Errorf("at %d×%d, %q is at x=%v..%v, outside the panel",
					size.Width, size.Height, s, r.X, r.X+r.W)
			}
		}
	}
}

// TestMoleAsksBeforeItCleans is the page's half of the safety rule: the click
// that cleans is the second one, and the first only turns the button into the
// question — with what it would free, so that the second click is informed.
func TestMoleAsksBeforeItCleans(t *testing.T) {
	_, p, m := moleFixture(t)
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	if err := tt.Click("立即清理"); err != nil {
		t.Fatalf("the cleanup button is not there to click: %v (%v)", err, tt.Texts())
	}
	tt.Frame()
	if !tt.HasText("确认清理") {
		t.Errorf("clicking 立即清理 did not ask for a confirmation: %v", tt.Texts())
	}
	if tt.HasText("立即清理") {
		t.Errorf("the page still offers the cleanup while it asks: %v", tt.Texts())
	}
	if st := m.State(); st.Running != nil {
		t.Errorf("the first click started %s; it should have only asked", st.Running.Action)
	}

	// Backing out leaves the page as it was, and starts nothing.
	if err := tt.Click("取消"); err != nil {
		t.Fatalf("the confirmation cannot be cancelled: %v (%v)", err, tt.Texts())
	}
	tt.Frame()
	if !tt.HasText("立即清理") {
		t.Errorf("cancelling left the page without its cleanup button: %v", tt.Texts())
	}
	if st := m.State(); st.Running != nil {
		t.Errorf("cancelling started %s", st.Running.Action)
	}
}

// TestMoleOffersNoCleanupWithoutAPreview checks the page before anything was
// previewed: there is nothing to confirm, so the button that deletes is not
// drawn at all rather than drawn and refused.
func TestMoleOffersNoCleanupWithoutAPreview(t *testing.T) {
	_, p, _ := emptyMole(t)
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	if !tt.HasText("预览") {
		t.Errorf("the page offers no way to preview a cleanup: %v", tt.Texts())
	}
	if tt.HasText("立即清理") {
		t.Errorf("the page offers a cleanup with nothing previewed: %v", tt.Texts())
	}
	if tt.HasText("立即优化") {
		t.Errorf("the page offers an optimization with nothing previewed: %v", tt.Texts())
	}
	if !tt.HasText("点「预览」看看能清出多少。清理会删掉缓存，重建要重新下载。") {
		t.Errorf("the empty page does not say what a preview does: %v", tt.Texts())
	}
}

// TestMoleTakesBackTheCleanupOnceItsPreviewIsSpent checks the page after a
// cleanup has been started with the preview: what the preview found is still
// worth reading, but the button that deletes is gone until a fresh preview,
// because a run that failed or was cancelled may already have deleted part of
// what that preview showed.
func TestMoleTakesBackTheCleanupOnceItsPreviewIsSpent(t *testing.T) {
	_, p, m := moleFixture(t)
	st := m.State()
	st.CleanSpent = true
	m.Seed(st)
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	if tt.HasText("立即清理") {
		t.Errorf("the page still offers a cleanup on a preview a run has used: %v", tt.Texts())
	}
	if !tt.HasText("128 MB") {
		t.Errorf("the page threw away what the preview found: %v", tt.Texts())
	}
}

// TestMoleSaysHowToInstallIt checks a machine without the CLI: the page names
// the command to install rather than showing four buttons that cannot work.
func TestMoleSaysHowToInstallIt(t *testing.T) {
	_, p, m := moleFixture(t)
	m.Seed(mole.Data{Ready: false})
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	if !tt.HasText("没找到 Mole") {
		t.Errorf("the page does not say Mole is missing: %v", tt.Texts())
	}
	if !tt.HasText("scoop install mole  ·  https://github.com/tw93/Mole") {
		t.Errorf("the page does not say how to install it: %v", tt.Texts())
	}
	if tt.HasText("磁盘占用") || tt.HasText("立即清理") {
		t.Errorf("the page still draws its commands with no CLI to run: %v", tt.Texts())
	}
}

// TestTheMolePageIsNotOnTheDashboard checks the plugin that does things is not
// also a card: the dashboard shows the machine and its devices, and Mole is
// neither. It is reached from the header and the tray, and shows its own page
// when it is opened.
func TestTheMolePageIsNotOnTheDashboard(t *testing.T) {
	h, p, _ := moleFixture(t)
	p.SetSelected("system")
	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)

	for _, notWanted := range []string{"磁盘占用", "最近清理", "立即清理"} {
		if tt.HasText(notWanted) {
			t.Errorf("the dashboard shows the Mole page's %q: %v", notWanted, tt.Texts())
		}
	}
	// Nor is it one of the cards: the machine and the mouse are.
	machine, devices := dashSplit(p.sorted(h.Instances()))
	if machine == nil || machine.ID != "system" {
		t.Errorf("the dashboard's machine is %v, want the system plugin", machine)
	}
	for _, d := range devices {
		if d.ID == "mole" {
			t.Errorf("Mole is drawn as a device card beside the mouse")
		}
	}

	// Opening it puts the page in place of the dashboard.
	p.SetSelected("mole")
	tt = ui.NewTester(p.View, DesignWidth, DesignHeight)
	if !tt.HasText("磁盘占用") {
		t.Errorf("selecting Mole did not open its page: %v", tt.Texts())
	}
}

// TestTheHeaderKeepsRoomForItsActions checks the third header button gives way
// on a panel narrower than the design's own 440 DIPs, rather than squeezing the
// gear off the edge: the two buttons every page needs stay, and Mole is still
// reached from the tray menu. Where it is drawn, it opens the page.
func TestTheHeaderKeepsRoomForItsActions(t *testing.T) {
	for _, c := range []struct {
		name  string
		size  Metrics
		shows bool
	}{
		{"design", designSize(), true},
		{"150% display", Fit(1706, 912, 1.5), true},
		{"tiny display", Fit(400, 400, 1), false},
	} {
		_, p := dashFixture(t)
		p.SetSize(c.size)
		tt := ui.NewTester(p.View, c.size.Width, c.size.Height)

		if got := tt.HasText("Mole"); got != c.shows {
			t.Errorf("%s (%d DIPs wide): the Mole button is drawn = %v, want %v",
				c.name, c.size.Width, got, c.shows)
		}
		if !tt.HasText("打开设置") {
			t.Errorf("%s (%d DIPs wide): the settings button is gone: %v",
				c.name, c.size.Width, tt.Texts())
		}
		if !c.shows {
			continue
		}
		if err := tt.Click("Mole"); err != nil {
			t.Fatalf("%s: the Mole button cannot be clicked: %v", c.name, err)
		}
		if got := p.Selected(); got != "mole" {
			t.Errorf("%s: the Mole button opened %q, want the mole page", c.name, got)
		}
	}
}
