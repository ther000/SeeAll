package system

import (
	"fmt"
	"math"
	"strings"

	"seeall/internal/core"
)

// The page's readings, in the order the dashboard draws its cards. Each ID is
// a contract with the panel — a card asks for "cpu" and shows whatever the
// sampler put there — and each Badge says what the number cannot: the clock
// behind a load, the device behind a rate, the health behind a charge.

// processCount is how many programs the dashboard's list has room for.
const processCount = 5

// readings assembles one sample into the page's readings.
func readings(m metrics) []core.Reading {
	out := make([]core.Reading, 0, 9)
	out = append(out, cpuReading(m))
	if r, ok := gpuReading(m); ok {
		out = append(out, r)
	}
	out = append(out, memReading(m))
	if r, ok := diskReading(m); ok {
		out = append(out, r)
	}
	out = append(out, netReading(m))
	if r, ok := tempReading(m); ok {
		out = append(out, r)
	}
	if r, ok := vramReading(m); ok {
		out = append(out, r)
	}
	if r, ok := batteryReading(m); ok {
		out = append(out, r)
	}
	out = append(out, uptimeReading(m))
	return out
}

// facts are the pills across the dashboard's header: what the machine is,
// rather than what it is doing. A fact the machine could not be asked about is
// left out, so the row never holds an empty pill.
func facts(m metrics) []core.Fact {
	out := make([]core.Fact, 0, 4)
	if name := shortCPUName(m.CPUName); name != "" {
		out = append(out, core.Fact{Value: name})
	}
	if m.MemTotal > 0 {
		out = append(out, core.Fact{Value: core.FormatBytes(m.MemTotal)})
	}
	if m.OSName != "" {
		out = append(out, core.Fact{Value: m.OSName})
	}
	if m.Uptime > 0 {
		out = append(out, core.Fact{Label: "已运行", Value: core.FormatSpan(m.Uptime)})
	}
	return out
}

func cpuReading(m metrics) core.Reading {
	r := core.Reading{
		ID: "cpu", Label: "CPU", Value: m.CPUPercent, Unit: core.UnitPercent,
		Severity: loadSeverity(m.CPUPercent),
		Badge:    cpuClock(m.CPUClockMHz),
	}
	if m.Threads > 0 {
		r.Detail = fmt.Sprintf("%d 线程", m.Threads)
	}
	return r
}

func gpuReading(m metrics) (core.Reading, bool) {
	switch {
	case m.HasGPU:
		r := core.Reading{
			ID: "gpu", Label: "GPU", Value: m.GPUPercent, Unit: core.UnitPercent,
			Severity: loadSeverity(m.GPUPercent),
			// The badge names the card, since the title only says "GPU"; its
			// temperature is the one number worth the line of small print.
			Badge: gpuModel(m.GPUName),
		}
		if m.HasGPUTemp {
			r.Detail = tempText(m.GPUTempC)
		}
		return r, true
	case m.GPUProblem != "":
		// The user chose a card that is not there any more. The page says so
		// in the place the reading would have taken.
		return core.Reading{
			ID: "gpu", Label: "GPU", Text: "不可用", Severity: core.SeverityOffline,
			Detail: m.GPUProblem,
		}, true
	}
	return core.Reading{}, false
}

func memReading(m metrics) core.Reading {
	r := core.Reading{
		ID: "mem", Label: "内存", Value: m.MemUsedPercent, Unit: core.UnitPercent,
		Severity: loadSeverity(m.MemUsedPercent),
		Badge:    memPressure(m.MemUsedPercent),
	}
	if m.MemTotal > 0 {
		r.Detail = core.FormatBytes(m.MemUsed) + " / " + core.FormatBytes(m.MemTotal)
	}
	return r
}

func diskReading(m metrics) (core.Reading, bool) {
	if !m.HasDisk || m.DiskTotal <= 0 {
		return core.Reading{}, false
	}
	used := math.Max(0, m.DiskTotal-m.DiskFree)
	usedPercent := 100 * used / m.DiskTotal
	return core.Reading{
		ID: "disk", Label: "可用空间", Value: m.DiskFree, Unit: core.UnitBytes,
		Severity: diskSeverity(usedPercent),
		Badge:    diskBadge(m.DiskDrive, m.DiskLabel, m.DiskTotal),
		Detail:   "已用 " + core.FormatBytes(used) + " · " + percentText(usedPercent),
	}, true
}

// netReading is one reading for both directions: the page has a card for the
// network, not two, and the upload rate is the second line of it.
func netReading(m metrics) core.Reading {
	return core.Reading{
		ID: "net", Label: "网络",
		// The number is the download rate, which is what a chart of this card
		// plots; the words say it in the unit people read rates in.
		Value: m.NetRxRate, Unit: core.UnitBytes, Text: rateText(m.NetRxRate),
		Severity: core.SeverityNone,
		Badge:    netBadge(m.NetKind),
		Detail:   "上传 " + rateText(m.NetTxRate),
	}
}

func tempReading(m metrics) (core.Reading, bool) {
	if !m.HasTemp {
		return core.Reading{}, false
	}
	return core.Reading{
		ID: "temp", Label: "温度",
		// The number is the degrees, which is what a bar of this card draws
		// against 100; the words are exactly how the card writes it.
		Value: m.TempC, Text: tempText(m.TempC),
		Severity: tempSeverity(m.TempC),
		Badge:    tempNote(m.TempC),
		Detail:   tempDetail(m),
	}, true
}

func vramReading(m metrics) (core.Reading, bool) {
	if !m.HasGPU || m.VRAMTotal <= 0 || m.VRAMUsed <= 0 {
		return core.Reading{}, false
	}
	used := 100 * m.VRAMUsed / m.VRAMTotal
	return core.Reading{
		ID: "vram", Label: "显存", Value: used, Unit: core.UnitPercent,
		Severity: loadSeverity(used),
		Detail:   core.FormatBytes(m.VRAMUsed) + " / " + core.FormatBytes(m.VRAMTotal),
	}, true
}

func batteryReading(m metrics) (core.Reading, bool) {
	if !m.HasBattery {
		return core.Reading{}, false
	}
	return core.Reading{
		ID: "battery", Label: "电池", Value: m.BatteryPercent, Unit: core.UnitPercent,
		Severity: batterySeverity(m.BatteryPercent, m.Charging),
		Badge:    batteryBadge(m),
		Detail:   batteryDetail(m),
	}, true
}

func uptimeReading(m metrics) core.Reading {
	return core.Reading{
		ID: "uptime", Label: "开机",
		// "2天2小时" rather than FormatMillis's "2.4 h": the pill and the card
		// are read at a glance, not measured.
		Text: core.FormatSpan(m.Uptime), Severity: core.SeverityNone,
	}
}

// cpuClock writes the speed the processor is running at now: "2.24 GHz", or
// "800 MHz" on a machine idling low enough to fall under one gigahertz.
func cpuClock(mhz float64) string {
	if mhz <= 0 || math.IsNaN(mhz) {
		return ""
	}
	if mhz >= 1000 {
		return core.FormatNumber(mhz/1000) + " GHz"
	}
	return core.FormatNumber(math.Round(mhz)) + " MHz"
}

// memPressure names how full the memory is in the words the card's badge has
// room for.
func memPressure(percent float64) string {
	switch {
	case percent >= 85:
		return "压力紧张"
	case percent >= 65:
		return "压力偏高"
	default:
		return "压力正常"
	}
}

// diskBadge names the volume and how big it is: "C: · 159 GB". A drive Windows
// gives no letter for is named by its label instead.
func diskBadge(drive, label string, total float64) string {
	who := drive
	if who == "" {
		who = label
	}
	if who == "" {
		return ""
	}
	if total <= 0 {
		return who
	}
	return who + " · " + core.FormatBytes(total)
}

// rateText writes a byte rate the way the network card reads it.
func rateText(bytesPerSecond float64) string {
	return core.FormatBytes(bytesPerSecond) + "/s"
}

// netBadge names the link the rates came over. The rates themselves sum every
// adapter, so a machine with no adapter up says so rather than claiming a
// technology.
func netBadge(kind string) string {
	if kind == "" {
		return "全部网卡"
	}
	return kind
}

// tempText writes a temperature the way the card's number and badge show it.
func tempText(celsius float64) string {
	return core.FormatNumber(math.Round(celsius)) + "°C"
}

// tempNote grades a temperature in the one word the badge holds.
func tempNote(celsius float64) string {
	switch {
	case celsius >= 85:
		return "过热"
	case celsius >= 70:
		return "偏高"
	default:
		return "正常"
	}
}

// tempDetail says which sensor the card's number came from: the machine's
// thermal zones, and the graphics card's own sensor when it has one.
func tempDetail(m metrics) string {
	parts := []string{"系统 " + core.FormatNumber(math.Round(m.TempC)) + "°"}
	if m.HasGPUTemp {
		parts = append(parts, "GPU "+core.FormatNumber(math.Round(m.GPUTempC))+"°")
	}
	return strings.Join(parts, " · ")
}

// batteryBadge says what the charge level does not: how worn the battery is,
// or what it is doing right now.
func batteryBadge(m metrics) string {
	if m.HasBatteryHealth && m.BatteryHealth > 0 {
		return "健康 " + percentText(m.BatteryHealth)
	}
	switch {
	case m.Charging && m.BatteryPercent >= 99:
		return "已充满"
	case m.Charging:
		return "充电中"
	}
	return ""
}

// batteryDetail is the battery's state as a sentence below the charge. The
// badge and this line divide the work: when the badge carries the wear, the line
// says what the battery is doing; when the badge already says "充电中", the line
// is only worth writing for a machine running on its battery.
func batteryDetail(m metrics) string {
	wearKnown := m.HasBatteryHealth && m.BatteryHealth > 0
	switch {
	case m.Charging && m.BatteryPercent >= 99:
		if wearKnown {
			return "已充满"
		}
	case m.Charging:
		if wearKnown {
			return "充电中"
		}
	case m.BatteryMinutes > 0:
		return spellMinutes(m.BatteryMinutes) + "续航"
	}
	return ""
}

// spellMinutes writes a span of minutes as "3 小时 12 分", which reads as a
// remaining time rather than as a clock.
func spellMinutes(minutes int) string {
	if minutes <= 0 {
		return ""
	}
	hours, rest := minutes/60, minutes%60
	switch {
	case hours == 0:
		return fmt.Sprintf("%d 分钟", rest)
	case rest == 0:
		return fmt.Sprintf("%d 小时", hours)
	default:
		return fmt.Sprintf("%d 小时 %d 分", hours, rest)
	}
}

// percentText writes a percentage the way the badges do: "60%".
func percentText(percent float64) string {
	return core.FormatNumber(math.Round(percent)) + "%"
}

// shortCPUName keeps the part of the processor's registry name a pill has room
// for: "AMD Ryzen 7 6800H with Radeon Graphics" reads "Ryzen 7 6800H".
func shortCPUName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ""
	}
	name = strings.NewReplacer("(R)", "", "(r)", "", "(TM)", "", "(tm)", "",
		"®", "", "™", "").Replace(name)
	// The integrated graphics and the nominal clock a BIOS appends are not
	// part of the model number.
	lower := strings.ToLower(name)
	for _, cut := range []string{" with ", " w/ ", " @", " cpu ", " processor "} {
		if i := strings.Index(lower, cut); i >= 0 {
			name, lower = name[:i], lower[:i]
		}
	}
	// "… 16-Core Processor" ends the name rather than sitting in the middle
	// of it, so the trailing word is dropped on its own.
	for _, tail := range []string{" Processor", " CPU"} {
		name = strings.TrimSuffix(name, tail)
	}
	for _, vendor := range []string{"AMD ", "Intel ", "ARM ", "Genuine Intel "} {
		if len(name) > len(vendor) && strings.HasPrefix(name, vendor) {
			name = name[len(vendor):]
			break
		}
	}
	return strings.Join(strings.Fields(name), " ")
}

// osPill writes the Windows version as the header pill does: family and
// release, "Windows 11 24H2". Windows 11 still calls itself "Windows 10 Pro" in
// the registry, so the build number decides which family it is.
func osPill(product, displayVersion string, build int) string {
	family := windowsFamily(product, build)
	if family == "" {
		return ""
	}
	if displayVersion == "" {
		return family
	}
	return family + " " + displayVersion
}

// windowsFamily drops the edition from a product name and settles the family
// from the build number.
func windowsFamily(product string, build int) string {
	name := strings.TrimSpace(product)
	for _, edition := range []string{" Professional", " Pro", " Home", " Enterprise",
		" Education", " Ultimate", " Single Language", " N", " K"} {
		name = strings.TrimSuffix(name, edition)
	}
	switch {
	case strings.HasPrefix(name, "Windows 11"), build >= 22000 && strings.HasPrefix(name, "Windows 10"):
		return "Windows 11"
	case strings.HasPrefix(name, "Windows 10"), build >= 10240 && strings.HasPrefix(name, "Windows"):
		return "Windows 10"
	}
	return name
}

// Severities: a high value is a problem, and how high depends on what is being
// measured.

// loadSeverity colors a percentage where high is bad.
func loadSeverity(p float64) core.Severity {
	switch {
	case p >= 92:
		return core.SeverityCritical
	case p >= 75:
		return core.SeverityWarn
	default:
		return core.SeverityOK
	}
}

// diskSeverity colors a volume by how much of it is gone.
func diskSeverity(usedPercent float64) core.Severity {
	switch {
	case usedPercent >= 95:
		return core.SeverityCritical
	case usedPercent >= 85:
		return core.SeverityWarn
	default:
		return core.SeverityOK
	}
}

// tempSeverity colors a temperature: a processor that stays past 85°C throttles.
func tempSeverity(celsius float64) core.Severity {
	switch {
	case celsius >= 85:
		return core.SeverityCritical
	case celsius >= 70:
		return core.SeverityWarn
	default:
		return core.SeverityOK
	}
}

// batterySeverity colors a charge level. A battery on the cable is charging,
// not draining, so a low level is not a problem while it is plugged in.
func batterySeverity(level float64, charging bool) core.Severity {
	switch {
	case charging:
		return core.SeverityOK
	case level <= 10:
		return core.SeverityCritical
	case level <= 30:
		return core.SeverityWarn
	default:
		return core.SeverityOK
	}
}
