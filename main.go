// SeeAll is a tray app that watches everything: the mouse on the desk, the
// keyboard beside it, the computer it runs on, the servers it talks to and
// the agents working on this machine. Each kind of thing is a plugin; the
// panel is the same panel for all of them.
//
//	go run .
package main

import (
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"seeall/internal/app"
	"seeall/internal/console"
	"seeall/internal/core"
	"seeall/internal/panel"
	"seeall/internal/plugin"
	"seeall/internal/tray"
)

// state is what main keeps: the host of the plugins, the panel's state, and
// the window and tray that show them.
type state struct {
	host  *app.Host
	panel *panel.App
	win   *mygo.Window
	tray  *mygo.Tray

	metrics   panel.Metrics
	trayState tray.State
	tooltip   string
	quitting  bool
}

func main() {
	// A "go build" without -H=windowsgui makes a console program, which
	// opens a black window beside the tray icon. Hide it when it is ours
	// alone; a shell that started us keeps its own console.
	console.HideIfOwned()

	s := &state{}

	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.App.SetName("SeeAll")

	if dir, err := mygo.App.Path(mygo.PathLogs); err == nil {
		if f, err := os.OpenFile(filepath.Join(dir, "seeall.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			log.SetOutput(f)
			defer f.Close()
		}
	}

	dataDir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		dataDir = "."
	}
	s.host = app.NewHost(dataDir)
	s.host.Logf = func(format string, args ...any) { log.Printf(format, args...) }
	s.panel = panel.New(s.host)
	s.panel.Select = func(string) { s.refresh() }

	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		s.quitting = true
		s.host.Stop()
		if s.tray != nil {
			s.tray.Destroy()
		}
	})
	mygo.App.WhenReady(s.ready)

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (s *state) ready() {
	if err := s.host.Load(); err != nil {
		log.Printf("读取设置失败：%v", err)
	}
	s.host.OnNotify = func(title, body string) {
		n := mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: body})
		if err := n.Show(); err != nil {
			log.Printf("通知失败：%v", err)
		}
	}
	s.host.OnChange = s.refresh

	// Apply the saved choice; a machine that has never run SeeAll starts
	// with it on, as the panel's switch shows.
	if err := mygo.App.SetOpenAtLogin(s.host.StartAtLogin()); err != nil {
		log.Printf("设置开机自启失败：%v", err)
	}

	s.createTray()
	s.createWindow()
	s.refresh()

	// The panel opens on the first launch, so a new install shows what it
	// found instead of an icon in a corner. A launch at login stays quiet.
	if !mygo.App.WasOpenedAtLogin() {
		s.show()
	}
}

// screenWorkArea is the work area of the display the panel docks into: the
// one holding the tray icon, or the primary display when the icon reports no
// bounds, as AppIndicator does on Linux.
//
// It reads the displays, so it only runs once the app is ready.
func (s *state) screenWorkArea() mygo.Rectangle {
	if s.tray != nil {
		if anchor := s.tray.Bounds(); anchor.Width > 0 && anchor.Height > 0 {
			return mygo.Screen.DisplayNearestPoint(mygo.Point{
				X: anchor.X + anchor.Width/2,
				Y: anchor.Y + anchor.Height/2,
			}).WorkArea
		}
	}
	return mygo.Screen.PrimaryDisplay().WorkArea
}

func (s *state) createWindow() {
	area := s.screenWorkArea()
	s.metrics = panel.Fit(area.Width, area.Height)
	s.panel.SetSize(s.metrics)
	x, y := tray.Dock(area, s.metrics.Width, s.metrics.Height)

	s.win = mygo.NewWindow(mygo.WindowOptions{
		Title:             "SeeAll",
		Width:             s.metrics.Width,
		Height:            s.metrics.Height,
		X:                 x,
		Y:                 y,
		Hidden:            true,
		Frameless:         true,
		DisableResize:     true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		AlwaysOnTop:       true,
		SkipTaskbar:       true,
		BackgroundColor:   "light-dark(#fafafa, #17171a)",
		Content:           panel.Content(s.panel),
	})
	s.win.OnBlur(s.win.Hide)
	s.win.OnClose(func(e *mygo.CloseEvent) {
		// Alt+F4 and Escape put the panel away; quitting still closes it.
		if !s.quitting {
			e.PreventDefault()
			s.win.Hide()
		}
	})

	// A display plugged in, unplugged or rescaled moves and resizes the
	// panel: it stays docked in the corner of its display.
	mygo.Screen.OnDisplaysChanged(func() {
		s.refit()
		s.refreshTrayIcon()
	})
}

// refit re-measures the display and keeps the panel in its corner.
func (s *state) refit() {
	area := s.screenWorkArea()
	if area.Width == 0 {
		return
	}
	m := panel.Fit(area.Width, area.Height)
	if m.Width != s.metrics.Width || m.Height != s.metrics.Height {
		s.metrics = m
		s.panel.SetSize(m)
		s.win.SetSize(m.Width, m.Height)
	}
	x, y := tray.Dock(area, s.metrics.Width, s.metrics.Height)
	s.win.SetPosition(x, y)
	s.win.Invalidate()
}

func (s *state) createTray() {
	t, err := mygo.NewTray(mygo.TrayOptions{
		ID:      "seeall",
		Icon:    tray.Icon(tray.State{Level: -1, Waiting: true, Dark: true}),
		ToolTip: "SeeAll",
	})
	if err != nil {
		log.Printf("托盘图标失败：%v", err)
		return
	}
	s.tray = t
	s.tray.OnClick(s.toggle)
	s.tray.OnRightClick(func() {
		s.win.Hide()
		s.tray.SetMenu(s.buildMenu())
		defer s.tray.SetMenu(nil)
		s.tray.PopUpMenu()
	})
	// The icon's ink follows the taskbar: white on a dark one.
	s.tray.OnAppearanceChange(s.refreshTrayIcon)
}

// refresh recomputes what the icon and the tooltip say and asks the panel
// for a frame. Snapshots arrive on the plugins' goroutines; the tray and the
// window take calls from any of them.
func (s *state) refresh() {
	if s.tray == nil || s.win == nil {
		return
	}
	s.refreshTrayIcon()
	s.win.Invalidate()
}

// refreshTrayIcon repaints the tray icon for the newest snapshot.
func (s *state) refreshTrayIcon() {
	if s.tray == nil {
		return
	}
	st, tip := s.state()
	changed := st != s.trayState || tip != s.tooltip
	s.trayState, s.tooltip = st, tip
	if !changed {
		return
	}
	if err := s.tray.SetIcon(tray.Icon(st), false); err != nil {
		log.Printf("画托盘图标失败：%v", err)
	}
	s.tray.SetToolTip(tip)
}

// state says what the icon shows: a device's battery when there is one,
// otherwise the worst state among the sources, and the tooltip lists them.
func (s *state) state() (tray.State, string) {
	dark := true
	if s.tray != nil {
		dark = s.tray.Dark()
	}
	instances := s.host.Instances()
	if len(instances) == 0 {
		return tray.State{Level: -1, Waiting: true, Dark: dark}, "SeeAll · 还没有监控项"
	}

	lines := make([]string, 0, len(instances)+1)
	lines = append(lines, "SeeAll")
	var device *app.Instance
	for _, in := range instances {
		snap := in.Snapshot()
		value := panel.Headline(snap)
		if value == "" {
			value = snap.Status.String()
		}
		lines = append(lines, in.Name+" · "+value)
		if in.Manifest().Kind != plugin.KindDevice {
			continue
		}
		if _, ok := snap.Reading("battery"); ok {
			if device == nil || snap.Status == core.StatusOK {
				device = in
			}
		}
	}
	tooltip := strings.Join(lines, "\n")

	if device != nil {
		snap := device.Snapshot()
		reading, ok := snap.Reading("battery")
		if !ok {
			return tray.State{Level: -1, Waiting: true, Dark: dark}, tooltip
		}
		st := tray.State{
			Level: reading.Value,
			Color: severityColor(reading.Severity, dark),
			Dark:  dark,
		}
		switch snap.Status {
		case core.StatusOffline:
			st.Offline = true
		case core.StatusStarting:
			st.Waiting = true
		}
		return st, tooltip
	}

	worst := core.StatusOK
	for _, in := range instances {
		if st := in.Snapshot().Status; statusRank(st) > statusRank(worst) {
			worst = st
		}
	}
	st := tray.State{Level: -1, Color: statusColor(worst, dark), Dark: dark}
	if worst == core.StatusOffline {
		st.Offline = true
	}
	return st, tooltip
}

func statusRank(s core.Status) int {
	switch s {
	case core.StatusError:
		return 3
	case core.StatusWarn:
		return 2
	case core.StatusOffline:
		return 1
	}
	return 0
}

// severityColor and statusColor match the panel's palette, for the icon the
// tray draws outside of it.
func severityColor(s core.Severity, dark bool) color.NRGBA {
	switch s {
	case core.SeverityWarn:
		return tint(dark, 0xFB, 0xBF, 0x24, 0xB4, 0x53, 0x09)
	case core.SeverityCritical:
		return tint(dark, 0xF8, 0x71, 0x71, 0xDC, 0x26, 0x26)
	case core.SeverityOffline:
		return tint(dark, 0x71, 0x71, 0x7A, 0xA1, 0xA1, 0xAA)
	default:
		return tint(dark, 0x34, 0xD3, 0x99, 0x0F, 0x9D, 0x63)
	}
}

func statusColor(s core.Status, dark bool) color.NRGBA {
	switch s {
	case core.StatusWarn:
		return severityColor(core.SeverityWarn, dark)
	case core.StatusError:
		return severityColor(core.SeverityCritical, dark)
	case core.StatusOffline:
		return severityColor(core.SeverityOffline, dark)
	default:
		return severityColor(core.SeverityOK, dark)
	}
}

func tint(dark bool, dr, dg, db, lr, lg, lb uint8) color.NRGBA {
	if dark {
		return color.NRGBA{R: dr, G: dg, B: db, A: 255}
	}
	return color.NRGBA{R: lr, G: lg, B: lb, A: 255}
}

// toggle shows the panel docked in its corner, or puts it away.
func (s *state) toggle() {
	if s.win.IsVisible() {
		s.win.Hide()
		return
	}
	s.show()
}

func (s *state) show() {
	s.refit()
	s.win.Show()
	s.win.Focus()
}

// buildMenu is the tray's right-click menu: every monitor, then the app's
// own items.
func (s *state) buildMenu() *mygo.Menu {
	instances := s.host.Instances()
	items := make([]*mygo.MenuItem, 0, len(instances)+8)
	for _, in := range instances {
		snap := in.Snapshot()
		label := in.Name + " · " + snap.Status.String()
		if value := panel.Headline(snap); value != "" {
			label = in.Name + " — " + value
		}
		id := in.ID
		items = append(items, &mygo.MenuItem{
			Label: label,
			Click: func(*mygo.MenuItem, *mygo.Window) {
				s.panel.SetSelected(id)
				s.show()
			},
		})
	}
	if len(instances) == 0 {
		items = append(items, &mygo.MenuItem{Label: "还没有监控项", Disabled: true})
	}
	items = append(items,
		mygo.Separator(),
		&mygo.MenuItem{Label: "刷新全部", Accelerator: "CmdOrCtrl+R",
			Click: func(*mygo.MenuItem, *mygo.Window) { s.host.Refresh() }},
		&mygo.MenuItem{Label: "设置…", Click: func(*mygo.MenuItem, *mygo.Window) {
			s.panel.SetSelected(panel.SettingsPage)
			s.show()
		}},
		&mygo.MenuItem{Label: "打开日志", Click: func(*mygo.MenuItem, *mygo.Window) {
			if dir, err := mygo.App.Path(mygo.PathLogs); err == nil {
				mygo.Shell.ShowItemInFolder(filepath.Join(dir, "seeall.log"))
			}
		}},
		mygo.Separator(),
		&mygo.MenuItem{Role: mygo.RoleQuit},
	)
	return mygo.NewMenu(items)
}
