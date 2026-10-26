// Package panel draws the SeeAll window: a title bar, the tabs that choose a
// monitor, its page, and a status bar.
package panel

import (
	"github.com/egoist/mygo/ui"

	"seeall/internal/core"
)

// SettingsPage is the row that shows the settings instead of a monitor. The
// tray menu uses it to open them.
const SettingsPage = "@settings"

// OpenLog opens the log file, as the status bar's button does. The app sets
// it once it knows where the file is.
var OpenLog = func() {}

func openLog() { OpenLog() }

// Content returns the window's content for a panel: the view MyGo calls to
// build each frame.
func Content(a *App) *ui.Content {
	return ui.View(a.View)
}

// Headline picks the one reading that says the most about a monitor: the
// summary the tray menu and the tabs show.
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
