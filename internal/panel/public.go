// Package panel draws the SeeAll window: the list of monitors on the left,
// the chosen monitor or the settings on the right.
package panel

import (
	"github.com/egoist/mygo/ui"

	"seeall/internal/core"
)

// SettingsPage is the sidebar row that shows the settings instead of a
// monitor. The tray menu uses it to open them.
const SettingsPage = "@settings"

// Content returns the window's content for a panel: the view MyGo calls to
// build each frame.
func Content(a *App) *ui.Content {
	return ui.View(a.View)
}

// Headline picks the one reading that says the most about a monitor: the
// summary the tray menu, the sidebar and the overview show.
func Headline(snap core.Snapshot) string {
	best, ok := snap.Lead()
	if !ok {
		return ""
	}
	value := best.Format()
	if value == "" {
		return ""
	}
	if best.Label == "" {
		return value
	}
	return best.Label + " " + value
}
