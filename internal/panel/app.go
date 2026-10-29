package panel

import (
	"sort"
	"time"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/plugins/mole"
)

// App is the panel's state. The view is a function of it, and it changes only
// on the main thread, as MyGo requires.
type App struct {
	// Host runs the plugins.
	Host *app.Host
	// Select is called when the panel changes page: the app remembers the
	// choice for the tray menu.
	Select func(id string)
	// Size is the window's size, as the app worked it out for its display.
	Size Metrics

	// selected is the page the panel shows: a monitor, or SettingsPage. The
	// dashboard itself is one page, so it only says whether the settings are
	// open and which monitor the tray menu is about.
	selected string
	// drafts keeps each settings field's value while it is being typed, so
	// that a half-typed host name does not restart a poller.
	drafts map[string]string

	// Opacity is how solid the settings page is drawing the panel, from 0 to
	// 1. A drag changes it while the pointer is down; OnOpacity is what makes
	// the window follow, and the file is written once the pointer is up.
	Opacity float64
	// OnOpacity is called while the slider moves, so the window fades as the
	// user drags rather than when they let go.
	OnOpacity func(float64)
	// opacityDirty is set while a drag has changed the value but not saved it.
	opacityDirty bool

	// confirm is the Mole command waiting for its second click, and when it
	// was asked for. The panel keeps it rather than the plugin because a
	// confirmation is a thing the user is doing, not a thing Mole is.
	confirm   mole.Action
	confirmAt time.Time
}

// New makes the panel's state.
func New(host *app.Host) *App {
	return &App{
		Host:      host,
		selected:  host.LastInstance(),
		drafts:    map[string]string{},
		Size:      Fit(1920, 1040, 1),
		Opacity:   host.PanelOpacity(),
		OnOpacity: func(float64) {},
	}
}

// SyncOpacity reads how solid the panel should be from the host, which is what
// the app calls once the settings file has been read: the panel is made before
// the file, so its value is the default until then.
func (a *App) SyncOpacity() {
	a.Opacity = a.Host.PanelOpacity()
	a.OnOpacity(a.Opacity)
}

// SetSize changes the panel's size, as another display asks for.
func (a *App) SetSize(m Metrics) { a.Size = m }

// Selected returns the page the panel shows: the settings, or a monitor.
func (a *App) Selected() string {
	if a.selected == "" {
		if instances := a.Host.Instances(); len(instances) > 0 {
			return instances[0].ID
		}
		return SettingsPage
	}
	return a.selected
}

// SetSelected shows another page: the settings, a monitor, or a plugin's own
// page. The panel remembers the monitor it was on, so that leaving the settings
// comes back to the machine rather than to a page nobody chose.
func (a *App) SetSelected(id string) {
	a.selected = id
	if id != SettingsPage && !a.standalone(id) {
		a.Host.SetLastInstance(id)
	}
	if a.Select != nil {
		a.Select(id)
	}
}

// standalone says whether an ID names a plugin's own page rather than a
// monitor: those are not what "back" returns to.
func (a *App) standalone(id string) bool {
	in := a.Host.Instance(id)
	return in != nil && in.Manifest().Standalone
}

// View builds the panel: the machine's dashboard, or the settings it opens.
func (a *App) View(c *ui.Context) {
	a.handleKeys(c)
	a.dashboard(c)
}

func (a *App) handleKeys(c *ui.Context) {
	if c.Shortcut(ui.Cmd, ui.KeyR) || c.Shortcut(0, ui.KeyF5) {
		a.Host.Refresh()
	}
}

// sorted puts the monitors in a stable order: by kind, then by name, so the
// page does not reorder itself as snapshots arrive.
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

// emptyDetail is the page for a panel with no monitors at all.
func (a *App) emptyDetail(c *ui.Context, p palette) {
	ui.Column(c).Fill().Center().Gap(8).Padding(24).Children(func() {
		ui.Icon(c, iconPlus).FontSize(28).TextColor(p.TextMuted)
		ui.Text(c, "没有监控项").FontSize(13).Bold()
		ui.Text(c, "到设置里加一个").FontSize(11).TextColor(p.TextMuted)
	})
}
