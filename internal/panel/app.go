package panel

import (
	"sort"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
)

// App is the panel's state. The view is a function of it, and it changes
// only on the main thread, as MyGo requires.
type App struct {
	// Host runs the plugins.
	Host *app.Host
	// Select is called when the user picks another tab: the app remembers
	// the choice.
	Select func(id string)
	// Size is the window's size, as the app worked it out for its display.
	Size Metrics

	// selected is the monitor the panel shows, or SettingsPage.
	selected string
	// tab is the index of the chosen tab: one per monitor in tab order, then
	// the settings. The tab list owns it between frames.
	tab int

	drafts map[string]string
	// confirmRemove is the instance whose remove button was pressed once.
	confirmRemove string
	// addChoice is the plugin the add card's select shows.
	addChoice string
}

// New makes the panel's state.
func New(host *app.Host) *App {
	return &App{
		Host:     host,
		selected: host.LastInstance(),
		drafts:   map[string]string{},
		Size:     Fit(1920, 1040, 1),
	}
}

// SetSize changes the panel's size, as another display asks for.
func (a *App) SetSize(m Metrics) { a.Size = m }

// Selected returns the monitor the panel shows.
func (a *App) Selected() string {
	if a.selected == "" {
		if instances := a.Host.Instances(); len(instances) > 0 {
			return instances[0].ID
		}
		return SettingsPage
	}
	return a.selected
}

// SetSelected shows another monitor.
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

// View builds the panel: a title bar, the tabs that choose what it shows, and
// the page.
func (a *App) View(c *ui.Context) {
	p := paletteFor(c)
	a.handleKeys(c)

	instances := a.Host.Instances()
	a.ensureSelection(instances)

	ui.Column(c).Fill().Background(p.Background).Children(func() {
		a.header(c, p)
		a.tabs(c, p, instances)
		ui.Column(c).Grow(1).Shrink(1).Basis(0).Justify(ui.Start).Clip().Children(func() {
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

// header is the title bar: what the panel is, and the refresh it offers from
// any page.
func (a *App) header(c *ui.Context, p palette) {
	// The logo and the page's cards share a left edge: the panel's gutter,
	// which the cards' own padding then insets their text from.
	ui.Row(c).Shrink(0).Padding(CardGap, Gutter).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconLogo).FontSize(17).TextColor(p.Accent)
		ui.Text(c, "SeeAll").FontSize(14).Bold().Grow(1)
		if ui.Button(c, "").Label("刷新").Description("刷新全部").Tooltip("刷新全部（Ctrl+R）").Children(func() {
			ui.Icon(c, iconRefresh).FontSize(15)
		}).Clicked() {
			a.Host.Refresh()
		}
		on := a.Selected() == SettingsPage
		if ui.Button(c, "").Label("打开设置").Description("打开设置").Tooltip("设置").
			Children(func() {
				ui.Icon(c, iconGear).FontSize(15).TextColor(colorIf(on, p.Accent, p.TextMuted))
			}).Clicked() {
			a.SetSelected(SettingsPage)
		}
	})
}

// tabs chooses what the page shows: one tab per monitor, then the settings.
func (a *App) tabs(c *ui.Context, p palette, instances []*app.Instance) {
	items := a.sorted(instances)
	a.syncTab(items)

	if len(items) == 0 {
		return
	}
	ui.Row(c).Shrink(0).Padding(0, Gutter, 0, Gutter).Children(func() {
		tabs := ui.TabsBase(c, &a.tab, len(items))
		// The settings are not a tab, so none of them is lit while they
		// show: a lit tab would claim a page that is not on screen.
		lit := a.Selected() != SettingsPage
		clicked := -1
		tabs.List.FillWidth().Gap(CardGap / 2).Children(func() {
			for i, in := range items {
				tab := a.tabButton(c, p, tabs.Tab(i), lit && i == a.tab,
					iconFor(in.Manifest().Icon), in.Name, in.Snapshot().Status)
				// A click chooses the tab even when it is the one the list
				// already holds, which is what makes a page opened from
				// outside the tabs — the settings — clickable back: the tab
				// list reports no change, since its choice never moved.
				if tab.Clicked() {
					clicked = i
				}
			}
		})
		switch {
		case clicked >= 0:
			a.SetSelected(items[clicked].ID)
		case tabs.List.Changed() && a.tab < len(items):
			// The arrows and Home and End choose without clicking.
			a.SetSelected(items[a.tab].ID)
		}
	})
}

// tabButton draws one tab: its icon, its name, and the dot that says whether
// the monitor behind it is fine, which the page cannot show for the tabs it is
// not on.
func (a *App) tabButton(c *ui.Context, p palette, tab ui.Element, on bool, icon *ui.SVG, name string, status core.Status) ui.Element {
	tab.Grow(1).Basis(0).Justify(ui.Center).Padding(7, 8).Radius(controlRadius).
		Gap(5).AlignItems(ui.Center).FocusRing(false)
	if on {
		tab.Background(p.Card).Border(1, p.CardBorder)
	} else if tab.Hovered() {
		tab.Background(p.SurfaceHover)
	}
	tab.Children(func() {
		ui.Icon(c, icon).FontSize(12).TextColor(colorIf(on, p.Accent, p.TextMuted))
		ui.Text(c, name).FontSize(12).NoWrap().MaxLines(1).Bold()
		if status != core.StatusOK {
			dot(c, p.statusColor(status), 6)
		}
	})
	return tab
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

// syncTab points the tab list at what the panel shows. The settings are not a
// tab, so they keep the tabs on the monitor shown last.
func (a *App) syncTab(items []*app.Instance) {
	if a.Selected() == SettingsPage {
		return
	}
	for i, in := range items {
		if in.ID == a.Selected() {
			a.tab = i
			return
		}
	}
	a.tab = 0
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

// sorted puts the monitors in a stable order: by kind, then by name, so the
// tabs do not jump as snapshots arrive.
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
