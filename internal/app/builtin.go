package app

import (
	"seeall/internal/plugin"
	"seeall/internal/plugins/mole"
	"seeall/internal/plugins/mouse"
	"seeall/internal/plugins/system"
)

// Builtin returns the plugins SeeAll ships, in the order the sidebar shows
// their kinds.
//
// Adding one is the whole extension point: a package that implements
// plugin.Plugin goes in the slice, and the app gives it a page, a settings
// form, persistence and a tray entry.
func Builtin() []plugin.Plugin {
	return []plugin.Plugin{
		mouse.Plugin{},
		system.Plugin{},
		mole.Plugin{},
	}
}
