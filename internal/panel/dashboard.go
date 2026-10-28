package panel

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
)

// The dashboard is the panel's other look: one page for the whole machine.
//
// Where the classic page has a title bar, a row of tabs and a page per monitor,
// the dashboard answers one question — how is this machine doing — with the
// score across the top, the machine's six parts in cards under it, a card for
// every other device the panel watches, the battery across the page, and the
// programs behind the numbers. A mouse's level belongs beside the processor's
// load rather than behind a tab, so the panel has no tabs in this look: the
// only other page is the settings, which the header's gear opens and whose own
// header leads back.
//
// The header stays as the page scrolls, so the score and the two things the
// panel can do are always where the eye left them.
//
// The sizes below are the ones a 660×840 panel — the design's own size — holds,
// with the machine's cards, the device card, the battery and the programs all
// on the page. A narrower window keeps every size and gets narrower cards, and
// a shorter one scrolls, which is what the page under the header is for.
const (
	// dashGutter is the room a page keeps from its edges, and dashGap the room
	// between two cards.
	dashGutter = 16
	dashGap    = 10
	// dashBottom is the room the page keeps below its last card.
	dashBottom = 16
	// dashCardH is the height of a reading's card: its two lines of text, the
	// shape of its recent values and the line under them, inside the padding.
	dashCardH   = 108
	dashCardPad = 12
	dashCardR   = 20
	dashChartH  = 14
	// dashBatteryH and dashDeviceH are the cards that have no shape in them: a
	// battery's level and a device's, which is one number and a state.
	dashBatteryH = 96
	dashDeviceH  = 96
	// dashProcRowH is one program's line in the list, and dashProcPad the room
	// the list keeps inside its card.
	dashProcRowH = 19
	dashProcPad  = 10

	// dashValueSize is a card's number, dashUnitSize the unit beside it:
	// the number is the card's subject, the unit only says what it counts.
	dashValueSize = 28
	dashUnitSize  = 14
	dashBadgeSize = 11.5
	dashLabelSize = 13.5
	dashDetailSZ  = 11.5
	dashProcSize  = 12.5
	dashScoreSize = 42
	dashNoteSize  = 13
	// dashActionSize is the header's two icon buttons.
	dashActionSize = 15
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
// the settings page is the one the panel had before the dashboard, and it
// takes its surfaces without losing its layout.
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

// dashboard draws the whole window in the dashboard's look: the settings, when
// they were asked for, and the machine's one page otherwise.
func (a *App) dashboard(c *ui.Context) {
	d := dashFor(c)
	instances := a.sorted(a.Host.Instances())

	ui.Column(c).Fill().Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.FillGradient(r, d.canvas, 0)
		pt.FillGradient(r, d.glow, 0)
	}).Children(func() {
		if a.Selected() == SettingsPage {
			a.dashSettingsHeader(c, d)
			a.dashPage(c, func() {
				a.settings(c, d.palette(), instances, false)
			})
			return
		}
		if len(instances) == 0 {
			a.dashHeader(c, d, core.Snapshot{})
			a.dashPage(c, func() { a.emptyDetail(c, d.palette()) })
			return
		}
		machine, devices := dashSplit(instances)
		var snap core.Snapshot
		if machine != nil {
			snap = machine.Snapshot()
		}
		a.dashHeader(c, d, snap)
		a.dashPage(c, func() { a.dashBody(c, d, machine, snap, devices) })
	})
}

// dashSplit divides the panel's monitors into the machine the dashboard
// composes and the devices it gives a card each. The machine is the monitor
// that watches the machine itself; a plugin whose readings all describe one
// device — a mouse and its battery — is a device, whichever order the panel
// lists the monitors in.
func dashSplit(instances []*app.Instance) (machine *app.Instance, devices []*app.Instance) {
	for _, in := range instances {
		if machine == nil && !in.Manifest().Combined {
			machine = in
			continue
		}
		devices = append(devices, in)
	}
	return machine, devices
}

// dashPage is the room a page scrolls in: everything under the header, clipped
// at both ends so that a long page cannot draw over the header or run out of
// the window's foot.
func (a *App) dashPage(c *ui.Context, page func()) {
	ui.Column(c).Grow(1).Shrink(1).Basis(0).Justify(ui.Start).Clip().Children(page)
}

// dashHeader is the top of the dashboard: how the machine is doing in one
// number, the pills that say what the machine is, and the two things the panel
// can do.
func (a *App) dashHeader(c *ui.Context, d dash, snap core.Snapshot) {
	ui.Column(c).Shrink(0).Padding(8, dashGutter, 4, dashGutter).Gap(6).Children(func() {
		ui.Row(c).Shrink(0).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Grow(1).Shrink(1).Basis(0).Gap(8).AlignItems(ui.Baseline).Children(func() {
				if snap.Health > 0 {
					// A sun for the score's color, set on the number's own
					// line: the eye reads "90 很好" as one phrase, not three.
					ui.Icon(c, iconLogo).FontSize(20).TextColor(d.mint)
					ui.Text(c, fmt.Sprintf("%d", snap.Health)).FontSize(dashScoreSize).Bold().
						TextColor(d.Text).NoWrap()
					if note := snap.HealthNote; note != "" {
						ui.Text(c, note).FontSize(dashNoteSize).TextColor(d.secondary).NoWrap()
					}
					return
				}
				// A machine that does not score itself still gets a heading:
				// what is being watched, and in what state. With no monitor
				// to name, the heading is the panel's own.
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					title := snap.Title
					if title == "" {
						title = "SeeAll"
					}
					ui.Text(c, title).FontSize(20).Bold().NoWrap().MaxLines(1)
					if snap.Subtitle != "" {
						ui.Text(c, snap.Subtitle).FontSize(dashDetailSZ).TextColor(d.muted).
							NoWrap().MaxLines(1)
					}
				})
			})
			if dashIconButton(c, d, "刷新", "刷新全部（Ctrl+R）", iconRefresh).Clicked() {
				a.Host.Refresh()
			}
			if dashIconButton(c, d, "打开设置", "设置", iconGear).Clicked() {
				a.SetSelected(SettingsPage)
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

// dashSettingsHeader is the settings page's heading: the way back to the
// machine, and what the page is.
func (a *App) dashSettingsHeader(c *ui.Context, d dash) {
	ui.Row(c).Shrink(0).Padding(10, dashGutter, 6, dashGutter).Gap(8).AlignItems(ui.Center).Children(func() {
		if dashIconButton(c, d, "返回", "返回", iconBack).Clicked() {
			a.dashBack()
		}
		ui.Text(c, "设置").FontSize(20).Bold().Grow(1).NoWrap()
	})
}

// dashBack leaves the settings for the machine's page. The dashboard is one
// page, so "back" is the monitor the panel was on last: the app remembers that
// when a monitor is chosen, and going to the settings does not overwrite it.
func (a *App) dashBack() {
	target := a.Host.LastInstance()
	if a.Host.Instance(target) == nil {
		target = ""
		if instances := a.sorted(a.Host.Instances()); len(instances) > 0 {
			target = instances[0].ID
		}
	}
	if target == "" {
		return
	}
	a.SetSelected(target)
}

// dashIconButton is one of the header's actions: an icon with no box around
// it, because the header is the box. It brightens as the pointer finds it.
func dashIconButton(c *ui.Context, d dash, label, tooltip string, icon *ui.SVG) ui.Element {
	button := ui.Button(c, "").Label(label).Description(tooltip).Tooltip(tooltip).
		Padding(4, 6).Radius(10).Background(ui.Transparent).Border(0, ui.Transparent)
	tint := d.muted
	if button.Hovered() {
		tint = d.Text
	}
	button.Children(func() {
		ui.Icon(c, icon).FontSize(dashActionSize).TextColor(tint)
	})
	return button
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

// dashBody is the scrolling part of the dashboard, top to bottom: the
// machine's cards, a card for every other device, the battery across the page,
// and the programs behind the numbers.
func (a *App) dashBody(c *ui.Context, d dash, machine *app.Instance, snap core.Snapshot, devices []*app.Instance) {
	ui.Scroll(c).Fill().Padding(0, dashGutter, dashBottom, dashGutter).Gap(dashGap).Children(func() {
		a.dashCards(c, d, machine, snap, devices)
		if battery, ok := snap.Reading("battery"); ok {
			a.dashBattery(c, d, snap, battery)
		}
		if len(snap.Processes) > 0 {
			a.dashProcesses(c, d, snap.Processes)
		}
	})
}

// dashCards lays the machine's readings out two to a row, and the devices the
// panel also watches under them: a device is not a part of the machine, so its
// card follows the machine's own readings rather than sitting among them, and
// comes before the battery, which is the machine's.
func (a *App) dashCards(c *ui.Context, d dash, machine *app.Instance, snap core.Snapshot, devices []*app.Instance) {
	half := float32(a.Size.Width-2*dashGutter-dashGap) / 2
	readings := dashReadings(snap)
	for i := 0; i < len(readings); i += 2 {
		row := readings[i:min(i+2, len(readings))]
		ui.Row(c).Shrink(0).Gap(dashGap).Children(func() {
			for _, r := range row {
				a.dashCard(c, d, machine, snap, r, half)
			}
		})
	}
	// A device's card takes the whole page width, like the battery's: it is
	// not a cell of the machine's grid but a card of its own, and a card
	// standing alone in a half-empty row reads as a layout that went wrong
	// rather than as a row waiting for a second device.
	full := float32(a.Size.Width - 2*dashGutter)
	for _, in := range devices {
		a.dashDevice(c, d, in, full)
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

// dashDevice is a device the panel watches besides the machine, as one card
// among the machine's: what it is called, how it is connected, its level and
// what else it reports. A device that is not answering still gets its card —
// a card that vanishes is a device the user forgets the panel watches.
func (a *App) dashDevice(c *ui.Context, d dash, in *app.Instance, width float32) {
	snap := in.Snapshot()
	primary, reported := snap.Lead()
	value, unit := "—", ""
	if reported {
		value, unit = dashValue(primary)
	}
	ui.Column(c).Shrink(0).Width(width).Height(dashDeviceH).MaxHeight(dashDeviceH).
		Padding(dashCardPad).Gap(2).Radius(dashCardR).Background(d.card).Children(func() {
		ui.Row(c).Shrink(0).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconFor(in.Manifest().Icon)).FontSize(13).
				TextColor(dashColor(d, primary))
			ui.Text(c, in.Name).FontSize(dashLabelSize).TextColor(d.muted).
				NoWrap().MaxLines(1).Grow(1)
			if badge := dashDeviceBadge(snap); badge != "" {
				dashBadge(c, d, badge)
			}
		})
		dashBig(c, d, value, unit)
		if detail := dashDeviceDetail(snap, primary, reported); detail != "" {
			ui.Text(c, detail).FontSize(dashDetailSZ).TextColor(d.muted).
				NoWrap().MaxLines(1)
		}
	})
}

// dashDeviceBadge is how a device is connected, in the device's own words: a
// mouse says "无线". A device that has stopped answering says so instead.
func dashDeviceBadge(snap core.Snapshot) string {
	if len(snap.Readings) == 0 || snap.Status == core.StatusOffline {
		return "未连接"
	}
	if link, ok := snap.Reading("link"); ok {
		if text := link.Format(); text != "" {
			return text
		}
	}
	if snap.Status == core.StatusOK {
		return ""
	}
	return snap.Status.String()
}

// dashDeviceDetail is what else a device reports, on its card's one line: the
// level is the card's number and the connection its badge, and everything else
// goes here, as "DPI 1200".
func dashDeviceDetail(snap core.Snapshot, primary core.Reading, reported bool) string {
	parts := make([]string, 0, len(snap.Readings))
	for _, r := range snap.Readings {
		if r.ID == "link" || reported && r.ID == primary.ID {
			continue
		}
		text := r.Format()
		if text == "" {
			continue
		}
		if r.Label != "" {
			text = r.Label + " " + text
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, " · ")
}

// dashNumber is a reading's value set large, with its unit beside it: the one
// thing on a card that is read from across the room.
func dashNumber(c *ui.Context, d dash, r core.Reading) {
	value, unit := dashValue(r)
	dashBig(c, d, value, unit)
}

// dashBig is a card's number and its unit, set on one baseline.
func dashBig(c *ui.Context, d dash, value, unit string) {
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
