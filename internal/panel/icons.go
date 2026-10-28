package panel

import "github.com/egoist/mygo/ui"

// The panel's icons are inline SVG paths, drawn in the text color by
// ui.Icon, so they follow the theme without a second asset pipeline.

const (
	logoSVG    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3.2"/><path d="M12 2.5v3.2M12 18.3v3.2M2.5 12h3.2M18.3 12h3.2"/><path d="M5.2 5.2l2.3 2.3M16.5 16.5l2.3 2.3M18.8 5.2l-2.3 2.3M7.5 16.5l-2.3 2.3"/></svg>`
	mouseSVG   = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="2" width="12" height="20" rx="6"/><path d="M12 2v6"/></svg>`
	cpuSVG     = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="5" y="5" width="14" height="14" rx="2.5"/><rect x="9" y="9" width="6" height="6" rx="1"/><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3"/></svg>`
	gearSVG    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3.2"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.7 1.7 0 0 0-1.87-.34 1.7 1.7 0 0 0-1.03 1.56V21a2 2 0 1 1-4 0v-.1A1.7 1.7 0 0 0 9 19.4a1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-1.56-1.03H3a2 2 0 1 1 0-4h.1A1.7 1.7 0 0 0 4.6 9a1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-1.56V3a2 2 0 1 1 4 0v.1A1.7 1.7 0 0 0 15 4.6a1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.7 1.7 0 0 0 19.4 9v.1a1.7 1.7 0 0 0 1.56 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/></svg>`
	refreshSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M20 11a8 8 0 1 0-2.3 6"/><path d="M20 4v7h-7"/></svg>`
	plusSVG    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>`
	closeSVG   = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M6 6l12 12M18 6L6 18"/></svg>`
	// The dashboard's cards are named by what they measure rather than by a
	// word, so each of the six parts of a machine has a mark of its own.
	gpuSVG     = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="6.5" width="19" height="11" rx="2"/><circle cx="8.5" cy="12" r="2.3"/><circle cx="15.5" cy="12" r="2.3"/></svg>`
	memSVG     = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="7" width="19" height="7.5" rx="1.2"/><path d="M2.5 14.5v2.5M6.5 14.5v2.5M10.5 14.5v2.5M14.5 14.5v2.5M18.5 14.5v2.5M21.5 14.5v2.5"/></svg>`
	diskSVG    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6.5C4 4.8 7.6 3.5 12 3.5s8 1.3 8 3V17c0 1.7-3.6 3-8 3s-8-1.3-8-3z"/><path d="M4 6.5c0 1.7 3.6 3 8 3s8-1.3 8-3"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/></svg>`
	netSVG     = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="8.5"/><path d="M3.5 12h17"/><path d="M12 3.5c2.4 2.4 3.8 5.4 3.8 8.5s-1.4 6.1-3.8 8.5c-2.4-2.4-3.8-5.4-3.8-8.5s1.4-6.1 3.8-8.5z"/></svg>`
	tempSVG    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13.9V5.5a2 2 0 1 1 4 0v8.4a4.5 4.5 0 1 1-4 0z"/><path d="M12 9.5v6"/></svg>`
	batterySVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="7.5" width="16.5" height="9" rx="2.2"/><path d="M21.5 10.8v2.4"/><path d="M5.5 10.8v2.4M8.5 10.8v2.4M11.5 10.8v2.4"/></svg>`
	gaugeSVG   = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="8.5"/><path d="M12 12l4.2-3"/></svg>`
)

var (
	iconLogo    = ui.MustParseSVG([]byte(logoSVG))
	iconMouse   = ui.MustParseSVG([]byte(mouseSVG))
	iconCPU     = ui.MustParseSVG([]byte(cpuSVG))
	iconGear    = ui.MustParseSVG([]byte(gearSVG))
	iconRefresh = ui.MustParseSVG([]byte(refreshSVG))
	iconPlus    = ui.MustParseSVG([]byte(plusSVG))
	iconClose   = ui.MustParseSVG([]byte(closeSVG))
	iconGPU     = ui.MustParseSVG([]byte(gpuSVG))
	iconMem     = ui.MustParseSVG([]byte(memSVG))
	iconDisk    = ui.MustParseSVG([]byte(diskSVG))
	iconNet     = ui.MustParseSVG([]byte(netSVG))
	iconTemp    = ui.MustParseSVG([]byte(tempSVG))
	iconBattery = ui.MustParseSVG([]byte(batterySVG))
	iconGauge   = ui.MustParseSVG([]byte(gaugeSVG))
)

// iconFor maps a manifest's icon name to the SVG it means.
func iconFor(name string) *ui.SVG {
	switch name {
	case "mouse":
		return iconMouse
	case "cpu":
		return iconCPU
	}
	return iconCPU
}
