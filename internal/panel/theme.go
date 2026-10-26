package panel

import (
	"image/color"

	"github.com/egoist/mygo/ui"

	"seeall/internal/core"
)

// The panel's spacing is one scale, so that every edge on a page lines up with
// the edges on the pages beside it: the title bar, the tabs, the cards and the
// settings all start at the same gutter and end at the same one.
const (
	// Gutter is the room between the panel's edges and what it shows.
	Gutter = 14
	// CardGap is the room between two cards, and above and below a card.
	CardGap = 10
	// cardPadX and cardPadY are the room a card leaves inside itself.
	cardPadX, cardPadY = 14, 12
	// cardRadius rounds cards; controlRadius rounds the controls on them.
	cardRadius    = 12
	controlRadius = 8
)

// palette is SeeAll's look: one surface, one accent, and the three states
// that the whole app uses for severity. It follows the desktop's light or
// dark appearance, so the panel looks like part of the system.
type palette struct {
	*ui.Theme

	// Canvas fills the panel behind the cards.
	Card ui.Color
	// CardBorder outlines a card, faintly.
	CardBorder ui.Color
	// Dot is the sidebar's status dot when nothing is wrong.
	Dot ui.Color
	// OK, Warn, Critical and Offline color readings by severity.
	OK       ui.Color
	Warn     ui.Color
	Critical ui.Color
	Offline  ui.Color
	// Track is the unfilled part of a ring or a bar.
	Track ui.Color
}

// paletteFor builds the look for the window's appearance.
func paletteFor(c *ui.Context) palette {
	base := c.Theme()
	p := palette{Theme: base}
	if base.Dark {
		p.Card = ui.Hex("#202024")
		p.CardBorder = ui.Hex("#2e2e34")
		p.Track = ui.Hex("#2d2d33")
		p.OK = ui.Hex("#34d399")
		p.Warn = ui.Hex("#fbbf24")
		p.Critical = ui.Hex("#f87171")
		p.Offline = ui.Hex("#71717a")
	} else {
		p.Card = ui.Hex("#ffffff")
		p.CardBorder = ui.Hex("#e6e6ea")
		p.Track = ui.Hex("#e8e8ec")
		p.OK = ui.Hex("#0f9d63")
		p.Warn = ui.Hex("#b45309")
		p.Critical = ui.Hex("#dc2626")
		p.Offline = ui.Hex("#a1a1aa")
	}
	p.Dot = p.OK
	newTheme := *base
	newTheme.Accent = accentFor(base.Dark)
	newTheme.Radius = 8
	newTheme.Spacing = 4
	c.SetTheme(&newTheme)
	p.Theme = &newTheme
	return p
}

func accentFor(dark bool) ui.Color {
	if dark {
		return ui.Hex("#2dd4bf")
	}
	return ui.Hex("#0d9488")
}

// severityColor colors a reading.
func (p palette) severityColor(s core.Severity) ui.Color {
	switch s {
	case core.SeverityOK:
		return p.OK
	case core.SeverityWarn:
		return p.Warn
	case core.SeverityCritical:
		return p.Critical
	case core.SeverityOffline:
		return p.Offline
	case core.SeverityInfo:
		return p.Accent
	default:
		return p.Text
	}
}

// statusColor colors a source's dot.
func (p palette) statusColor(s core.Status) ui.Color {
	switch s {
	case core.StatusOK:
		return p.OK
	case core.StatusWarn:
		return p.Warn
	case core.StatusError:
		return p.Critical
	case core.StatusOffline:
		return p.Offline
	default:
		return p.TextMuted
	}
}

// inputAccent is the accent as an image/color, for the tray icon.
func inputAccent(dark bool) color.NRGBA {
	c := accentFor(dark)
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255}
}
