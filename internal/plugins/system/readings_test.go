package system

import (
	"testing"
	"time"

	"seeall/internal/core"
)

// fullMetrics is a whole machine in one value: a laptop with a card, a battery,
// a thermometer and a volume, so that the page's assembly can be read without
// asking the machine for anything.
func fullMetrics() metrics {
	return metrics{
		Hostname: "TUF",
		Platform: "Windows · windows/amd64",

		CPUPercent:  12,
		CPUName:     "AMD Ryzen 7 6800H with Radeon Graphics",
		CPUClockMHz: 2240,
		Threads:     16,

		MemTotal: 16 << 30, MemUsed: 8 << 30, MemUsedPercent: 50,

		HasTemp: true, TempC: 54.3,
		HasGPUTemp: true, GPUTempC: 50.2,

		HasDisk: true, DiskDrive: "C:", DiskLabel: "Windows",
		DiskTotal: 512 << 30, DiskFree: 128 << 30,

		NetKind: "Wi-Fi", NetRxRate: 2048, NetTxRate: 1024,

		HasBattery: true, BatteryPercent: 100, Charging: true,
		HasBatteryHealth: true, BatteryHealth: 78,

		HasGPU: true, GPUPercent: 4, GPUName: "RTX 3060 Laptop GPU",
		VRAMUsed: 2 << 30, VRAMTotal: 6 << 30,

		OSName: "Windows 11 24H2",
		Uptime: 49*time.Hour + 59*time.Minute,
	}
}

// TestTheValuesBehindTheCharts checks the number each card draws its chart
// from, which is not always the number it writes: the temperature bar is
// drawn against 100 degrees, the memory area against 100 percent, the network
// line against the download rate and the bottom bar's free space in bytes. A
// reading that only carried words would leave its chart empty.
func TestTheValuesBehindTheCharts(t *testing.T) {
	m := fullMetrics()
	byID := map[string]core.Reading{}
	for _, r := range readings(m) {
		byID[r.ID] = r
	}
	cases := map[string]float64{
		"cpu":  m.CPUPercent,
		"mem":  m.MemUsedPercent,
		"disk": float64(128 << 30),
		"net":  m.NetRxRate,
		"temp": m.TempC,
	}
	for id, want := range cases {
		if got := byID[id].Value; got != want {
			t.Errorf("%s plots %v, want %v", id, got, want)
		}
	}
}

// TestReadingsFollowTheDashboardOrder pins the order the cards are drawn in:
// the panel asks for readings by ID, but the page reads top to bottom, and the
// card for a value the machine has is not allowed to drift down the page.
func TestReadingsFollowTheDashboardOrder(t *testing.T) {
	got := make([]string, 0, 9)
	for _, r := range readings(fullMetrics()) {
		got = append(got, r.ID)
	}
	want := []string{"cpu", "gpu", "mem", "disk", "net", "temp", "vram", "battery", "uptime"}
	if len(got) != len(want) {
		t.Fatalf("the page has %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the page has %v, want %v", got, want)
		}
	}
}

// TestReadingsCarryWhatTheCardNeeds checks each card's badge and line of small
// print, with numbers the machine's own format writes to the last character:
// "2.24 GHz" beside a load, "C: · 512 GB" beside the free space, "已充满"
// under a battery that has one.
func TestReadingsCarryWhatTheCardNeeds(t *testing.T) {
	m := fullMetrics()
	byID := map[string]core.Reading{}
	for _, r := range readings(m) {
		byID[r.ID] = r
	}

	cases := []struct {
		id     string
		badge  string
		detail string
		text   string
	}{
		{"cpu", "2.24 GHz", "16 线程", ""},
		{"gpu", "50°C", "RTX 3060 Laptop GPU", ""},
		{"mem", "压力正常", "8 GB / 16 GB", ""},
		{"disk", "C: · 512 GB", "已用 384 GB · 75%", ""},
		{"net", "Wi-Fi", "上传 1 KB/s", "2 KB/s"},
		{"temp", "正常", "系统 54° · GPU 50°", "54°C"},
		{"vram", "", "2 GB / 6 GB", ""},
		{"battery", "健康 78%", "已充满", ""},
		{"uptime", "", "", "2天1小时"},
	}
	for _, c := range cases {
		r, ok := byID[c.id]
		if !ok {
			t.Errorf("no %s reading", c.id)
			continue
		}
		if r.Badge != c.badge {
			t.Errorf("%s badge is %q, want %q", c.id, r.Badge, c.badge)
		}
		if r.Detail != c.detail {
			t.Errorf("%s detail is %q, want %q", c.id, r.Detail, c.detail)
		}
		if r.Text != c.text {
			t.Errorf("%s text is %q, want %q", c.id, r.Text, c.text)
		}
	}
}

// TestDiskReadsAsSpaceNotAsAPercentage checks the free-space card shows bytes:
// the dashboard draws a bar from the percentage, and the number above it is
// what the user is looking for when the disk fills up.
func TestDiskReadsAsSpaceNotAsAPercentage(t *testing.T) {
	disk, ok := diskReading(fullMetrics())
	if !ok {
		t.Fatal("a machine with a readable volume has no free-space reading")
	}
	if disk.Unit != core.UnitBytes || disk.Value != float64(128<<30) {
		t.Errorf("disk is %v %v, want the free byte count", disk.Value, disk.Unit)
	}
	if got := disk.Format(); got != "128 GB" {
		t.Errorf("the card reads %q, want 128 GB", got)
	}
}

// TestNetworkIsOneReadingWithTheUploadUnderIt checks the two directions share
// one card: the number is what the chart plots, and the upload rate is the line
// of small print rather than a card of its own.
func TestNetworkIsOneReadingWithTheUploadUnderIt(t *testing.T) {
	m := fullMetrics()
	m.NetRxRate, m.NetTxRate = 0, 0
	var nets []core.Reading
	for _, r := range readings(m) {
		if r.ID == "net" || r.ID == "net-down" || r.ID == "net-up" {
			nets = append(nets, r)
		}
	}
	if len(nets) != 1 {
		t.Fatalf("the page has %d network readings, want one", len(nets))
	}
	net := nets[0]
	if net.Text != "0 B/s" {
		t.Errorf("a quiet network reads %q, want 0 B/s", net.Text)
	}
	if net.Detail != "上传 0 B/s" {
		t.Errorf("the upload line reads %q", net.Detail)
	}
	if net.Badge != "Wi-Fi" {
		t.Errorf("the badge is %q, want the link", net.Badge)
	}
}

// TestThePageLeavesOutWhatTheMachineCannotSay checks that a machine without a
// thermometer, a battery or a readable volume gets no readings for them, and
// that its network badge says "全部网卡" rather than naming a link it never saw.
func TestThePageLeavesOutWhatTheMachineCannotSay(t *testing.T) {
	m := metrics{
		Hostname: "TUF", Platform: "linux/amd64",
		CPUPercent: 3, Threads: 8,
		MemTotal: 4 << 30, MemUsed: 1 << 30, MemUsedPercent: 25,
		Uptime: 3 * time.Hour,
	}
	got := make([]string, 0, 4)
	for _, r := range readings(m) {
		got = append(got, r.ID)
	}
	want := []string{"cpu", "mem", "net", "uptime"}
	if len(got) != len(want) {
		t.Fatalf("the page has %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the page has %v, want %v", got, want)
		}
	}
	net := readings(m)[2]
	if net.Badge != "全部网卡" {
		t.Errorf("a machine with no adapter up is called %q", net.Badge)
	}
	// A processor whose clock nobody reported has no clock badge; an empty
	// badge is what the card draws as nothing.
	if cpu := readings(m)[0]; cpu.Badge != "" {
		t.Errorf("a processor with no clock reports %q", cpu.Badge)
	}
}

// TestAChosenCardThatIsGoneSaysSoInTheGraphsPlace checks the offline reading
// sits in the GPU card's position, above memory, rather than at the end of the
// page where the user would not look for it.
func TestAChosenCardThatIsGoneSaysSoInTheGraphsPlace(t *testing.T) {
	m := fullMetrics()
	m.HasGPU, m.VRAMUsed, m.VRAMTotal = false, 0, 0
	m.GPUProblem = "找不到这张显卡"
	list := readings(m)
	if list[1].ID != "gpu" {
		t.Fatalf("the page starts %v", []string{list[0].ID, list[1].ID})
	}
	if list[1].Text != "不可用" || list[1].Severity != core.SeverityOffline {
		t.Errorf("the missing card reads %q / %v", list[1].Text, list[1].Severity)
	}
	for _, r := range list {
		if r.ID == "vram" {
			t.Error("a card that is not there still reports its memory")
		}
	}
}

// TestFactsAreTheMachinesOwnWords checks the header pills: the model without
// the trademarks, the memory in the unit people buy memory in, the Windows
// release, and the uptime in the words FormatSpan writes.
func TestFactsAreTheMachinesOwnWords(t *testing.T) {
	got := facts(fullMetrics())
	want := []string{"Ryzen 7 6800H", "16 GB", "Windows 11 24H2", "已运行 2天1小时"}
	if len(got) != len(want) {
		t.Fatalf("the header has %v, want %v", got, want)
	}
	for i, c := range want {
		if text := got[i].Text(); text != c {
			t.Errorf("pill %d reads %q, want %q", i, text, c)
		}
	}
}

// TestTheHeaderHasNoEmptyPills checks that a machine which could not be asked
// its name, its memory or its Windows release shows no pills at all rather
// than three blank ones.
func TestTheHeaderHasNoEmptyPills(t *testing.T) {
	if got := facts(metrics{Uptime: 0}); len(got) != 0 {
		t.Errorf("an unknown machine's header is %v", got)
	}
	only := facts(metrics{CPUName: "AMD Ryzen 7 6800H", Uptime: 90 * time.Second})
	if len(only) != 2 {
		t.Fatalf("the header is %v, want the name and the uptime", only)
	}
	if only[0].Text() != "Ryzen 7 6800H" || only[1].Text() != "已运行 1分钟" {
		t.Errorf("the header reads %q and %q", only[0].Text(), only[1].Text())
	}
}

// TestBatteryBadgeAndDetailDoNotRepeatThemselves checks the two lines of the
// battery card divide the work: the wear in the badge, the state in the small
// print, and no line saying "充电中" twice.
func TestBatteryBadgeAndDetailDoNotRepeatThemselves(t *testing.T) {
	cases := []struct {
		name   string
		m      metrics
		badge  string
		detail string
	}{
		{"worn and charging", metrics{HasBattery: true, BatteryPercent: 82, Charging: true,
			HasBatteryHealth: true, BatteryHealth: 78}, "健康 78%", "充电中"},
		{"worn and running", metrics{HasBattery: true, BatteryPercent: 82,
			HasBatteryHealth: true, BatteryHealth: 78, BatteryMinutes: 192}, "健康 78%", "3 小时 12 分续航"},
		{"full but unknown wear", metrics{HasBattery: true, BatteryPercent: 100, Charging: true}, "已充满", ""},
		{"charging", metrics{HasBattery: true, BatteryPercent: 55, Charging: true}, "充电中", ""},
		{"running on a charge", metrics{HasBattery: true, BatteryPercent: 55, BatteryMinutes: 45}, "", "45 分钟续航"},
	}
	for _, c := range cases {
		r, ok := batteryReading(c.m)
		if !ok {
			t.Errorf("%s: no battery reading", c.name)
			continue
		}
		if r.Badge != c.badge {
			t.Errorf("%s: badge is %q, want %q", c.name, r.Badge, c.badge)
		}
		if r.Detail != c.detail {
			t.Errorf("%s: detail is %q, want %q", c.name, r.Detail, c.detail)
		}
		if r.Badge != "" && r.Badge == r.Detail {
			t.Errorf("%s: the badge and the detail both say %q", c.name, r.Badge)
		}
	}
}

// TestTemperatureBadgeFollowsTheHeat checks the word beside the temperature and
// the color of the number move together: a machine at 54°C is normal, one at
// 78°C is worth remarking on, and one at 90°C is a problem.
func TestTemperatureBadgeFollowsTheHeat(t *testing.T) {
	cases := []struct {
		celsius float64
		badge   string
		sev     core.Severity
	}{
		{54, "正常", core.SeverityOK},
		{69.9, "正常", core.SeverityOK},
		{70, "偏高", core.SeverityWarn},
		{85, "过热", core.SeverityCritical},
	}
	for _, c := range cases {
		m := metrics{HasTemp: true, TempC: c.celsius}
		r, ok := tempReading(m)
		if !ok {
			t.Fatalf("%v°C has no reading", c.celsius)
		}
		if r.Badge != c.badge || r.Severity != c.sev {
			t.Errorf("%v°C reads %q / %v, want %q / %v", c.celsius, r.Badge, r.Severity, c.badge, c.sev)
		}
	}
}

// TestMemoryPressureIsWorded checks the memory badge's three words land on the
// three bands the panel's card has room for.
func TestMemoryPressureIsWorded(t *testing.T) {
	cases := map[float64]string{0: "压力正常", 64.9: "压力正常", 65: "压力偏高", 84.9: "压力偏高", 85: "压力紧张", 100: "压力紧张"}
	for percent, want := range cases {
		if got := memPressure(percent); got != want {
			t.Errorf("memPressure(%v) = %q, want %q", percent, got, want)
		}
	}
}
