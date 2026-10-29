package panel

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
)

// The dashboard is the panel: one page for the whole machine.
//
// The sky over the machine leads the top with the score beside it and the
// machine's facts under them, the machine's six parts in cards under those, and
// the battery and every other device the panel watches on the row after them —
// the same size as any other card, because a mouse's level is the same kind of
// fact as a processor's load. The programs behind the numbers close the page.
// The only other page is the settings, which the header's gear opens and whose
// own header leads back.
//
// A machine with no service to ask about the sky leads with its own score
// instead, in the size the sky would have taken, and a machine that does not
// score itself leads with its name.
//
// The sizes below are the ones a 660×1110 panel — the design's own size, which
// is 440×740 DIPs on a 150% display — holds with nothing pushed out of the
// window: a lead line, the machine's facts, four rows of cards, the programs,
// and the room under them. A narrower window keeps every size and gets narrower
// cards; a display whose whole work area is smaller than the design gets a
// smaller panel, and its page scrolls.
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

// dashMoleButtonWidth is how wide the panel must be to carry a third header
// action. Below the design's own 440 DIPs — a 150% display — the header keeps
// the two buttons every page needs, the refresh and the settings, and Mole is
// reached from the tray menu: an icon squeezed to nothing is a click nobody can
// land on.
const dashMoleButtonWidth = 440

// dashAwakeButtonWidth is how wide the panel must be to carry the hold on the
// screen, the action a small display loses first. The header's floor is the
// two buttons every page needs, the refresh and the settings: 400×400 DIPs
// gives the panel 238, where those two are the whole room there is.
const dashAwakeButtonWidth = 300

// dashMetricIDs are the readings the dashboard makes a card of first, in the
// order it puts them: the machine's six parts, widest to narrowest. A reading
// that is not here still gets a card, after these, unless it is one of the
// values the cards fold in: vram into the GPU's line, uptime into the header's
// pills, battery into a card of its own after the machine's parts.
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
	// scorePill is the surface the machine's score sits on once the weather
	// has the header: the facts' pill, tinted with the accent.
	scorePill ui.Color
	// secondary and muted are the second and third ranks of text.
	secondary, muted ui.Color
	// mint, teal, amber, gold and blue color what the cards measure.
	mint, teal, amber, gold, blue ui.Color
	// track is the unfilled part of a bar.
	track ui.Color
}

// dashFor builds the dashboard's look and tells the widgets under it to draw
// their text for a dark surface, so that a button or a scroll bar on this page
// is the dashboard's too.
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
		scorePill: ui.RGBA(79, 214, 169, 0.14),
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

// palette is the dashboard's look in the shape the settings page draws with.
func (d dash) palette() palette {
	return palette{
		Theme:      d.Theme,
		Card:       d.card,
		CardBorder: d.hairline,
		OK:         d.mint,
		Warn:       d.amber,
		Critical:   ui.Hex("#f87171"),
		Offline:    d.muted,
		Track:      d.track,
	}
}

// dashboard draws the whole window: the machine's page, or the settings when
// they were asked for.
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
				a.settings(c, d.palette(), instances)
			})
			return
		}
		// A plugin that does something draws its own page: Mole cleans the
		// disk, which needs a preview and a confirmation rather than a card.
		if in := a.ownPage(); in != nil {
			a.molePage(c, d, in)
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
// lists the monitors in. A plugin with a page of its own is neither: it is
// drawn there, not here.
func dashSplit(instances []*app.Instance) (machine *app.Instance, devices []*app.Instance) {
	for _, in := range instances {
		if in.Manifest().Standalone {
			continue
		}
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

// dashHeader is the top of the dashboard: the sky over the machine when a
// service can say, how the machine itself is doing, the pills that say what the
// machine is, and the two things the panel can do.
func (a *App) dashHeader(c *ui.Context, d dash, snap core.Snapshot) {
	ui.Column(c).Shrink(0).Padding(8, dashGutter, 4, dashGutter).Gap(6).Children(func() {
		ui.Row(c).Shrink(0).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Grow(1).Shrink(1).Basis(0).Gap(8).AlignItems(ui.Baseline).Children(func() {
				if w := snap.Weather; w != nil {
					dashWeather(c, d, *w)
					return
				}
				dashScore(c, d, snap)
			})
			// The score steps aside once the sky leads: it is still the one
			// number about the machine, but it is a fact about the machine
			// now rather than the page's subject, so it is set as a pill the
			// way the machine's facts are.
			if snap.Weather != nil && snap.Health > 0 {
				dashScorePill(c, d, snap.Health, snap.HealthNote)
			}
			if dashIconButton(c, d, "刷新", "刷新全部（Ctrl+R）", iconRefresh, false).Clicked() {
				a.Host.Refresh()
			}
			// Holding the screen on is a mode rather than a click that opens
			// something, so its button stays lit for as long as it is on. A
			// panel too narrow for it keeps the two actions with no other way
			// in rather than squeezing one of them off its edge.
			if a.Size.Width >= dashAwakeButtonWidth {
				if dashIconButton(c, d, keepAwakeLabel, keepAwakeNote(a.KeepAwake), iconAwake, a.KeepAwake).Clicked() {
					a.toggleKeepAwake(c)
				}
			}
			if a.Size.Width >= dashMoleButtonWidth {
				if in := a.ownInstance(); in != nil {
					if dashIconButton(c, d, "Mole", "Mole 清理", iconMole, false).Clicked() {
						a.SetSelected(in.ID)
					}
				}
			}
			if dashIconButton(c, d, "打开设置", "设置", iconGear, false).Clicked() {
				a.SetSelected(SettingsPage)
			}
		})
		if len(snap.Facts) > 0 {
			caps := factCaps(c, a.Size.Width, snap.Facts)
			ui.Row(c).Shrink(0).Gap(dashFactGap).Children(func() {
				for i, f := range snap.Facts {
					var cap float32
					if i < len(caps) {
						cap = caps[i]
					}
					dashFact(c, d, f.Text(), cap)
				}
			})
		}
	})
}

// dashWeather leads the header with the sky: the shape it has, the temperature
// in the score's own size, and then the sky and the city it is over in a word.
// It is the same three things the score says — a mark, a number and a word —
// about the place the machine is in rather than about the machine.
func dashWeather(c *ui.Context, d dash, w core.Weather) {
	ui.Icon(c, weatherIcon(w.Kind)).FontSize(20).TextColor(dashSkyColor(d, w.Kind))
	ui.Text(c, fmt.Sprintf("%.0f°", w.TempC)).FontSize(dashScoreSize).Bold().
		TextColor(d.Text).NoWrap().Tooltip(weatherTooltip(w))

	// The sky and the place are one phrase — "多云 · 重庆" — so they are one
	// text: the sky in the second rank of color, the city in the third, which
	// is the order they matter in.
	spans := make([]ui.Span, 0, 3)
	if w.Text != "" {
		spans = append(spans, ui.Span{Text: w.Text, Size: dashNoteSize, Color: d.secondary})
	}
	if w.Place != "" {
		if len(spans) > 0 {
			spans = append(spans, ui.Span{Text: " · ", Size: dashNoteSize, Color: d.muted})
		}
		spans = append(spans, ui.Span{Text: w.Place, Size: dashNoteSize, Color: d.muted})
	}
	if len(spans) == 0 {
		return
	}
	ui.RichText(c, spans...).MaxLines(1)
}

// weatherTooltip says when the temperature was taken, which is the one thing
// about it the header has no room for.
func weatherTooltip(w core.Weather) string {
	if w.At.IsZero() {
		return ""
	}
	return fmt.Sprintf("天气 %s 更新", w.At.Format("15:04"))
}

// dashSkyColor colors the sky's shape by what it shows: the sun is the
// dashboard's gold, what falls out of the cloud is its blue, and a covered sky
// is the quietest rank of text.
func dashSkyColor(d dash, kind string) ui.Color {
	switch kind {
	case "sun", "partly":
		return d.gold
	case "rain", "snow":
		return d.blue
	case "thunder":
		return d.amber
	}
	return d.secondary
}

// dashScorePill is the machine's score once the weather leads the header: the
// facts' pill in the score's own color, because it is a judgement of the whole
// machine rather than one of its parts.
//
// It sits on the header's first line, beside the two buttons, and not on the
// facts' line: four facts fill a 440 DIP panel exactly, and a fifth pill there
// would cut them. The score is the one thing on the page that may be made
// smaller without losing anything.
func dashScorePill(c *ui.Context, d dash, health int, note string) {
	text := fmt.Sprintf("%d", health)
	if note != "" {
		text += " " + note
	}
	ui.Row(c).Shrink(0).Padding(4, dashFactPadX).Radius(12).Background(d.scorePill).
		Children(func() {
			ui.Text(c, text).FontSize(12.5).TextColor(d.mint).MaxLines(1)
		})
}

// dashScore is the header's lead when there is no sky to show: how the machine
// is doing in one number, or — for a source that does not score itself — what
// is being watched and in what state.
func dashScore(c *ui.Context, d dash, snap core.Snapshot) {
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
		ui.Text(c, title).FontSize(20).Bold().MaxLines(1)
		if snap.Subtitle != "" {
			ui.Text(c, snap.Subtitle).FontSize(dashDetailSZ).TextColor(d.muted).
				MaxLines(1)
		}
	})
}

// dashSettingsHeader is the settings page's heading: the way back to the
// machine, and what the page is.
func (a *App) dashSettingsHeader(c *ui.Context, d dash) {
	ui.Row(c).Shrink(0).Padding(8, dashGutter, 4, dashGutter).Gap(8).AlignItems(ui.Center).Children(func() {
		if dashIconButton(c, d, "返回", "返回", iconBack, false).Clicked() {
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
// it, because the header is the box. It brightens as the pointer finds it, and
// an action that is on takes the accent instead, so a mode the panel is in
// shows without a word.
func dashIconButton(c *ui.Context, d dash, label, tooltip string, icon *ui.SVG, on bool) ui.Element {
	button := ui.Button(c, "").Label(label).Description(tooltip).Tooltip(tooltip).
		Padding(4, 6).Radius(10).Background(ui.Transparent).Border(0, ui.Transparent)
	tint := d.muted
	switch {
	case on && button.Hovered():
		tint = d.AccentHover
	case on:
		tint = d.Accent
	case button.Hovered():
		tint = d.Text
	}
	button.Children(func() {
		ui.Icon(c, icon).FontSize(dashActionSize).TextColor(tint)
	})
	return button
}

// keepAwakeLabel names the header's hold on the screen: the label is what a
// reader hears, and the note under it says what a click would do right now.
const keepAwakeLabel = "保持屏幕常亮"

// keepAwakeNote is what the button would do if it were pressed.
func keepAwakeNote(on bool) string {
	if on {
		return "屏幕常亮、电脑不睡，点一下交还给系统"
	}
	return "让屏幕常亮、电脑不进入睡眠"
}

// toggleKeepAwake holds the screen on, or lets the power plan have it back.
// A machine that refuses says so rather than leaving a lit button over a screen
// that still turns itself off.
func (a *App) toggleKeepAwake(c *ui.Context) {
	on := !a.KeepAwake
	if err := a.OnKeepAwake(on); err != nil {
		c.Toast("改不了屏幕常亮：" + err.Error())
		return
	}
	a.KeepAwake = on
}

// dashFactGap is the room between two pills, and dashFactPadX the room a pill
// keeps to the left and right of its text.
//
// The padding is the width at which the four facts a machine usually shows —
// the processor, its memory, the system and the uptime — fit a 440 DIP panel,
// which is what a 150% display gives the design: reading "已运行 2天18小时" cut
// to "已运行 2天18小…" would be the pill line paying for 2 DIP of padding it
// never needed.
const (
	dashFactGap  = 6
	dashFactPadX = 7
)

// factCaps is how wide each pill may be, in the order of the facts. A line
// with room for its words gets no caps: every pill keeps the width its words
// ask for. One without gets the room the short pills leave over, so that the
// words that have to be cut are the longest ones, and are cut as little as the
// line allows.
func factCaps(c *ui.Context, width int, facts []core.Fact) []float32 {
	if len(facts) < 2 {
		return nil
	}
	room := float32(width-2*dashGutter) - float32(len(facts)-1)*dashFactGap
	need := make([]float32, len(facts))
	total := float32(0)
	for i, f := range facts {
		w, _ := c.MeasureText(0, ui.Span{Text: f.Text(), Size: 12.5})
		need[i] = w + 2*dashFactPadX
		total += need[i]
	}
	if total <= room {
		return nil
	}
	caps := make([]float32, len(facts))
	fixed := make([]bool, len(facts))
	for left, count := room, len(facts); count > 0; {
		share := left / float32(count)
		grew := false
		for i := range need {
			if !fixed[i] && need[i] <= share {
				fixed[i], grew = true, true
				left -= need[i]
				count--
			}
		}
		if grew {
			continue
		}
		for i := range need {
			if !fixed[i] {
				caps[i] = share
			}
		}
		break
	}
	return caps
}

// dashFact is one pill: a fact about the machine, not a measurement of it, so
// it is set smaller and quieter than the cards under it. A cap of zero means
// the line has room for its words.
func dashFact(c *ui.Context, d dash, text string, cap float32) ui.Element {
	pill := ui.Row(c).Shrink(1).Padding(4, dashFactPadX).Radius(12).Background(d.pill)
	if cap > 0 {
		pill.MaxWidth(cap)
	}
	pill.Children(func() {
		// The words are what gives way: a box narrower than its text would
		// draw the rest over the pill beside it. MaxLines ends the cut with
		// an ellipsis, and NoWrap has to stay off — text that may not wrap is
		// text the shaper never cuts.
		label := ui.Text(c, text).FontSize(12.5).TextColor(d.pillText).MaxLines(1)
		if cap > 2*dashFactPadX {
			label.MaxWidth(cap - 2*dashFactPadX)
		}
	})
	return pill
}

// dashBody is the scrolling part of the dashboard, top to bottom: the cards,
// and the programs behind the numbers.
func (a *App) dashBody(c *ui.Context, d dash, machine *app.Instance, snap core.Snapshot, devices []*app.Instance) {
	ui.Scroll(c).Fill().Padding(0, dashGutter, dashBottom, dashGutter).Gap(dashGap).Children(func() {
		a.dashCards(c, d, machine, snap, devices)
		if len(snap.Processes) > 0 {
			a.dashProcesses(c, d, snap.Processes)
		}
	})
}

// dashItem is one card of the page: a reading of the machine's, or a device the
// panel watches.
type dashItem struct {
	reading core.Reading
	device  *app.Instance
}

// dashCards lays the page's cards out two to a row, all of them one size: the
// machine's readings, then the battery, then a card for every other device.
// The battery and a mouse are facts of the same weight as a processor's load,
// so they are cards of the same size rather than strips across the page.
func (a *App) dashCards(c *ui.Context, d dash, machine *app.Instance, snap core.Snapshot, devices []*app.Instance) {
	items := dashItems(snap, devices)
	half := float32(a.Size.Width-2*dashGutter-dashGap) / 2
	for i := 0; i < len(items); i += 2 {
		row := items[i:min(i+2, len(items))]
		ui.Row(c).Shrink(0).Gap(dashGap).Children(func() {
			for _, it := range row {
				if it.device != nil {
					a.dashDeviceCard(c, d, it.device, half)
					continue
				}
				a.dashCard(c, d, machine, snap, it.reading, half)
			}
		})
	}
}

// dashItems is the page's cards in the order it shows them: the machine's six
// parts as it has them, any other reading it reports, the battery, and then the
// devices beside the machine.
func dashItems(snap core.Snapshot, devices []*app.Instance) []dashItem {
	readings := dashReadings(snap)
	items := make([]dashItem, 0, len(readings)+1+len(devices))
	for _, r := range readings {
		items = append(items, dashItem{reading: r})
	}
	if battery, ok := snap.Reading("battery"); ok {
		items = append(items, dashItem{reading: battery})
	}
	for _, in := range devices {
		items = append(items, dashItem{device: in})
	}
	return items
}

// dashReadings is the machine's own cards, in the dashboard's order: the six
// parts of it as it has them, then any other reading it reports that no other
// part of the page already says.
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

// dashCard is one reading: what it is and its badge across the top, the number
// under them, the shape of its recent values, and the line that explains them.
// Every card is the same height, so a row of them is a row.
func (a *App) dashCard(c *ui.Context, d dash, in *app.Instance, snap core.Snapshot, r core.Reading, width float32) {
	color := dashColor(d, r)
	ui.Column(c).Shrink(0).Width(width).Height(dashCardH).MaxHeight(dashCardH).
		Padding(dashCardPad).Gap(2).Radius(dashCardR).Background(d.card).Children(func() {
		ui.Row(c).Shrink(0).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, dashIcon(r.ID)).FontSize(13).TextColor(color)
			ui.Text(c, r.Label).FontSize(dashLabelSize).TextColor(d.muted).
				MaxLines(1).Grow(1)
			if r.Badge != "" {
				dashBadge(c, d, r.Badge, width)
			}
		})
		dashNumber(c, d, r)
		dashChart(c, d, in, r, color)
		if detail := dashDetail(snap, r); detail != "" {
			ui.Text(c, detail).FontSize(dashDetailSZ).TextColor(d.muted).
				MaxLines(1)
		}
	})
}

// dashDeviceCard is a device the panel watches besides the machine, as a card
// among the machine's: what it is called, how it is connected, its level, the
// shape of that level, and what the device is and says besides. A device that
// is not answering still gets its card — a card that vanishes is a device the
// user forgets the panel watches.
func (a *App) dashDeviceCard(c *ui.Context, d dash, in *app.Instance, width float32) {
	snap := in.Snapshot()
	primary, reported := snap.Lead()
	value, unit := "—", ""
	if reported {
		value, unit = dashValue(primary)
	}
	aside, detail := dashDeviceLines(snap, primary, reported)
	color := dashColor(d, primary)
	ui.Column(c).Shrink(0).Width(width).Height(dashCardH).MaxHeight(dashCardH).
		Padding(dashCardPad).Gap(2).Radius(dashCardR).Background(d.card).Children(func() {
		ui.Row(c).Shrink(0).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconFor(in.Manifest().Icon)).FontSize(13).TextColor(color)
			ui.Text(c, in.Name).FontSize(dashLabelSize).TextColor(d.muted).
				MaxLines(1).Grow(1)
			if badge := dashDeviceBadge(snap); badge != "" {
				dashBadge(c, d, badge, width)
			}
		})
		// The level leads; the device's other number sits at the far end of the
		// same line, where it reads as a property of that level.
		ui.Row(c).Shrink(0).AlignItems(ui.Baseline).Justify(ui.SpaceBetween).Children(func() {
			dashBig(c, d, value, unit)
			if aside != "" {
				ui.Text(c, aside).FontSize(dashDetailSZ).TextColor(d.muted).MaxLines(1)
			}
		})
		dashChart(c, d, in, primary, color)
		if detail != "" {
			ui.Text(c, detail).FontSize(dashDetailSZ).TextColor(d.muted).
				MaxLines(1)
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

// dashDeviceLines splits what a device reports into the two places its card
// has: the aside beside the level — the device's other number, such as a
// mouse's DPI — and the one line under it, which names the device and says
// whatever neither the level, the badge nor the aside does.
func dashDeviceLines(snap core.Snapshot, primary core.Reading, reported bool) (aside, detail string) {
	parts := make([]string, 0, len(snap.Readings)+1)
	if snap.Title != "" {
		parts = append(parts, snap.Title)
	}
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
		if aside == "" {
			aside = text
			continue
		}
		parts = append(parts, text)
	}
	return aside, strings.Join(parts, " · ")
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

// dashBadge is the small tag beside a card's title: a frequency, a model, a
// state in two words. It gives way before the title does: a card whose label
// had been pushed out by its badge would not say what it measures.
func dashBadge(c *ui.Context, d dash, text string, cardWidth float32) ui.Element {
	// The badge's own padding, which is what its words have less of.
	const padX, padY = 7, 1
	max := float32(0)
	if cardWidth > 0 {
		if room := cardWidth*0.5 - dashCardPad; room > 0 {
			max = room
		}
	}
	badge := ui.Row(c).Shrink(0).Padding(padY, padX).Radius(8).Background(d.badge)
	shown := text
	if max > 0 {
		badge.MaxWidth(max)
		shown = fitBadge(c, text, max-2*padX)
	}
	badge.Children(func() {
		// The badge's own box is only as wide as its text, so the text is what
		// has to give way: a box that kept its natural width would draw the
		// model over the card's edge. MaxLines ends the cut with an ellipsis,
		// and NoWrap has to stay off — text that may not wrap is text the
		// shaper never cuts, however narrow its box.
		label := ui.Text(c, shown).FontSize(dashBadgeSize).TextColor(d.badgeText).
			MaxLines(1)
		if max > 2*padX {
			label.MaxWidth(max - 2*padX)
		}
		if shown != text {
			// The badge says less than the card knows; what a reader cannot
			// see, assistive technology still gets whole.
			label.Description(text)
		}
	})
	return badge
}

// fitBadge shortens a badge to fit max, dropping whole words where it can, so
// that a model reads "RTX 3060…" rather than "RTX 3060 La…". Text that does not
// break at spaces — a Chinese state, a volume label — is left whole for the
// shaper to end with its own ellipsis.
func fitBadge(c *ui.Context, text string, max float32) string {
	if max <= 0 {
		return text
	}
	fits := func(s string) bool {
		w, _ := c.MeasureText(0, ui.Span{Text: s, Size: dashBadgeSize})
		return w <= max
	}
	if fits(text) {
		return text
	}
	for _, r := range text {
		if r > 0x7F {
			return text
		}
	}
	words := strings.Fields(text)
	for len(words) > 1 {
		words = words[:len(words)-1]
		if candidate := strings.Join(words, " ") + "…"; fits(candidate) {
			return candidate
		}
	}
	return text
}

// dashProcesses names the programs behind the numbers, heaviest first. It is
// a list rather than a chart because a machine's problem is usually a name.
//
// Its two number columns keep their room and the name takes what is left, so
// that a panel narrower than the design shortens the names rather than running
// its columns out of the card.
func (a *App) dashProcesses(c *ui.Context, d dash, procs []core.Process) {
	if len(procs) > 5 {
		procs = procs[:5]
	}
	const (
		colMem  = 76
		colCPU  = 48
		colGap  = 8
		twoCols = 2 * colGap
	)
	room := float32(a.Size.Width - 4*dashGutter)
	if room < 1 {
		room = 1
	}
	memW := min32(colMem, room*0.3)
	cpuW := min32(colCPU, room*0.18)
	nameW := room - memW - cpuW - twoCols
	if nameW < 0 {
		nameW = 0
	}
	ui.Column(c).Shrink(0).Padding(dashProcPad, dashGutter).Radius(dashCardR).
		Background(d.card).Gap(4).Children(func() {
		ui.Row(c).Shrink(0).Gap(colGap).Children(func() {
			title := ui.Text(c, "高占用进程").FontSize(13).TextColor(d.muted).MaxLines(1).Grow(1)
			if nameW > 0 {
				title.MaxWidth(nameW)
			}
			ui.Text(c, "内存").FontSize(12).TextColor(d.muted).Width(memW).
				TextAlign(ui.End).NoWrap()
			ui.Text(c, "% CPU").FontSize(12).TextColor(d.muted).Width(cpuW).
				TextAlign(ui.End).NoWrap()
		})
		for _, p := range procs {
			ui.Row(c).Shrink(0).Height(dashProcRowH).Gap(colGap).AlignItems(ui.Center).Children(func() {
				name := ui.Text(c, p.Name).FontSize(dashProcSize).TextColor(d.Text).
					MaxLines(1).Grow(1)
				if nameW > 0 {
					name.MaxWidth(nameW)
				}
				ui.Text(c, core.FormatBytes(p.Mem)).FontSize(12).TextColor(d.secondary).
					Width(memW).TextAlign(ui.End).NoWrap()
				ui.Text(c, fmt.Sprintf("%.1f", p.CPU)).FontSize(12).TextColor(d.secondary).
					Width(cpuW).TextAlign(ui.End).NoWrap()
			})
		}
	})
}
