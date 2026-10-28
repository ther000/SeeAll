package system

import (
	"testing"

	"seeall/internal/core"
)

// TestShortCPUNameKeepsTheModel checks the first pill in the dashboard's
// header: the model number, without the vendor's trademarks, the integrated
// graphics it happens to have, or the clock a BIOS appends.
func TestShortCPUNameKeepsTheModel(t *testing.T) {
	cases := map[string]string{
		"AMD Ryzen 7 6800H with Radeon Graphics": "Ryzen 7 6800H",
		"AMD Ryzen 7 6800H":                      "Ryzen 7 6800H",
		"AMD Ryzen 9 5950X 16-Core Processor":    "Ryzen 9 5950X 16-Core",
		"Intel(R) Core(TM) i7-1165G7 @ 2.80GHz":  "Core i7-1165G7",
		"12th Gen Intel(R) Core(TM) i7-12700H":   "12th Gen Intel Core i7-12700H",
		"Apple M2 Pro":                           "Apple M2 Pro",
		"":                                       "",
	}
	for raw, want := range cases {
		if got := shortCPUName(raw); got != want {
			t.Errorf("shortCPUName(%q) = %q, want %q", raw, got, want)
		}
	}
}

// TestOSPillSettlesTheFamilyFromTheBuild checks the third pill: Windows 11
// still calls itself "Windows 10 Pro" in the registry, so the build number is
// the only thing that can be believed about it.
func TestOSPillSettlesTheFamilyFromTheBuild(t *testing.T) {
	cases := []struct {
		product, display string
		build            int
		want             string
	}{
		{"Windows 10 Pro", "24H2", 26100, "Windows 11 24H2"},
		{"Windows 10 Pro", "22H2", 19045, "Windows 10 22H2"},
		{"Windows 11 Home", "23H2", 22631, "Windows 11 23H2"},
		{"Windows 10 Enterprise", "", 19044, "Windows 10"},
		{"Windows 10 Pro N", "21H2", 19044, "Windows 10 21H2"},
		{"", "", 0, ""},
	}
	for _, c := range cases {
		if got := osPill(c.product, c.display, c.build); got != c.want {
			t.Errorf("osPill(%q, %q, %d) = %q, want %q", c.product, c.display, c.build, got, c.want)
		}
	}
}

// TestCPUClockIsWrittenInTheUnitItReads checks the clock badge: gigahertz where
// a desktop processor lives, megahertz for a machine idling under one, and no
// badge at all when nobody reported a clock.
func TestCPUClockIsWrittenInTheUnitItReads(t *testing.T) {
	cases := map[float64]string{
		0:      "",
		-5:     "",
		2240:   "2.24 GHz",
		3200:   "3.2 GHz",
		3500.4: "3.5 GHz",
		800:    "800 MHz",
		999.9:  "1000 MHz",
	}
	for mhz, want := range cases {
		if got := cpuClock(mhz); got != want {
			t.Errorf("cpuClock(%v) = %q, want %q", mhz, got, want)
		}
	}
}

// TestSpellMinutes checks the battery's remaining time, which is a span and not
// a clock: "3 小时 12 分" rather than FormatDuration's "3 小时前".
func TestSpellMinutes(t *testing.T) {
	cases := map[int]string{
		0: "", -1: "", 45: "45 分钟", 60: "1 小时", 192: "3 小时 12 分",
	}
	for minutes, want := range cases {
		if got := spellMinutes(minutes); got != want {
			t.Errorf("spellMinutes(%d) = %q, want %q", minutes, got, want)
		}
	}
}

// TestGPUModelKeepsTheModelNotTheBrand checks the GPU card's badge: the panel
// puts it beside a title that already says "GPU", so "GeForce RTX 3060 Laptop
// GPU" must not spend the badge on words that say who made the card.
func TestGPUModelKeepsTheModelNotTheBrand(t *testing.T) {
	cases := map[string]string{
		"GeForce RTX 3060 Laptop GPU":                 "RTX 3060 Laptop",
		"NVIDIA GeForce RTX 4090":                     "RTX 4090",
		"AMD Radeon RX 6800M":                         "RX 6800M",
		"Intel(R) UHD Graphics 630":                   "UHD 630",
		"Intel Iris Xe Graphics":                      "Iris Xe",
		"Radeon(TM) Graphics":                         "Radeon Graphics",
		"NVIDIA GeForce RTX 4070 Ti SUPER Laptop GPU": "RTX 4070 Ti SUPER",
		"Microsoft Basic Render Driver":               "Microsoft Basic",
	}
	for raw, want := range cases {
		if got := gpuModel(raw); got != want {
			t.Errorf("gpuModel(%q) = %q, want %q", raw, got, want)
		}
	}
	// A machine whose graphics could be counted but not named has no model to
	// show, and the card's badge stays empty rather than saying "显卡".
	for _, nothing := range []string{"", gpuPlaceholderName} {
		if got := gpuModel(nothing); got != "" {
			t.Errorf("gpuModel(%q) = %q, want no badge", nothing, got)
		}
	}
}

// TestGPUModelFitsTheBadge checks the length the panel's badge has room for: a
// model is cut at a word boundary, so a badge never ends in half a word.
func TestGPUModelFitsTheBadge(t *testing.T) {
	if gpuModelMax != 18 {
		t.Errorf("the badge fits %d characters, want 18", gpuModelMax)
	}
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"RTX 4090 Laptop", 18, "RTX 4090 Laptop"},
		{"RTX 4070 Ti SUPER Laptop", 18, "RTX 4070 Ti SUPER"},
		{"ABCDEFGHIJ", 4, "ABCD"},
		{"中文显卡", 3, "中文显"},
		{"short", 10, "short"},
		{"", 5, ""},
	}
	for _, c := range cases {
		if got := clip(c.in, c.max); got != c.want {
			t.Errorf("clip(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
	}
	for _, raw := range []string{
		"GeForce RTX 3060 Laptop GPU",
		"NVIDIA GeForce RTX 4070 Ti SUPER Laptop GPU",
		"Microsoft Basic Render Driver",
	} {
		if got := gpuModel(raw); len([]rune(got)) > gpuModelMax {
			t.Errorf("gpuModel(%q) = %q, longer than the badge", raw, got)
		}
	}
}

// TestDiskSeverityColorsByHowFullItIs checks the free-space number's color: a
// volume past 85% gone is worth a look, one past 95% is a problem.
func TestDiskSeverityColorsByHowFullItIs(t *testing.T) {
	cases := map[float64]core.Severity{
		60: core.SeverityOK, 85: core.SeverityWarn, 95: core.SeverityCritical,
	}
	for used, want := range cases {
		if got := diskSeverity(used); got != want {
			t.Errorf("diskSeverity(%v) = %v, want %v", used, got, want)
		}
	}
}

// TestBatterySeverityWaitsUntilItMatters checks the battery's color: 10% or
// less on the battery is critical, 30% or less is a warning, and none of it
// matters while the machine is charging.
func TestBatterySeverityWaitsUntilItMatters(t *testing.T) {
	cases := []struct {
		level    float64
		charging bool
		want     core.Severity
	}{
		{5, false, core.SeverityCritical},
		{10, false, core.SeverityCritical},
		{25, false, core.SeverityWarn},
		{80, false, core.SeverityOK},
		{5, true, core.SeverityOK},
	}
	for _, c := range cases {
		if got := batterySeverity(c.level, c.charging); got != c.want {
			t.Errorf("batterySeverity(%v, charging=%v) = %v, want %v", c.level, c.charging, got, c.want)
		}
	}
}
