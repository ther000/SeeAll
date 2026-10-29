package panel

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
	"seeall/internal/molecli"
	"seeall/internal/plugins/mole"
)

// The Mole page is the one page that does something instead of watching
// something, so it is laid out as the list of things it can do rather than as
// cards of readings: what a cleanup would free, where the disk went, what the
// maintenance would apply, and what Mole cleaned before.
//
// Two of those commands delete files. The page treats them differently from the
// two that only read: a removing command is drawn in the warning's amber rather
// than the accent, it is offered only after a preview from this hour, and the
// click that runs it is the second click on that button. The plugin refuses the
// same way, so the rule holds whichever path the click came through.

// moleRows is how much of a list the page shows: six entries of the disk and
// four cleanups. A panel this size reads the top of a list, and the rest would
// only push the cards under it out of the window.
const (
	moleDiskRows = 6
	moleHistRows = 4
)

// moleConfirmLife is how long the second click stays available. A confirmation
// left standing overnight is not a confirmation of anything.
const moleConfirmLife = time.Minute

// moleAction is one card of the page: what it is called, the command that
// reads and the command that changes — either of which may be absent.
type moleAction struct {
	Title string
	Read  mole.Action
	Write mole.Action
	// ReadLabel and WriteLabel are the words on the two buttons;
	// ConfirmLabel is what the second click says, once the first has asked.
	ReadLabel, WriteLabel, ConfirmLabel string
}

// The page's four cards, in the order it shows them: the cleanup first,
// because freeing space is what Mole is for.
var (
	moleCleanCard    = moleAction{Title: "清理", Read: mole.ScanClean, Write: mole.Clean, ReadLabel: "预览", WriteLabel: "立即清理", ConfirmLabel: "确认清理"}
	moleDiskCard     = moleAction{Title: "磁盘占用", Read: mole.ScanDisk, ReadLabel: "扫描"}
	moleOptimizeCard = moleAction{Title: "优化", Read: mole.ScanOptimize, Write: mole.Optimize, ReadLabel: "预览", WriteLabel: "立即优化", ConfirmLabel: "确认优化"}
	moleHistoryCard  = moleAction{Title: "最近清理", Read: mole.ScanHistory, ReadLabel: "刷新"}
)

// molePage draws the page: the way back, then the command the user picks.
func (a *App) molePage(c *ui.Context, d dash, in *app.Instance) {
	m := moleOf(in)
	if m == nil {
		return
	}
	st := m.State()
	p := d.palette()
	a.moleHeader(c, d)
	a.dashPage(c, func() {
		ui.Scroll(c).Fill().Padding(CardGap, Gutter).Gap(CardGap).Children(func() {
			if !st.Ready {
				a.moleMissing(c, p, st)
				return
			}
			a.moleClean(c, d, p, m, st)
			a.moleDisk(c, d, p, m, st)
			a.moleOptimize(c, d, p, m, st)
			a.moleHistory(c, d, p, m, st)
		})
	})
}

// moleHeader is the page's heading: the way back to the dashboard, and what
// the page is.
func (a *App) moleHeader(c *ui.Context, d dash) {
	ui.Row(c).Shrink(0).Padding(8, dashGutter, 4, dashGutter).Gap(8).
		AlignItems(ui.Center).Children(func() {
		if dashIconButton(c, d, "返回", "返回", iconBack).Clicked() {
			a.dashBack()
		}
		ui.Text(c, "Mole").FontSize(20).Bold().Grow(1).NoWrap()
	})
}

// moleMissing is the page on a machine without Mole: what to install, and what
// the panel runs.
func (a *App) moleMissing(c *ui.Context, p palette, st mole.Data) {
	card(c, p).Children(func() {
		label(c, p, "没找到 Mole")
		ui.Text(c, "这一页跑的是 mo 命令。装好后它自己会重新检测；本机装的是 Mole CLI for Windows。").
			FontSize(12).TextColor(p.Text).Wrap()
		ui.Text(c, "scoop install mole  ·  https://github.com/tw93/Mole").
			FontSize(11.5).TextColor(p.TextMuted).Wrap()
	})
}

// moleClean is what a cleanup would free: the number the preview found, when
// it saw it, and the two buttons that read the disk and then clean it.
func (a *App) moleClean(c *ui.Context, d dash, p palette, m *mole.Instance, st mole.Data) {
	card(c, p).Children(func() {
		a.moleHead(c, d, p, m, st, moleCleanCard)
		ui.Row(c).FillWidth().AlignItems(ui.Baseline).Gap(8).Children(func() {
			size := "—"
			if st.Clean != nil && st.Clean.Size != "" {
				size = st.Clean.Size
			}
			ui.Text(c, size).FontSize(26).Bold().NoWrap().
				TextColor(moleSizeColor(p, st))
			if st.Clean != nil && st.Clean.Items > 0 {
				ui.Text(c, fmt.Sprintf("%d 项", st.Clean.Items)).FontSize(12).
					TextColor(p.TextMuted).NoWrap()
			}
		})
		for _, line := range cleanlines(st) {
			ui.Text(c, line).FontSize(11.5).TextColor(p.TextMuted).MaxLines(2)
		}
		moleResult(c, p, st, moleCleanCard)
	})
}

// cleanlines is what the cleanup card says under its number: what the last
// command was, when, and what kind of thing the preview found.
func cleanlines(st mole.Data) []string {
	if st.Clean == nil {
		return []string{"点「预览」看看能清出多少。清理会删掉缓存，重建要重新下载。"}
	}
	verb := "预览"
	if st.CleanRan {
		verb = "清理"
	}
	line := fmt.Sprintf("%s %s", st.CleanAt.Format("15:04"), verb)
	if st.Clean.Free != "" {
		line += " · 剩余 " + st.Clean.Free
	}
	out := []string{line}
	if names := groupNames(st.Clean.Groups, 3); names != "" {
		out = append(out, names)
	}
	return out
}

// moleSizeColor greys a number with nothing behind it: a preview that found no
// space is a fact, not a result.
func moleSizeColor(p palette, st mole.Data) ui.Color {
	if st.Clean == nil || st.Clean.Bytes <= 0 {
		return p.TextMuted
	}
	return p.Text
}

// groupNames joins the first few names of a preview's groups, so that the card
// says what kind of thing would go without listing a path for each.
func groupNames(groups []molecli.Group, max int) string {
	names := make([]string, 0, max)
	for _, g := range groups {
		if g.Name == "" || len(names) >= max {
			continue
		}
		names = append(names, g.Name)
	}
	return strings.Join(names, " · ")
}

// moleDisk is where the space went: the largest entries Mole found, as deep as
// the panel has room for.
func (a *App) moleDisk(c *ui.Context, d dash, p palette, m *mole.Instance, st mole.Data) {
	card(c, p).Children(func() {
		a.moleHead(c, d, p, m, st, moleDiskCard)
		if st.Disk == nil {
			ui.Text(c, "扫描一次，看看哪个文件夹最大。").
				FontSize(11.5).TextColor(p.TextMuted).Wrap()
			moleResult(c, p, st, moleDiskCard)
			return
		}
		for i, e := range st.Disk.Entries {
			if i >= moleDiskRows {
				break
			}
			// Mole marks two kinds of entry: places its cleanup already
			// covers (cleanable, from a CACHEDIR.TAG) and the known heavy
			// ones (insight). Either means "this is not your data".
			moleEntryRow(c, p, e.Name, core.FormatBytes(float64(e.Bytes)),
				e.Insight || e.Cleanable)
		}
		shown := min(len(st.Disk.Entries), moleDiskRows)
		line := fmt.Sprintf("%s 扫描 · 前 %d 项", st.DiskAt.Format("15:04"), shown)
		if st.Disk.Partial {
			line += " · 部分目录没数完"
		}
		ui.Text(c, line).FontSize(11.5).TextColor(p.TextMuted).MaxLines(1)
		moleResult(c, p, st, moleDiskCard)
	})
}

// moleEntryRow is one line of the disk list: what it is and how much it takes.
// Mole marks the places its cleanup covers, and they take the accent.
func moleEntryRow(c *ui.Context, p palette, name, size string, covered bool) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
		color := p.Text
		if covered {
			color = p.Accent
		}
		ui.Text(c, name).FontSize(12).TextColor(color).MaxLines(1).Grow(1)
		ui.Text(c, size).FontSize(12).TextColor(p.TextMuted).NoWrap().
			TextAlign(ui.End).Width(72)
	})
}

// moleOptimize is the maintenance tasks: what the last preview would apply,
// and what the last run applied.
func (a *App) moleOptimize(c *ui.Context, d dash, p palette, m *mole.Instance, st mole.Data) {
	card(c, p).Children(func() {
		a.moleHead(c, d, p, m, st, moleOptimizeCard)
		line := "刷新缓存、整理卷、检查系统更新与镜像，不动文件。"
		if st.Optimize != nil {
			when := st.OptAt.Format("15:04")
			if st.OptRan {
				line = fmt.Sprintf("%s 优化完成 · 应用 %d 项", when, st.Optimize.Applied)
				if st.Optimize.Issues > 0 {
					line += fmt.Sprintf(" · 发现 %d 个问题", st.Optimize.Issues)
				}
			} else {
				line = fmt.Sprintf("%s 预览 · 将优化 %d 项", when, st.Optimize.Applied)
			}
		}
		ui.Text(c, line).FontSize(11.5).TextColor(p.TextMuted).MaxLines(1)
		moleResult(c, p, st, moleOptimizeCard)
	})
}

// moleHistory is what Mole cleaned before: a few sessions from its own log,
// newest first.
func (a *App) moleHistory(c *ui.Context, d dash, p palette, m *mole.Instance, st mole.Data) {
	card(c, p).Children(func() {
		a.moleHead(c, d, p, m, st, moleHistoryCard)
		if st.History == nil || len(st.History.Sessions) == 0 {
			ui.Text(c, "Mole 的日志里还没有清理记录。").
				FontSize(11.5).TextColor(p.TextMuted).Wrap()
			moleResult(c, p, st, moleHistoryCard)
			return
		}
		for i, s := range st.History.Sessions {
			if i >= moleHistRows {
				break
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, s.Started.Format("01-02 15:04")).FontSize(12).
					TextColor(p.TextMuted).NoWrap().Width(82)
				ui.Text(c, commandWord(s.Command)).FontSize(12).Grow(1).MaxLines(1)
				ui.Text(c, fmt.Sprintf("%d 项", s.Count)).FontSize(12).
					TextColor(p.TextMuted).NoWrap().TextAlign(ui.End).Width(48)
			})
		}
		ui.Text(c, fmt.Sprintf("%s 读取 · %d 次记录", st.HistAt.Format("15:04"),
			len(st.History.Sessions))).FontSize(11.5).TextColor(p.TextMuted).MaxLines(1)
		moleResult(c, p, st, moleHistoryCard)
	})
}

// commandWord names a command the way the card's narrow column has room for.
func commandWord(command string) string {
	switch command {
	case "clean":
		return "清理"
	case "optimize":
		return "优化"
	case "purge":
		return "清项目"
	case "uninstall":
		return "卸载"
	case "":
		return "—"
	}
	return command
}

// moleHead is a card's first line: what it is, and what it can do. It is the
// one place the page decides whether a button is offered, whether a command's
// progress takes the buttons' place, and whether the removing button has
// turned into the confirmation.
func (a *App) moleHead(c *ui.Context, d dash, p palette, m *mole.Instance, st mole.Data, act moleAction) {
	busy := st.Running != nil
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, act.Title).FontSize(12).Bold().Grow(1).NoWrap()

		if busy && (st.Running.Action == act.Read || st.Running.Action == act.Write) {
			ui.Text(c, st.Running.Action.Running()).FontSize(11.5).
				TextColor(p.Accent).MaxLines(1)
			ui.Text(c, fmt.Sprintf("%ds", int(time.Since(st.Running.Started).Seconds()))).
				FontSize(11.5).TextColor(p.TextMuted).NoWrap()
			if moleButton(c, p, "取消", moleTonePlain).Clicked() {
				m.Cancel()
			}
			return
		}
		if busy {
			// One command at a time: every button waits, dimmed. The one
			// that deletes is drawn only if it would be offered anyway.
			if act.Read != "" {
				moleButton(c, p, act.ReadLabel, moleTonePlain).Disabled(true)
			}
			if act.Write != "" && a.mayRemove(st, act.Write) {
				moleButton(c, p, act.WriteLabel, moleToneDanger).Disabled(true)
			}
			return
		}
		if act.Read != "" {
			if moleButton(c, p, act.ReadLabel, moleTonePlain).Clicked() {
				a.runMole(c, m, act.Read)
			}
		}
		if act.Write == "" || !a.mayRemove(st, act.Write) {
			// Nothing to confirm: without a preview from this hour the
			// removing button is not offered at all, rather than offered
			// and then refused.
			return
		}
		if a.confirming(act.Write) {
			if moleButton(c, p, act.ConfirmLabel, moleToneDanger).Clicked() {
				a.confirm = ""
				a.runMole(c, m, act.Write)
			}
			if moleButton(c, p, "取消", moleTonePlain).Clicked() {
				a.confirm = ""
			}
			return
		}
		if moleButton(c, p, act.WriteLabel, moleToneDanger).Clicked() {
			// The first click only asks: the button under the pointer
			// becomes the confirmation, and nothing has run yet.
			a.confirm, a.confirmAt = act.Write, time.Now()
		}
	})
}

// mayRemove says whether a removing command may be offered: a preview of the
// same kind, from the last half hour, that no run has been started with yet, is
// what stands in for the list the user agreed to.
func (a *App) mayRemove(st mole.Data, write mole.Action) bool {
	switch write {
	case mole.Clean:
		return st.Clean != nil && !st.CleanRan && !st.CleanSpent &&
			time.Since(st.CleanAt) <= mole.PreviewFresh
	case mole.Optimize:
		return st.Optimize != nil && !st.OptRan && !st.OptSpent &&
			time.Since(st.OptAt) <= mole.PreviewFresh
	}
	return false
}

// confirming says whether an action is waiting for its second click.
func (a *App) confirming(write mole.Action) bool {
	return a.confirm == write && time.Since(a.confirmAt) <= moleConfirmLife
}

// moleResult is the line under a card: what the last command said, in the
// accent when it worked and in red when it did not.
func moleResult(c *ui.Context, p palette, st mole.Data, acts ...moleAction) {
	if st.Last == nil || st.Last.Note == "" {
		return
	}
	for _, act := range acts {
		if st.Last.Action != act.Read && st.Last.Action != act.Write {
			continue
		}
		color := p.OK
		if st.Last.Err {
			color = p.Critical
		}
		// Two lines: a failure says what happened and what to do next, and
		// Mole's own error is a sentence, not a word.
		ui.Text(c, st.Last.Note).FontSize(11.5).TextColor(color).MaxLines(2)
		return
	}
}

// runMole starts a command and keeps the panel honest about it: a refusal from
// the plugin — a preview that went stale between two frames — is told to the
// user rather than swallowed.
func (a *App) runMole(c *ui.Context, m *mole.Instance, act mole.Action) {
	a.confirm = ""
	if err := m.Run(act); err != nil {
		c.Toast(err.Error())
	}
}

// moleTone says how loud a button on this page is.
type moleTone int

const (
	// moleTonePlain is a button that reads: preview, scan, refresh, cancel.
	moleTonePlain moleTone = iota
	// moleToneDanger is one that deletes: amber, never the accent.
	moleToneDanger
)

// moleButton is one action on the Mole page. Its words are the whole button —
// the page has no room for an icon beside them — and the two tones are the two
// kinds of thing a click here can do.
func moleButton(c *ui.Context, p palette, text string, tone moleTone) ui.Element {
	b := ui.Button(c, text).Label(text).FontSize(12).Padding(5, 10).Radius(8)
	switch tone {
	case moleToneDanger:
		b.Background(ui.RGBA(217, 162, 110, 0.16)).
			Border(1, ui.RGBA(217, 162, 110, 0.45)).TextColor(p.Warn)
	default:
		b.Background(ui.RGBA(255, 255, 255, 0.08)).
			Border(1, ui.RGBA(255, 255, 255, 0.12)).TextColor(p.Text)
	}
	return b
}

// moleOf finds the plugin's own instance behind the one the host runs.
func moleOf(in *app.Instance) *mole.Instance {
	m, _ := in.Monitor().(*mole.Instance)
	return m
}

// ownInstance is the instance that draws a page of its own, if the app has
// one: the Mole plugin, whichever page the panel is on.
func (a *App) ownInstance() *app.Instance {
	for _, in := range a.Host.Instances() {
		if in.Manifest().Standalone {
			return in
		}
	}
	return nil
}

// ownPage is the same instance when the panel is showing its page.
func (a *App) ownPage() *app.Instance {
	in := a.ownInstance()
	if in == nil || a.Selected() != in.ID {
		return nil
	}
	return in
}
