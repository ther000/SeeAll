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

// detail is a monitor's page: the reading that matters in the hero card at the
// top, and every other reading as a card of its own under it.
func (a *App) detail(c *ui.Context, p palette, in *app.Instance) {
	m := in.Manifest()
	snap := in.Snapshot()

	ui.Scroll(c).Fill().Padding(14, 4, 14, 10).Gap(10).Children(func() {
		a.hero(c, p, in, snap)
		a.readings(c, p, in, snap)
		if m.Experimental {
			ui.Row(c).Gap(8).Children(func() { badge(c, "实验", p.Warn) })
		}
	})
}

// hero is the card the page leads with: the ring for a percentage, the value
// for anything else, and its trend beside it.
func (a *App) hero(c *ui.Context, p palette, in *app.Instance, snap core.Snapshot) {
	primary, ok := pickPrimary(snap)
	if !ok {
		a.heroEmpty(c, p, snap)
		return
	}

	card(c, p).Shrink(0).Padding(14, 12).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Baseline).Children(func() {
			label(c, p, primary.Label)
			ui.Spacer(c)
			if primary.Detail != "" {
				ui.Text(c, primary.Detail).FontSize(11).TextColor(p.TextMuted).NoWrap()
			}
		})
		ui.Row(c).AlignItems(ui.Center).Gap(14).Children(func() {
			if primary.Unit == core.UnitPercent {
				ui.Box(c).Size(118, 118).
					Draw(func(pt *ui.Painter, r ui.Rect) {
						drawRing(pt, r, p.Track, clamp01(primary.Value/100),
							p.severityColor(primary.Severity), 11)
					}).Children(func() {
					ui.Column(c).Fill().Center().Children(func() {
						ui.Row(c).AlignItems(ui.Baseline).Gap(1).Children(func() {
							bigValue(c, core.FormatNumber(primary.Value), 36,
								p.severityColor(primary.Severity))
							ui.Text(c, "%").FontSize(14).TextColor(p.TextMuted)
						})
					})
				})
			} else {
				bigValue(c, primary.Format(), 38, p.severityColor(primary.Severity))
			}

			ui.Column(c).Grow(1).Basis(0).Gap(6).Children(func() {
				if caption := heroCaption(primary); caption != "" {
					ui.Text(c, caption).FontSize(13).Wrap().
						TextColor(p.severityColor(primary.Severity))
				}
				if history := in.History(primary.ID); varies(history) {
					sparkline(c, p, history, quietColor(p, primary.Severity), 44)
					label(c, p, "最近 "+historyWindow(in, primary.ID))
				}
			})
		})
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

// readings lays every reading but the primary one out as a card, two to a row.
// A page that fills up with them scrolls rather than folding any away: what a
// monitor reports is the whole point of its page.
func (a *App) readings(c *ui.Context, p palette, in *app.Instance, snap core.Snapshot) {
	primary, _ := pickPrimary(snap)
	var rest []core.Reading
	for _, r := range snap.Readings {
		if r.ID != primary.ID {
			rest = append(rest, r)
		}
	}
	if len(rest) == 0 {
		return
	}
	ui.Grid(c).Columns(cardsPerRow).Gap(10).Children(func() {
		for _, r := range rest {
			a.readingCard(c, p, in, r)
		}
	})
}

// readingCard is one reading as a card: its label, its value in the size the
// value deserves, whatever explains it, and its trend under them.
func (a *App) readingCard(c *ui.Context, p palette, in *app.Instance, r core.Reading) {
	valueColor := p.Text
	if r.Severity != core.SeverityNone {
		valueColor = p.severityColor(r.Severity)
	}
	card(c, p).Padding(13, 11).Gap(2).MinHeight(96).Children(func() {
		ui.Text(c, r.Label).FontSize(11).TextColor(p.TextMuted).NoWrap().MaxLines(1)
		// A value of words is set as the same size as a number: a card of
		// "无线" beside a card of "1200" is one row of cards, not two.
		ui.Text(c, r.Format()).FontSize(22).Bold().TextColor(valueColor).Wrap().MaxLines(2)
		if r.Detail != "" {
			ui.Text(c, r.Detail).FontSize(11).TextColor(p.TextMuted).NoWrap().MaxLines(1)
		}
		if history := in.History(r.ID); varies(history) {
			ui.Spacer(c)
			sparkline(c, p, history, quietColor(p, r.Severity), 20)
		}
	})
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

// heroCaption is the word beside the ring: for a battery, what its level
// means; for anything else, nothing, since the reading already says what it
// is.
func heroCaption(r core.Reading) string {
	if r.ID != "battery" {
		return ""
	}
	switch {
	case r.Value <= 10:
		return "快没电了"
	case r.Value <= 30:
		return "该充电了"
	case r.Value <= 60:
		return "还够用"
	default:
		return "电量充足"
	}
}

// quietColor is a reading's color for drawing: readings without a severity
// of their own, as a trend rather than a level, are drawn faintly.
func quietColor(p palette, s core.Severity) ui.Color {
	if s == core.SeverityNone {
		return p.TextMuted
	}
	return p.severityColor(s)
}
