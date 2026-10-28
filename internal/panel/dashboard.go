package panel

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
)

// The dashboard is the panel's other look. Where the classic page is a title
// bar over a row of tabs over cards of readings, the dashboard is one canvas:
// how the machine is doing across the top, what each part of it is doing on
// the cards under that, and a bar at the foot that says what is watched and
// offers the two things the panel can do.
//
// It is drawn from the top of the window down, and its page scrolls between a
// header that stays and a bar that stays, so the score and the way out are
// always where the eye left them.
//
// The sizes below are the ones a 660×840 panel — the design's own size — holds
// with nothing pushed out of the window: a score and the machine's four facts,
// three rows of cards, the battery, the programs behind the numbers, and the
// bar. A narrower window keeps every size and gets narrower cards, and a
// shorter one scrolls, which is what the page between the header and the bar
// is for.
const (
	// dashGutter is the room a page keeps from its edges, and dashGap the room
	// between two cards.
	dashGutter = 16
	dashGap    = 12
	// dashCardH is the height of a reading's card: its two lines of text, the
	// shape of its recent values and the line under them, inside the padding.
	dashCardH   = 112
	dashCardPad = 12
	dashCardR   = 20
	dashChartH  = 18
	// dashBatteryH is the battery's card, which has no shape in it.
	dashBatteryH = 96
	// dashProcRowH is one program's line in the list, and dashProcPad the room
	// the list keeps inside its card.
	dashProcRowH = 19
	dashProcPad  = 12
	// dashBarPad is the room inside the bar at the foot.
	dashBarPad = 5

	// dashValueSize is a card's number, dashUnitSize the unit beside it:
	// the number is the card's subject, the unit only says what it counts.
	dashValueSize = 28
	dashUnitSize  = 14
	dashBadgeSize = 11.5
	dashLabelSize = 13.5
	dashDetailSZ  = 11.5
	dashProcSize  = 12.5
	dashStatSize  = 22
	dashStatLabel = 12
	dashScoreSize = 42
	dashNoteSize  = 13
)

// dashMetricIDs are the readings the dashboard makes a card of, in the order
// it puts them: the machine's six parts, widest to narrowest. A reading that
// is not here still gets a card, after these, unless it is one of the values
// the cards fold in: vram into the GPU's line, uptime into the header's
// pills, battery into a card of its own across the page.
var dashMetricIDs = []string{"cpu", "gpu", "mem", "disk", "net", "temp"}

// dashFolded are the readings another part of the page says already.
var dashFolded = map[string]bool{"vram": true, "uptime": true, "battery": true}

// dash is the dashboard's look: a canvas of two colors with a warm light at
// its foot, cards that lift a little off it, and the accents that color a
// reading by what it measures.
//
// The colors are the dashboard's own rather than the desktop appearance's: it
// is a dark panel by design, so it must not turn into a light one under a
// light Windows.
type dash struct {
	*ui.Theme

	// canvas is the window's background, glow the warm light over its foot.
	canvas, glow ui.LinearGradient
	// bar is the foot's own gradient: the canvas darkened, warmed at the foot.
	bar ui.LinearGradient
	// card is the face of a card, badge and pill the two small surfaces on it.
	card, badge, pill ui.Color
	// hairline separates two groups inside a surface.
	hairline ui.Color
	// badgeText and pillText are the words on those surfaces.
	badgeText, pillText ui.Color
	// secondary and muted are the second and third ranks of text.
	secondary, muted ui.Color
	// mint, teal, amber, gold and blue color what the cards measure.
	mint, teal, amber, gold, blue ui.Color
	// track is the unfilled part of a bar.
	track ui.Color
}

// dashFor builds the dashboard's look and tells the widgets under it to draw
// their text for a dark surface, so that a button or a scroll bar on this
// page is the dashboard's too.
func dashFor(c *ui.Context) dash {
	base := *c.Theme()
	base.Dark = true
	base.Background = ui.Hex("#232840")
	base.Surface = ui.RGBA(255, 255, 255, 0.06)
	base.SurfaceHover = ui.RGBA(255, 255, 255, 0.1)
	base.SurfacePressed = ui.RGBA(255, 255, 255, 0.14)
	base.Border = ui.RGBA(255, 255, 255, 0.1)
	base.Text = ui.Hex("#edecee")
	base.TextMuted = ui.Hex("#9a9ba5")
	base.Accent = ui.Hex("#4fd6a9")
	base.AccentHover = ui.Hex("#63e0b6")
	base.AccentPressed = ui.Hex("#3fc79b")
	base.AccentText = ui.Hex("#0d1f1a")
	base.Danger = ui.Hex("#f87171")
	base.Warning = ui.Hex("#d9a26e")
	base.Success = ui.Hex("#4fd6a9")
	base.Selection = ui.RGBA(79, 214, 169, 0.3)
	base.Focus = ui.Hex("#4fd6a9")
	base.Scrollbar = ui.RGBA(255, 255, 255, 0.2)
	base.Radius = 10
	base.Spacing = 4
	c.SetTheme(&base)

	d := dash{
		Theme: &base,
		canvas: ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
			{Color: ui.Hex("#2b2b43"), At: 0},
			{Color: ui.Hex("#1f2f4b"), At: 1},
		}},
		glow: ui.LinearGradient{Angle: 172, Stops: []ui.GradientStop{
			{Color: ui.RGBA(58, 51, 43, 0), At: 0.76},
			{Color: ui.RGBA(104, 84, 58, 0.85), At: 1},
		}},
		bar: ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
			{Color: ui.Hex("#2b2c3d"), At: 0.15},
			{Color: ui.Hex("#3a332b"), At: 1},
		}},
		card:      ui.RGBA(255, 255, 255, 0.05),
		badge:     ui.RGBA(255, 255, 255, 0.07),
		pill:      ui.RGBA(255, 255, 255, 0.05),
		hairline:  ui.RGBA(255, 255, 255, 0.08),
		badgeText: ui.Hex("#b9bcc8"),
		pillText:  ui.Hex("#a9adba"),
		secondary: ui.Hex("#bdbfc6"),
		muted:     ui.Hex("#9a9ba5"),
		mint:      ui.Hex("#4fd6a9"),
		teal:      ui.Hex("#45b394"),
		amber:     ui.Hex("#d9a26e"),
		gold:      ui.Hex("#ebd18f"),
		blue:      ui.Hex("#4a8fe7"),
		track:     ui.RGBA(255, 255, 255, 0.1),
	}
	return d
}

// palette is the dashboard's look in the shape the pages it borrows draw with:
// the settings page and a device's compact cards are the ones the panel had
// before the dashboard, and they take its surfaces without losing their
// layout.
func (d dash) palette() palette {
	return palette{
		Theme:      d.Theme,
		Card:       d.card,
		CardBorder: d.hairline,
		Dot:        d.mint,
		OK:         d.mint,
		Warn:       d.amber,
		Critical:   ui.Hex("#f87171"),
		Offline:    d.muted,
		Track:      d.track,
	}
}

// dashboard draws the whole window in the dashboard's look.
func (a *App) dashboard(c *ui.Context) {
	d := dashFor(c)
	instances := a.Host.Instances()
	a.ensureSelection(instances)
	sel := a.Selected()

	ui.Column(c).Fill().Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.FillGradient(r, d.canvas, 0)
		pt.FillGradient(r, d.glow, 0)
	}).Children(func() {
		switch {
		case sel == SettingsPage:
			a.dashPage(c, func() { a.settings(c, d.palette(), instances) })
		default:
			in := a.Host.Instance(sel)
			switch {
			case in == nil:
				a.dashPage(c, func() { a.emptyDetail(c, d.palette()) })
			case in.Manifest().Combined:
				// A device whose readings describe one thing — a mouse and
				// its battery, its link and its DPI — keeps the card that
				// holds them together. They are not the parts of a machine,
				// and a grid of them would be three cards of one subject.
				a.dashHeader(c, d, in)
				a.dashPage(c, func() { a.detail(c, d.palette(), in) })
			case machineSnapshot(in.Snapshot()):
				a.dashHeader(c, d, in)
				a.dashPage(c, func() { a.dashBody(c, d, in) })
			default:
				// A device the dashboard has no composition for — a mouse,
				// a sensor — keeps the compact cards it had: one card per
				// group of readings, two to a row, under the heading that
				// names the device.
				a.dashHeader(c, d, in)
				a.dashPage(c, func() { a.detail(c, d.palette(), in) })
			}
		}
		a.dashBar(c, d, instances)
	})
}

// machineSnapshot reports whether a page describes a machine the dashboard
// can compose: it scores itself, names its parts, or lists what it runs. A
// page of a device's readings falls back to the compact cards.
func machineSnapshot(snap core.Snapshot) bool {
	if snap.Health > 0 || len(snap.Facts) > 0 || len(snap.Processes) > 0 {
		return true
	}
	for _, id := range dashMetricIDs {
		if _, ok := snap.Reading(id); ok {
			return true
		}
	}
	return false
}

// dashPage is the room a page scrolls in: everything between the header and
// the bar, clipped at both ends so that a long page cannot draw over either.
func (a *App) dashPage(c *ui.Context, page func()) {
	ui.Column(c).Grow(1).Shrink(1).Basis(0).Justify(ui.Start).Clip().Children(page)
}

// dashHeader is the top of the dashboard: how the machine is doing in one
// number, and the four pills that say what the machine is.
func (a *App) dashHeader(c *ui.Context, d dash, in *app.Instance) {
	snap := in.Snapshot()
	ui.Column(c).Shrink(0).Padding(10, dashGutter, 6, dashGutter).Gap(7).Children(func() {
		ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Baseline).Children(func() {
			if snap.Health > 0 {
				// A sun for the score's color, set on the number's own line:
				// the eye reads "90 很好" as one phrase, not as three.
				ui.Icon(c, iconLogo).FontSize(20).TextColor(d.mint)
				ui.Text(c, fmt.Sprintf("%d", snap.Health)).FontSize(dashScoreSize).Bold().
					TextColor(d.Text).NoWrap()
				if note := snap.HealthNote; note != "" {
					ui.Text(c, note).FontSize(dashNoteSize).TextColor(d.secondary).
						NoWrap().Grow(1)
				}
			} else {
				// A source that does not score itself still gets a heading:
				// what is being watched, and in what state.
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, deviceName(snap)).FontSize(20).Bold().
						NoWrap().MaxLines(1)
					if snap.Subtitle != "" {
						ui.Text(c, snap.Subtitle).FontSize(11.5).TextColor(d.muted).
							NoWrap().MaxLines(1)
					}
				})
			}
		})
		if len(snap.Facts) > 0 {
			share := factShare(c, a.Size.Width, snap.Facts)
			ui.Row(c).Shrink(0).Gap(dashFactGap).Children(func() {
				for _, f := range snap.Facts {
					dashFact(c, d, f.Text(), share)
				}
			})
		}
	})
}

// dashFactGap is the room between two pills, and dashFactPadX the room a pill
// keeps to the left and right of its text.
const (
	dashFactGap  = 6
	dashFactPadX = 9
)

// factShare is how wide a pill may be. A line of facts that fits keeps the
// width its words ask for; one that does not gives all of them equal shares,
// so that the line stays a line on a panel narrower than the design, and the
// longest fact is cut rather than pushed out of the window.
func factShare(c *ui.Context, width int, facts []core.Fact) float32 {
	if len(facts) < 2 {
		return 0
	}
	room := float32(width-2*dashGutter) - float32(len(facts)-1)*dashFactGap
	total := float32(0)
	for _, f := range facts {
		w, _ := c.MeasureText(0, ui.Span{Text: f.Text(), Size: 12.5})
		total += w + 2*dashFactPadX
	}
	if total <= room {
		return 0
	}
	return room / float32(len(facts))
}

// dashFact is one pill: a fact about the machine, not a measurement of it, so
// it is set smaller and quieter than the cards under it. A share of zero means
// the line has room for its words.
func dashFact(c *ui.Context, d dash, text string, share float32) ui.Element {
	pill := ui.Row(c).Shrink(1).Padding(4, dashFactPadX).Radius(12).Background(d.pill)
	if share > 0 {
		pill.MaxWidth(share)
	}
	pill.Children(func() {
		ui.Text(c, text).FontSize(12.5).TextColor(d.pillText).NoWrap().MaxLines(1)
	})
	return pill
}

// dashBody is the scrolling part of the dashboard: the machine's cards, the
// battery across the page, and the programs behind the numbers.
func (a *App) dashBody(c *ui.Context, d dash, in *app.Instance) {
	snap := in.Snapshot()
	ui.Scroll(c).Fill().Padding(0, dashGutter, dashGap, dashGutter).Gap(dashGap).Children(func() {
		a.dashCards(c, d, in, snap)
		if battery, ok := snap.Reading("battery"); ok {
			a.dashBattery(c, d, snap, battery)
		}
		if len(snap.Processes) > 0 {
			a.dashProcesses(c, d, snap.Processes)
		}
	})
}

// dashCards lays the readings out two to a row, in the order the dashboard
// composes them: the machine's own readings first, then whatever else the
// source reports.
func (a *App) dashCards(c *ui.Context, d dash, in *app.Instance, snap core.Snapshot) {
	readings := dashReadings(snap)
	if len(readings) == 0 {
		return
	}
	half := float32(a.Size.Width-2*dashGutter-dashGap) / 2
	for i := 0; i < len(readings); i += 2 {
		row := readings[i:min(i+2, len(readings))]
		ui.Row(c).Shrink(0).Gap(dashGap).Children(func() {
			for _, r := range row {
				a.dashCard(c, d, in, snap, r, half)
			}
		})
	}
}

// dashReadings is the cards a page shows, in the dashboard's order: the six
// parts of a machine as it has them, then any other reading the source
// reports that no other part of the page already says.
func dashReadings(snap core.Snapshot) []core.Reading {
	out := make([]core.Reading, 0, len(snap.Readings))
	used := map[string]bool{}
	for _, id := range dashMetricIDs {
		if r, ok := snap.Reading(id); ok {
			out = append(out, r)
			used[id] = true
		}
	}
	for _, r := range snap.Readings {
		if used[r.ID] || dashFolded[r.ID] {
			continue
		}
		out = append(out, r)
	}
	return out
}

// dashCard is one reading: what it is and its badge across the top, the
// number under them, the shape of its recent values, and the line that
// explains them. Every card is the same height, so a row of them is a row.
func (a *App) dashCard(c *ui.Context, d dash, in *app.Instance, snap core.Snapshot, r core.Reading, width float32) {
	color := dashColor(d, r)
	ui.Column(c).Shrink(0).Width(width).Height(dashCardH).MaxHeight(dashCardH).
		Padding(dashCardPad).Gap(2).Radius(dashCardR).Background(d.card).Children(func() {
		ui.Row(c).Shrink(0).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, dashIcon(r.ID)).FontSize(13).TextColor(color)
			ui.Text(c, r.Label).FontSize(dashLabelSize).TextColor(d.muted).
				NoWrap().MaxLines(1).Grow(1)
			if r.Badge != "" {
				dashBadge(c, d, r.Badge)
			}
		})
		dashNumber(c, d, r)
		dashChart(c, d, in, r, color)
		if detail := dashDetail(snap, r); detail != "" {
			ui.Text(c, detail).FontSize(dashDetailSZ).TextColor(d.muted).
				NoWrap().MaxLines(1)
		}
	})
}

// dashNumber is a reading's value set large, with its unit beside it: the one
// thing on a card that is read from across the room.
func dashNumber(c *ui.Context, d dash, r core.Reading) {
	value, unit := dashValue(r)
	ui.Row(c).Shrink(0).AlignItems(ui.Baseline).Gap(0).Children(func() {
		ui.Text(c, value).FontSize(dashValueSize).Bold().TextColor(d.Text).
			NoWrap().MaxLines(1)
		if unit != "" {
			ui.Text(c, unit).FontSize(dashUnitSize).TextColor(d.muted).NoWrap()
		}
	})
}

// dashBadge is the small tag beside a card's title: a frequency, a
// temperature, a state in two words.
func dashBadge(c *ui.Context, d dash, text string) ui.Element {
	return ui.Row(c).Shrink(0).Padding(1, 7).Radius(8).Background(d.badge).Children(func() {
		ui.Text(c, text).FontSize(dashBadgeSize).TextColor(d.badgeText).
			NoWrap().MaxLines(1)
	})
}

// dashBattery is the battery's own card, across the page: a level is one
// number and a state, and there is no shape in it worth a chart.
func (a *App) dashBattery(c *ui.Context, d dash, snap core.Snapshot, r core.Reading) {
	ui.Column(c).Shrink(0).Height(dashBatteryH).Padding(dashCardPad).
		Radius(dashCardR).Background(d.card).Gap(2).Children(func() {
		ui.Row(c).Shrink(0).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconBattery).FontSize(13).TextColor(d.mint)
			ui.Text(c, r.Label).FontSize(dashLabelSize).TextColor(d.muted).
				NoWrap().MaxLines(1).Grow(1)
			if r.Badge != "" {
				dashBadge(c, d, r.Badge)
			}
		})
		dashNumber(c, d, r)
		if detail := dashDetail(snap, r); detail != "" {
			ui.Text(c, detail).FontSize(dashDetailSZ).TextColor(d.muted).
				NoWrap().MaxLines(1)
		}
	})
}

// dashProcesses names the programs behind the numbers, heaviest first. It is
// a list rather than a chart because a machine's problem is usually a name.
func (a *App) dashProcesses(c *ui.Context, d dash, procs []core.Process) {
	if len(procs) > 5 {
		procs = procs[:5]
	}
	const memW, cpuW = 76, 48
	ui.Column(c).Shrink(0).Padding(dashProcPad, dashGutter).Radius(dashCardR).
		Background(d.card).Gap(4).Children(func() {
		ui.Row(c).Shrink(0).Gap(8).Children(func() {
			ui.Text(c, "高占用进程").FontSize(13).TextColor(d.muted).NoWrap().Grow(1)
			ui.Text(c, "内存").FontSize(12).TextColor(d.muted).Width(memW).
				TextAlign(ui.End).NoWrap()
			ui.Text(c, "% CPU").FontSize(12).TextColor(d.muted).Width(cpuW).
				TextAlign(ui.End).NoWrap()
		})
		for _, p := range procs {
			ui.Row(c).Shrink(0).Height(dashProcRowH).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, p.Name).FontSize(dashProcSize).TextColor(d.Text).
					Grow(1).NoWrap().MaxLines(1)
				ui.Text(c, core.FormatBytes(p.Mem)).FontSize(12).TextColor(d.secondary).
					Width(memW).TextAlign(ui.End).NoWrap()
				ui.Text(c, fmt.Sprintf("%.1f", p.CPU)).FontSize(12).TextColor(d.secondary).
					Width(cpuW).TextAlign(ui.End).NoWrap()
			})
		}
	})
}

// dashBar is the foot of the panel: which monitor is shown and the two things
// the panel can do, a rule, and how the machine stands. It does not scroll
// with the page: the way out of a long page must not be at the end of it.
func (a *App) dashBar(c *ui.Context, d dash, instances []*app.Instance) {
	items := a.sorted(instances)
	ui.Column(c).Shrink(0).Margin(0, dashGap, dashGap, dashGap).
		Padding(dashBarPad, 14).Gap(3).Radius(22).Border(1, ui.RGBA(255, 255, 255, 0.06)).
		LinearGradient(d.bar).Children(func() {
		ui.Row(c).Shrink(0).MinHeight(24).Gap(12).AlignItems(ui.Center).Children(func() {
			for _, in := range items {
				a.dashReport(c, d, in)
			}
			ui.Spacer(c).Grow(1)
			if dashIconButton(c, d, "刷新", "刷新全部（Ctrl+R）", iconRefresh, false).Clicked() {
				a.Host.Refresh()
			}
			on := a.Selected() == SettingsPage
			if dashIconButton(c, d, "打开设置", "设置", iconGear, on).Clicked() {
				a.SetSelected(SettingsPage)
			}
		})
		ui.Box(c).Shrink(0).Height(1).Background(d.hairline)
		ui.Row(c).Shrink(0).AlignItems(ui.End).Children(func() {
			dashStat(c, d, "可用空间", a.dashFreeSpace(items))
			dashStat(c, d, "异常项", dashProblems(items))
			dashStat(c, d, "监控项", fmt.Sprintf("%d", len(items)))
		})
	})
}

// dashIconButton is one of the foot's two actions: an icon with no box around
// it, because the bar itself is the box.
func dashIconButton(c *ui.Context, d dash, label, tooltip string, icon *ui.SVG, on bool) ui.Element {
	button := ui.Button(c, "").Label(label).Description(tooltip).Tooltip(tooltip).
		Padding(4, 8).Radius(10).Background(ui.Transparent).Border(0, ui.Transparent)
	button.Children(func() {
		ui.Icon(c, icon).FontSize(15).TextColor(colorIf(on, d.mint, d.secondary))
	})
	return button
}

// dashReport is one monitor in the foot's first line: its icon and its name,
// lit while its page is the one on screen.
func (a *App) dashReport(c *ui.Context, d dash, in *app.Instance) {
	on := a.Selected() == in.ID
	tab := ui.Button(c, "").Label(in.Name).Description("查看" + in.Name).Tooltip(in.Name)
	tab.Shrink(0).Padding(2, 6).Radius(10).Background(ui.Transparent).
		Border(0, ui.Transparent).FocusRing(false)
	tab.Children(func() {
		ui.Icon(c, iconFor(in.Manifest().Icon)).FontSize(13).
			TextColor(colorIf(on, d.mint, d.muted))
		ui.Text(c, in.Name).FontSize(12.5).NoWrap().MaxLines(1).
			TextColor(colorIf(on, d.Text, d.muted))
	})
	if tab.Clicked() {
		a.SetSelected(in.ID)
	}
}

// dashStat is one of the three numbers at the foot: what it counts above,
// what it is under.
func dashStat(c *ui.Context, d dash, label, value string) {
	ui.Column(c).Grow(1).Basis(0).AlignItems(ui.Center).Children(func() {
		ui.Text(c, value).FontSize(dashStatSize).Bold().NoWrap().MaxLines(1)
		ui.Text(c, label).FontSize(dashStatLabel).TextColor(d.muted).NoWrap()
	})
}

// dashFreeSpace is the room left on the machine's disk, from whichever monitor
// watches it: the bar belongs to the window rather than to one page, and a
// device's page still has a machine under it. A dash when nothing watches the
// disk.
func (a *App) dashFreeSpace(items []*app.Instance) string {
	for _, in := range items {
		if r, ok := in.Snapshot().Reading("disk"); ok {
			return core.FormatBytes(r.Value)
		}
	}
	return "—"
}

// dashProblems counts what wants the user's eye across every monitor: a
// reading at a warning or worse, and a source that is well or badly off. The
// bar counts the window's trouble, so it must not forget the page not shown.
func dashProblems(items []*app.Instance) string {
	n := 0
	for _, in := range items {
		snap := in.Snapshot()
		for _, r := range snap.Readings {
			if r.Severity == core.SeverityWarn || r.Severity == core.SeverityCritical {
				n++
			}
		}
		if snap.Status == core.StatusWarn || snap.Status == core.StatusError {
			n++
		}
	}
	return fmt.Sprintf("%d", n)
}
