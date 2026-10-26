package panel

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
)

// maxTiles caps the readings a page shows as tiles. A plugin reporting a
// dozen of them would push the page past the window.
const maxTiles = 6

// detail is a monitor's page, laid out as a dashboard: the value that
// matters across the top as a hero card, the other readings as tiles under it
// two to a row, and what the source last did pinned at the foot.
func (a *App) detail(c *ui.Context, p palette, in *app.Instance) {
	m := in.Manifest()
	snap := in.Snapshot()

	ui.Column(c).Fill().Children(func() {
		a.header(c, p, in.Name, snap.Subtitle, snap, true)
		// The cards are centered in the room a short page leaves: a page
		// with two readings looks laid out rather than half empty.
		ui.Scroll(c).Grow(1).Shrink(1).Basis(0).Justify(ui.Center).
			Padding(18, 2, 18, 12).Gap(12).Children(func() {
			a.hero(c, p, in, snap)
			a.readings(c, p, in, snap)
		})
		ui.Row(c).Shrink(0).Padding(18, 0, 18, 14).Gap(8).AlignItems(ui.Center).Children(func() {
			if m.Experimental {
				badge(c, "实验", p.Warn)
			}
			a.footer(c, p, in, snap)
		})
	})
}

// hero is the big card: the ring for a percentage, the number for anything
// else, with its trend beside it.
//
// It keeps its natural height. A card that grew its box while its ring kept
// its size would be a border with a ring spilling out of it.
func (a *App) hero(c *ui.Context, p palette, in *app.Instance, snap core.Snapshot) {
	primary, ok := pickPrimary(snap)
	if !ok {
		a.heroEmpty(c, p, snap)
		return
	}

	card(c, p).Shrink(0).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Baseline).Children(func() {
			label(c, p, primary.Label)
			ui.Spacer(c)
			if primary.Detail != "" {
				ui.Text(c, primary.Detail).FontSize(11).TextColor(p.TextMuted).NoWrap()
			}
		})
		ui.Row(c).AlignItems(ui.Center).Gap(18).Children(func() {
			if primary.Unit == core.UnitPercent {
				ui.Box(c).Size(120, 120).
					Draw(func(pt *ui.Painter, r ui.Rect) {
						drawRing(pt, r, p.Track, clamp01(primary.Value/100),
							p.severityColor(primary.Severity), 11)
					}).Children(func() {
					ui.Column(c).Fill().Center().Children(func() {
						ui.Row(c).AlignItems(ui.Baseline).Gap(1).Children(func() {
							bigValue(c, core.FormatNumber(primary.Value), 36,
								p.severityColor(primary.Severity))
							ui.Text(c, "%").FontSize(15).TextColor(p.TextMuted)
						})
					})
				})
			} else {
				bigValue(c, primary.Format(), 44, p.severityColor(primary.Severity))
			}

			ui.Column(c).Grow(1).Basis(0).Gap(6).Children(func() {
				if caption := heroCaption(primary); caption != "" {
					ui.Text(c, caption).FontSize(13).Wrap().
						TextColor(p.severityColor(primary.Severity))
				}
				if history := in.History(primary.ID); varies(history) {
					sparkline(c, p, history, quietColor(p, primary.Severity), 48)
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

// readings shows every reading but the primary one as a tile, two to a row:
// a dashboard rather than a form.
func (a *App) readings(c *ui.Context, p palette, in *app.Instance, snap core.Snapshot) {
	primary, _ := pickPrimary(snap)
	var rest []core.Reading
	for _, r := range snap.Readings {
		if r.ID != primary.ID {
			rest = append(rest, r)
		}
	}
	if len(rest) > maxTiles {
		rest = rest[:maxTiles]
	}
	if len(rest) == 0 {
		return
	}
	ui.Grid(c).Columns(2).Gap(10).Children(func() {
		for _, r := range rest {
			a.readingTile(c, p, in, r)
		}
	})
}

// readingTile is one reading as a small card: its label, its value, and
// whatever else says the most about it.
func (a *App) readingTile(c *ui.Context, p palette, in *app.Instance, r core.Reading) {
	valueColor := p.Text
	if r.Severity != core.SeverityNone {
		valueColor = p.severityColor(r.Severity)
	}
	card(c, p).Padding(10, 8).Gap(3).MinHeight(76).Children(func() {
		ui.Text(c, r.Label).FontSize(11).TextColor(p.TextMuted).NoWrap().MaxLines(1)
		if r.Text != "" {
			// A reading whose value is words: the words are the value, with
			// the detail under them.
			ui.Text(c, r.Text).FontSize(13).Bold().Wrap().TextColor(valueColor)
			if r.Detail != "" {
				ui.Text(c, r.Detail).FontSize(11).TextColor(p.TextMuted).NoWrap().MaxLines(1)
			}
			return
		}
		ui.Row(c).AlignItems(ui.Baseline).Gap(6).Children(func() {
			ui.Text(c, r.Format()).FontSize(19).Bold().NoWrap().TextColor(valueColor)
			if r.Detail != "" {
				ui.Text(c, r.Detail).FontSize(11).TextColor(p.TextMuted).
					Grow(1).NoWrap().MaxLines(1)
			}
		})
		if history := in.History(r.ID); varies(history) {
			sparkline(c, p, history, quietColor(p, r.Severity), 22)
		}
	})
}

// varies reports whether a history is worth a sparkline: enough points, and
// something other than a flat line.
func varies(values []float64) bool {
	if len(values) < 4 {
		return false
	}
	lo, hi := values[0], values[0]
	for _, v := range values {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return hi-lo > 1e-9
}

// footer says when the source last reported, and offers a refresh.
func (a *App) footer(c *ui.Context, p palette, in *app.Instance, snap core.Snapshot) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, updatedText(snap)).FontSize(11).TextColor(p.TextMuted).Grow(1).NoWrap()
		if ui.Button(c, "刷新").Label("刷新").Clicked() {
			a.Host.RefreshOne(in.ID)
		}
	})
}

func updatedText(snap core.Snapshot) string {
	if snap.At.IsZero() {
		return "还没有数据"
	}
	return core.FormatDuration(time.Since(snap.At)) + "更新"
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
