package app

// The panel has two looks. The dashboard is the one it opens with: a health
// score across the top, a grid of cards under it and the programs behind the
// numbers at the bottom. The classic look is the one the panel had before it:
// a title bar, a row of tabs and a page of reading cards.
//
// The choice belongs to the user and lives in the settings file beside the
// opacity, so that switching looks does not switch monitors.
const (
	// ThemeDashboard is the health dashboard: the default.
	ThemeDashboard = "dashboard"
	// ThemeClassic is the title bar and the tabs.
	ThemeClassic = "classic"
)

// Theme returns the look the panel draws. A file that names neither, or
// something else entirely, gets the dashboard: that is what a new install
// shows, and a value nobody offers is a typo, not a preference.
func (h *Host) Theme() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.saved.UI.Theme == nil || *h.saved.UI.Theme != ThemeClassic {
		return ThemeDashboard
	}
	return ThemeClassic
}

// SetTheme remembers the look the user picked.
func (h *Host) SetTheme(theme string) {
	if theme != ThemeClassic {
		theme = ThemeDashboard
	}
	h.mu.Lock()
	if h.saved.UI.Theme != nil && *h.saved.UI.Theme == theme {
		h.mu.Unlock()
		return
	}
	h.saved.UI.Theme = &theme
	h.mu.Unlock()
	h.save()
	h.OnChange()
}
