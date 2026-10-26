package panel

import (
	"fmt"
	"sort"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
	"seeall/internal/plugin"
)

// App is the panel's state. The view is a function of it, and it changes
// only on the main thread, as MyGo requires.
type App struct {
	// Host runs the plugins.
	Host *app.Host
	// Select is called when the user picks another row: the app remembers
	// the choice.
	Select func(id string)
	// Size is the window's size, and the sidebar's width in it, as the app
	// worked them out for its display.
	Size Metrics

	selected string
	drafts   map[string]string
	// confirmRemove is the instance whose remove button was pressed once.
	confirmRemove string
	// showAll lists every monitor on the overview page.
	showAll bool
}

// New makes the panel's state.
func New(host *app.Host) *App {
	return &App{
		Host:     host,
		selected: host.LastInstance(),
		drafts:   map[string]string{},
		Size:     Fit(1920, 1040),
	}
}

// SetSize changes the panel's size, as another display asks for.
func (a *App) SetSize(m Metrics) { a.Size = m }

// Selected returns the row the panel shows.
func (a *App) Selected() string {
	if a.selected == "" {
		if instances := a.Host.Instances(); len(instances) > 0 {
			return instances[0].ID
		}
		return SettingsPage
	}
	return a.selected
}

// SetSelected shows another row.
func (a *App) SetSelected(id string) {
	a.selected = id
	a.confirmRemove = ""
	if id != SettingsPage {
		a.Host.SetLastInstance(id)
	}
	if a.Select != nil {
		a.Select(id)
	}
}

// View builds the panel. It is the window's content.
func (a *App) View(c *ui.Context) {
	p := paletteFor(c)
	a.handleKeys(c)

	instances := a.Host.Instances()
	a.ensureSelection(instances)

	ui.Row(c).Fill().Background(p.Background).Children(func() {
		a.sidebar(c, p, instances)
		ui.Box(c).Width(1).FillHeight().Background(p.CardBorder)
		ui.Column(c).Grow(1).FillHeight().Justify(ui.Start).Clip().Children(func() {
			switch a.Selected() {
			case SettingsPage:
				a.settings(c, p, instances)
			default:
				if in := a.Host.Instance(a.Selected()); in != nil {
					a.detail(c, p, in)
				} else {
					a.emptyDetail(c, p)
				}
			}
		})
	})
}

// ensureSelection keeps the panel on a row that exists: a monitor the user
// removed, or the first launch, falls back to the first one.
func (a *App) ensureSelection(instances []*app.Instance) {
	if a.selected == SettingsPage {
		return
	}
	if a.selected != "" && a.Host.Instance(a.selected) != nil {
		return
	}
	if len(instances) > 0 {
		a.selected = instances[0].ID
		return
	}
	a.selected = SettingsPage
}

func (a *App) handleKeys(c *ui.Context) {
	if c.Shortcut(ui.Cmd, ui.KeyR) || c.Shortcut(0, ui.KeyF5) {
		a.Host.Refresh()
	}
	if c.Shortcut(0, ui.KeyEscape) {
		if a.confirmRemove != "" {
			a.confirmRemove = ""
		}
	}
}

// sidebar lists the monitors by kind, with settings at its foot.
func (a *App) sidebar(c *ui.Context, p palette, instances []*app.Instance) {
	ui.Column(c).Width(float32(a.Size.Sidebar)).FillHeight().Background(p.Card).
		BorderWidth(0, 0, 0, 1).BorderColor(p.CardBorder).Children(func() {
		ui.Row(c).Padding(14, 14, 12, 12).Gap(6).Children(func() {
			ui.Text(c, "SeeAll").FontSize(14).Bold().Grow(1)
			if ui.Button(c, "").Label("刷新").Tooltip("刷新全部（Ctrl+R）").Children(func() {
				ui.Icon(c, iconRefresh).FontSize(14)
			}).Clicked() {
				a.Host.Refresh()
			}
		})
		ui.Scroll(c).Grow(1).Padding(8, 0, 8, 0).Gap(1).Children(func() {
			var kind plugin.Kind
			for _, in := range a.sorted(instances) {
				m := in.Manifest()
				if m.Kind != kind {
					kind = m.Kind
					ui.Text(c, kind.String()).FontSize(10).TextColor(p.TextMuted).
						Padding(8, 8, 6, 6)
				}
				a.sidebarRow(c, p, in)
			}
			if len(instances) == 0 {
				ui.Text(c, "还没有监控项").FontSize(11).TextColor(p.TextMuted).Padding(8)
			}
		})
		ui.Box(c).Height(1).Background(p.CardBorder)
		a.settingsRow(c, p)
	})
}

// sorted puts the monitors in a stable order: by kind, then by name, so the
// sidebar does not jump as snapshots arrive.
func (a *App) sorted(instances []*app.Instance) []*app.Instance {
	out := append([]*app.Instance(nil), instances...)
	sort.SliceStable(out, func(i, j int) bool {
		ki, kj := out[i].Manifest().Kind.Order(), out[j].Manifest().Kind.Order()
		if ki != kj {
			return ki < kj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (a *App) sidebarRow(c *ui.Context, p palette, in *app.Instance) {
	selected := a.Selected() == in.ID
	snap := in.Snapshot()
	row := ui.Row(c).Key(in.ID).Padding(6, 8).Radius(7).Gap(7).Cursor(ui.CursorPointer)
	if selected {
		row.Background(p.Surface)
	} else if row.Hovered() {
		row.Background(p.SurfaceHover)
	}
	row.Children(func() {
		ui.Icon(c, iconFor(in.Manifest().Icon)).FontSize(13).TextColor(colorIf(selected, p.Text, p.TextMuted))
		ui.Column(c).Grow(1).Gap(0).Children(func() {
			ui.Text(c, in.Name).FontSize(12).NoWrap().MaxLines(1).
				TextColor(colorIf(selected, p.Text, p.Text))
			ui.Text(c, sidebarHint(snap)).FontSize(10).NoWrap().MaxLines(1).TextColor(p.TextMuted)
		})
		dot(c, p.statusColor(snap.Status), 6)
	})
	if row.Clicked() {
		a.SetSelected(in.ID)
	}
}

// sidebarHint is the second line of a sidebar row: the value that matters
// most, or what the source is waiting for.
func sidebarHint(snap core.Snapshot) string {
	if len(snap.Readings) == 0 {
		if snap.Subtitle != "" {
			return snap.Subtitle
		}
		return snap.Status.String()
	}
	best, ok := snap.Lead()
	if !ok {
		return snap.Status.String()
	}
	return best.Format()
}

func severityRank(s core.Severity) int {
	switch s {
	case core.SeverityCritical:
		return 0
	case core.SeverityWarn:
		return 1
	case core.SeverityOffline:
		return 2
	case core.SeverityOK:
		return 3
	}
	return 4
}

func (a *App) settingsRow(c *ui.Context, p palette) {
	selected := a.Selected() == SettingsPage
	row := ui.Row(c).Key(SettingsPage).Padding(10, 8).Gap(7).Cursor(ui.CursorPointer)
	if selected {
		row.Background(p.Surface)
	} else if row.Hovered() {
		row.Background(p.SurfaceHover)
	}
	row.Children(func() {
		ui.Icon(c, iconGear).FontSize(13).TextColor(colorIf(selected, p.Text, p.TextMuted))
		ui.Text(c, "设置").FontSize(12)
	})
	if row.Clicked() {
		a.SetSelected(SettingsPage)
	}
}

func colorIf(cond bool, yes, no ui.Color) ui.Color {
	if cond {
		return yes
	}
	return no
}

// emptyDetail is the page for a panel with no monitors at all.
func (a *App) emptyDetail(c *ui.Context, p palette) {
	ui.Column(c).Fill().Center().Gap(8).Padding(24).Children(func() {
		ui.Icon(c, iconPlus).FontSize(28).TextColor(p.TextMuted)
		ui.Text(c, "没有监控项").FontSize(13).Bold()
		ui.Text(c, "到设置里加一个").FontSize(11).TextColor(p.TextMuted)
		if ui.Button(c, "打开设置").Clicked() {
			a.SetSelected(SettingsPage)
		}
	})
}

// emptySnap is what a page that is not a monitor passes to header.
func emptySnap() core.Snapshot { return core.Snapshot{} }

// header is the title line every page shares.
func (a *App) header(c *ui.Context, p palette, title, subtitle string, snap core.Snapshot, showStatus bool) {
	ui.Row(c).Padding(16, 14, 12, 10).Gap(8).Children(func() {
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, title).FontSize(13).Bold().NoWrap().MaxLines(1)
			if subtitle != "" {
				ui.Text(c, subtitle).FontSize(11).TextColor(p.TextMuted).NoWrap().MaxLines(1)
			}
		})
		if showStatus {
			badge(c, snap.Status.String(), p.statusColor(snap.Status))
		}
	})
}

var _ = fmt.Sprintf
var _ = mygo.App
