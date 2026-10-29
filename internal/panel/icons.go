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
	// backSVG is the way out of a page: the arrow the settings lead back to
	// the machine by.
	backSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M14.5 5l-7 7 7 7"/></svg>`
	// moleSVG is Mole: a broom, because the page it opens is the one that
	// sweeps. It is drawn in the header beside the gear.
	moleSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M15.6 2.8l-5.4 9.4"/><path d="M3.2 21.2c1.6-2.1 2.6-4 3.1-5.6.4-1.2 1.5-1.9 2.7-1.7l2.8.5c1.2.2 2 1.2 2.1 2.4.2 1.6.9 3.4 2.2 5.4z"/><path d="M6.7 19.6l-.6 2M9.5 20.1l-.4 2M12.3 20.6l-.3 2"/></svg>`
	// The dashboard's cards are named by what they measure rather than by a
	// word, so each of the six parts of a machine has a mark of its own.
	gpuSVG     = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="6.5" width="19" height="11" rx="2"/><circle cx="8.5" cy="12" r="2.3"/><circle cx="15.5" cy="12" r="2.3"/></svg>`
	memSVG     = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="7" width="19" height="7.5" rx="1.2"/><path d="M2.5 14.5v2.5M6.5 14.5v2.5M10.5 14.5v2.5M14.5 14.5v2.5M18.5 14.5v2.5M21.5 14.5v2.5"/></svg>`
	diskSVG    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6.5C4 4.8 7.6 3.5 12 3.5s8 1.3 8 3V17c0 1.7-3.6 3-8 3s-8-1.3-8-3z"/><path d="M4 6.5c0 1.7 3.6 3 8 3s8-1.3 8-3"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/></svg>`
	netSVG     = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="8.5"/><path d="M3.5 12h17"/><path d="M12 3.5c2.4 2.4 3.8 5.4 3.8 8.5s-1.4 6.1-3.8 8.5c-2.4-2.4-3.8-5.4-3.8-8.5s1.4-6.1 3.8-8.5z"/></svg>`
	tempSVG    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13.9V5.5a2 2 0 1 1 4 0v8.4a4.5 4.5 0 1 1-4 0z"/><path d="M12 9.5v6"/></svg>`
	batterySVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="7.5" width="16.5" height="9" rx="2.2"/><path d="M21.5 10.8v2.4"/><path d="M5.5 10.8v2.4M8.5 10.8v2.4M11.5 10.8v2.4"/></svg>`
	gaugeSVG   = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="8.5"/><path d="M12 12l4.2-3"/></svg>`
	// The sky's shapes: the header leads with one of these, drawn in the color
	// the sky has rather than in the theme's text color. They share one cloud,
	// because four of the five skies are the same cloud with something
	// falling out of it.
	skySunSVG     = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="4"/><path d="M12 2.2v2.4M12 19.4v2.4M2.2 12h2.4M19.4 12h2.4M5.1 5.1l1.7 1.7M17.2 17.2l1.7 1.7M18.9 5.1l-1.7 1.7M6.8 17.2l-1.7 1.7"/></svg>`
	skyPartlySVG  = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="6.8" r="3"/><path d="M12 1.8v1.4M7 2.9l1 1M17 2.9l-1 1M5.6 6.8H7M17 6.8h1.4"/><path d="M6.2 21.4h9.4a3.1 3.1 0 0 0 .3-6.18 4.45 4.45 0 0 0-8.52-1.4A3.55 3.55 0 0 0 6.2 21.4z"/></svg>`
	skyCloudSVG   = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M6.5 18.5h10.3a3.6 3.6 0 0 0 .35-7.18 5.2 5.2 0 0 0-9.96-1.6A4.35 4.35 0 0 0 6.5 18.5z"/></svg>`
	skyFogSVG     = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M6.5 14.5h10.3a3.6 3.6 0 0 0 .35-7.18 5.2 5.2 0 0 0-9.96-1.6A4.35 4.35 0 0 0 6.5 14.5z"/><path d="M4.5 18h15M7.5 21.5h9"/></svg>`
	skyRainSVG    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M6.5 14.5h10.3a3.6 3.6 0 0 0 .35-7.18 5.2 5.2 0 0 0-9.96-1.6A4.35 4.35 0 0 0 6.5 14.5z"/><path d="M8.6 17.6l-.8 2.8M12.4 17.6l-.8 2.8M16.2 17.6l-.8 2.8"/></svg>`
	skySnowSVG    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M6.5 14.5h10.3a3.6 3.6 0 0 0 .35-7.18 5.2 5.2 0 0 0-9.96-1.6A4.35 4.35 0 0 0 6.5 14.5z"/><path d="M8.2 18.2v3M6.9 19.1l2.6 1.2M9.5 19.1l-2.6 1.2M15.8 18.2v3M14.5 19.1l2.6 1.2M17.1 19.1l-2.6 1.2"/></svg>`
	skyThunderSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M6.5 14.5h10.3a3.6 3.6 0 0 0 .35-7.18 5.2 5.2 0 0 0-9.96-1.6A4.35 4.35 0 0 0 6.5 14.5z"/><path d="M12.9 16.4l-2.6 3.6h3.3l-1.2 3"/></svg>`
)

var (
	iconLogo    = ui.MustParseSVG([]byte(logoSVG))
	iconMouse   = ui.MustParseSVG([]byte(mouseSVG))
	iconCPU     = ui.MustParseSVG([]byte(cpuSVG))
	iconGear    = ui.MustParseSVG([]byte(gearSVG))
	iconRefresh = ui.MustParseSVG([]byte(refreshSVG))
	iconPlus    = ui.MustParseSVG([]byte(plusSVG))
	iconBack    = ui.MustParseSVG([]byte(backSVG))
	iconMole    = ui.MustParseSVG([]byte(moleSVG))
	iconGPU     = ui.MustParseSVG([]byte(gpuSVG))
	iconMem     = ui.MustParseSVG([]byte(memSVG))
	iconDisk    = ui.MustParseSVG([]byte(diskSVG))
	iconNet     = ui.MustParseSVG([]byte(netSVG))
	iconTemp    = ui.MustParseSVG([]byte(tempSVG))
	iconBattery = ui.MustParseSVG([]byte(batterySVG))
	iconGauge   = ui.MustParseSVG([]byte(gaugeSVG))

	iconSkySun     = ui.MustParseSVG([]byte(skySunSVG))
	iconSkyPartly  = ui.MustParseSVG([]byte(skyPartlySVG))
	iconSkyCloud   = ui.MustParseSVG([]byte(skyCloudSVG))
	iconSkyFog     = ui.MustParseSVG([]byte(skyFogSVG))
	iconSkyRain    = ui.MustParseSVG([]byte(skyRainSVG))
	iconSkySnow    = ui.MustParseSVG([]byte(skySnowSVG))
	iconSkyThunder = ui.MustParseSVG([]byte(skyThunderSVG))
)

// weatherIcon maps the sky's kind to the shape the header draws before its
// temperature. A kind this panel does not know is drawn as a covered sky, which
// says the least of the seven.
func weatherIcon(kind string) *ui.SVG {
	switch kind {
	case "sun":
		return iconSkySun
	case "partly":
		return iconSkyPartly
	case "fog":
		return iconSkyFog
	case "rain":
		return iconSkyRain
	case "snow":
		return iconSkySnow
	case "thunder":
		return iconSkyThunder
	}
	return iconSkyCloud
}

// iconFor maps a manifest's icon name to the SVG it means.
func iconFor(name string) *ui.SVG {
	switch name {
	case "mouse":
		return iconMouse
	case "cpu":
		return iconCPU
	case "mole":
		return iconMole
	}
	return iconCPU
}
