package panel

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
)

// cardsPerRow is how many reading cards the page puts side by side: two fit
// the panel at its narrowest, wide enough for a number and its unit.
const cardsPerRow = 2

// trendWidth and trendHeight size the trend in the top right of a card. The
// room is kept whether a reading has a trend or not, so that cards in a row
// keep the same height and the trends line up.
const (
	trendWidth  = 54
	trendHeight = 18
)

// detail is a monitor's page. A plugin whose readings belong together gets one
// card with all of them; any other gets a card per reading, all of them the
// same size, two to a row.
func (a *App) detail(c *ui.Context, p palette, in *app.Instance) {
	m := in.Manifest()
	snap := in.Snapshot()

	// GridView insets its cells by 2 DIPs on each side of a page's column. The
	// column gives that back, so a card's text starts on the panel's gutter
	// whether the page is a grid of cards or one card with everything in it.
	const gridInset = 2
	ui.Column(c).Fill().Padding(CardGap, Gutter-gridInset).Gap(CardGap).Children(func() {
		if m.Combined {
			ui.Scroll(c).Fill().Gap(CardGap).Padding(0, gridInset).Children(func() {
				a.combined(c, p, in, snap)
				if m.Experimental {
					ui.Row(c).Gap(8).Children(func() { badge(c, "实验", p.Warn) })
				}
			})
			return
		}
		a.cards(c, p, in, snap)
	})
}

// cards lays every reading out as a card of the same size, two to a row, in the
// order the user dragged them into. The reading a plugin leads with starts at
// the head of the grid the first time a page is seen: a page of one grid reads
// as a set of peers, which is what these readings are.
//
// GridView drags its cells into another order (GridState.Reorder), which is
// what the cards want: equal boxes in fixed rows.
func (a *App) cards(c *ui.Context, p palette, in *app.Instance, snap core.Snapshot) {
	if len(snap.Readings) == 0 {
		a.heroEmpty(c, p, snap)
		return
	}
	primary, hasPrimary := pickPrimary(snap)

	// The plugin's own order, the reading it leads with first.
	natural := make([]core.Reading, 0, len(snap.Readings))
	if hasPrimary {
		natural = append(natural, primary)
	}
	for _, r := range snap.Readings {
		if hasPrimary && r.ID == primary.ID {
			continue
		}
		natural = append(natural, r)
	}

	readings := arrangeOrders(natural, a.Host.CardOrder(in.ID))
	lead := hasPrimary && readings[0].ID == primary.ID
	state := a.gridState(in, readings)

	// Two cards to a row: the grid gives each card half its width, less the
	// gap between them.
	width := a.Size.Width - 2*Gutter
	half := float32(width)/cardsPerRow - CardGap/2
	ui.GridView(c, state, len(readings), half, ReadingCardH, func(i int) {
		a.readingCard(c, p, in, readings[i], lead && i == 0)
	}).Grow(1).Shrink(1).Basis(0)
}

// gridState keeps one GridState per instance, remembering the order its cards
// were dragged into.
func (a *App) gridState(in *app.Instance, readings []core.Reading) *ui.GridState {
	if a.grids == nil {
		a.grids = map[string]*ui.GridState{}
	}
	state := a.grids[in.ID]
	if state == nil {
		state = &ui.GridState{}
		a.grids[in.ID] = state
	}
	order := orderIDs(readings)
	state.Key = func(item int) any {
		if item < 0 || item >= len(order) {
			return nil
		}
		return order[item]
	}
	state.Reorder = func(items []int, to int) {
		if len(items) != 1 {
			// The user chose several cards before dragging one; the page
			// moves them as a block, in the order they already had.
			return
		}
		a.Host.SetCardOrder(in.ID, moveCard(order, items[0], to))
	}
	return state
}

// combined is the page of a plugin whose readings describe one device: its
// name across the top, the reading that matters on the left, and the rest of
// them down the right, one to a line.
func (a *App) combined(c *ui.Context, p palette, in *app.Instance, snap core.Snapshot) {
	primary, ok := pickPrimary(snap)
	if !ok {
		a.heroEmpty(c, p, snap)
		return
	}
	var rest []core.Reading
	for _, r := range snap.Readings {
		if r.ID != primary.ID {
			rest = append(rest, r)
		}
	}

	card(c, p).Shrink(0).Children(func() {
		// The device first: what the numbers below belong to.
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, deviceName(snap)).FontSize(13).Bold().NoWrap().MaxLines(1).Grow(1)
			if primary.Detail != "" {
				ui.Text(c, primary.Detail).FontSize(11).TextColor(p.TextMuted).NoWrap()
			}
		})
		ui.Row(c).AlignItems(ui.Center).Gap(16).Children(func() {
			a.headline(c, p, primary)
			// The device's other readings sit beside its level rather than
			// under it: one card, read left to right.
			if len(rest) > 0 {
				ui.Column(c).Grow(1).Basis(0).Gap(10).Children(func() {
					for _, r := range rest {
						a.inlineReading(c, p, in, r)
					}
				})
			}
		})
	})
}

// headline is the reading a card is about: the ring for a percentage, the
// value itself for anything else. A combined page shows no trend and no
// wording beside it — the number and the device name say enough.
func (a *App) headline(c *ui.Context, p palette, primary core.Reading) {
	if primary.Unit == core.UnitPercent {
		ui.Box(c).Size(110, 110).
			Draw(func(pt *ui.Painter, r ui.Rect) {
				drawRing(pt, r, p.Track, clamp01(primary.Value/100),
					p.severityColor(primary.Severity), 10)
			}).Children(func() {
			ui.Column(c).Fill().Center().Children(func() {
				ui.Row(c).AlignItems(ui.Baseline).Gap(1).Children(func() {
					bigValue(c, core.FormatNumber(primary.Value), 34,
						p.severityColor(primary.Severity))
					ui.Text(c, "%").FontSize(13).TextColor(p.TextMuted)
				})
			})
		})
		return
	}
	ui.Column(c).Gap(1).Children(func() {
		label(c, p, primary.Label)
		bigValue(c, primary.Format(), 34, p.severityColor(primary.Severity))
	})
}

// deviceName is what the page calls the thing it watches: the name the plugin
// reports, or the snapshot's subtitle when it has none.
func deviceName(snap core.Snapshot) string {
	if snap.Title != "" {
		return snap.Title
	}
	if snap.Subtitle != "" {
		return snap.Subtitle
	}
	return "设备"
}

// inlineReading is one of the readings a combined card shows down its right:
// its label and its value on one line, the two ends of a row.
func (a *App) inlineReading(c *ui.Context, p palette, in *app.Instance, r core.Reading) {
	valueColor := p.Text
	if r.Severity != core.SeverityNone {
		valueColor = p.severityColor(r.Severity)
	}
	ui.Row(c).Gap(8).AlignItems(ui.Baseline).Children(func() {
		ui.Text(c, r.Label).FontSize(11).TextColor(p.TextMuted).NoWrap().MaxLines(1).Grow(1)
		ui.Text(c, r.Format()).FontSize(15).Bold().TextColor(valueColor).NoWrap().MaxLines(1)
	})
}

// heroEmpty is what a source with nothing to report shows: why, and what to
// do about it.
func (a *App) heroEmpty(c *ui.Context, p palette, snap core.Snapshot) {
	card(c, p).Shrink(0).Children(func() {
		text := snap.Subtitle
		if text == "" {
			text = snap.Status.String()
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Size(16, 16).Label("等待数据")
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "还没有数据").FontSize(13).Bold()
				ui.Text(c, text).FontSize(11).TextColor(p.TextMuted).Wrap()
			})
		})
	})
}

// pickPrimary chooses the reading the page leads with: the one the plugin
// names, or the most severe.
func pickPrimary(snap core.Snapshot) (core.Reading, bool) {
	return snap.Lead()
}

// readingCard is one reading as a card: its label and its trend across the top,
// its value under them, and whatever explains the value at the foot. Every card
// is the same height, so a row of them is a row.
//
// lead marks the reading the plugin named: it is said larger, and its value is
// not followed by a second line saying the same thing.
func (a *App) readingCard(c *ui.Context, p palette, in *app.Instance, r core.Reading, lead bool) ui.Element {
	valueColor := p.Text
	if r.Severity != core.SeverityNone {
		valueColor = p.severityColor(r.Severity)
	}
	size := float32(22)
	if lead {
		size = 28
	}
	box := card(c, p).Gap(2).Height(ReadingCardH).MaxHeight(ReadingCardH).Children(func() {
		// The label and the trend share the card's top row, the trend at its
		// right, so the eye finds the shape of a value in the same place on
		// every card.
		ui.Row(c).Gap(8).AlignItems(ui.Start).MinHeight(trendHeight).Children(func() {
			ui.Text(c, r.Label).FontSize(11).TextColor(p.TextMuted).
				NoWrap().MaxLines(1).Grow(1)
			a.trend(c, p, in, r, trendWidth)
		})
		// A value of words is set as the same size as a number: a card of
		// "无线" beside a card of "1200" is one row of cards, not two.
		ui.Text(c, r.Format()).FontSize(size).Bold().TextColor(valueColor).Wrap().MaxLines(2)
		if r.Detail != "" {
			ui.Text(c, r.Detail).FontSize(11).TextColor(p.TextMuted).NoWrap().MaxLines(1)
		}
	})
	return box
}

// trend draws the recent values of a reading as a sparkline in a box of the
// size every card keeps for it. A reading without a history worth drawing
// leaves the box empty, so that the room stays where the other cards have it.
func (a *App) trend(c *ui.Context, p palette, in *app.Instance, r core.Reading, width float32) ui.Element {
	box := ui.Box(c).Width(width).Height(trendHeight)
	if history := in.History(r.ID); varies(history) {
		box.Draw(func(pt *ui.Painter, rect ui.Rect) {
			drawSparkline(pt, rect, history, quietColor(p, r.Severity))
		})
	}
	return box
}

func historyWindow(in *app.Instance, readingID string) string {
	n := len(in.History(readingID))
	seconds := intervalSeconds(in)
	if n <= 1 || seconds <= 0 {
		return "一小会儿"
	}
	total := time.Duration(n*seconds) * time.Second
	switch {
	case total < time.Minute:
		return fmt.Sprintf("%d 秒", int(total.Seconds()))
	case total < time.Hour:
		return fmt.Sprintf("%d 分钟", int(total.Minutes()))
	default:
		return fmt.Sprintf("%d 小时", int(total.Hours()))
	}
}

// intervalSeconds is how often an instance polls, for the history label.
func intervalSeconds(in *app.Instance) int {
	switch in.Manifest().ID {
	case "mouse":
		return 3
	case "system":
		return 2
	}
	return 5
}

// quietColor is a reading's color for drawing: readings without a severity
// of their own, as a trend rather than a level, are drawn faintly.
func quietColor(p palette, s core.Severity) ui.Color {
	if s == core.SeverityNone {
		return p.TextMuted
	}
	return p.severityColor(s)
}

func offsetY(p *ui.Point) int {
	if p == nil {
		return -999
	}
	return int(p.Y)
}
