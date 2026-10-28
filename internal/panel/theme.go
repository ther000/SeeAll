package panel

import (
	"github.com/egoist/mygo/ui"

	"seeall/internal/core"
)

// The panel's spacing is one scale, so that every edge on the page lines up
// with the edges beside it: the header, the cards and the settings all start at
// the same gutter and end at the same one.
const (
	// Gutter is the room between the panel's edges and what it shows.
	Gutter = 14
	// CardGap is the room between two cards on the settings page.
	CardGap = 10
	// cardPadX and cardPadY are the room a card leaves inside itself.
	cardPadX, cardPadY = 14, 12
	// cardRadius rounds a settings card; controlRadius rounds the controls on
	// it.
	cardRadius    = 12
	controlRadius = 8
)

// palette is the look the settings page draws with: the dashboard's surfaces in
// the names the rows and cards of a settings page use. The dashboard itself
// draws with dash, which is the same colors under the names a card of readings
// uses.
type palette struct {
	*ui.Theme

	// Card fills a card, and CardBorder outlines it faintly.
	Card       ui.Color
	CardBorder ui.Color
	// OK, Warn, Critical and Offline color a reading by its severity.
	OK       ui.Color
	Warn     ui.Color
	Critical ui.Color
	Offline  ui.Color
	// Track is the unfilled part of a bar.
	Track ui.Color
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

// card is the panel's surface with a faint outline, at the one padding and
// radius every card on the settings page uses.
func card(c *ui.Context, p palette) ui.Element {
	return ui.Column(c).Padding(cardPadY, cardPadX).Gap(CardGap).Radius(cardRadius).
		Background(p.Card).Border(1, p.CardBorder)
}

// label is the small, muted text of a card's heading.
func label(c *ui.Context, p palette, text string) ui.Element {
	return ui.Text(c, text).FontSize(11).TextColor(p.TextMuted).NoWrap()
}

// hline is a hairline between two groups of content.
func hline(c *ui.Context, p palette) ui.Element {
	return ui.Box(c).Height(1).MarginY(2).Background(p.CardBorder)
}
