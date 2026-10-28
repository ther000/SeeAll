package system

import (
	"strings"
	"testing"
	"time"

	"seeall/internal/core"
	"seeall/internal/plugin"
)

// newTestInstance makes a system monitor with its own environment, so that a
// test can collect without a host or a window.
func newTestInstance(t *testing.T) *instance {
	t.Helper()
	m := Manifest()
	env := plugin.NewEnv("system", m, plugin.NewSettings(m.Settings))
	return &instance{env: env}
}

// TestCollectPublishesTheCounters checks the system page's readings come from
// the sample, and that the graphics pair only shows up on a machine that has a
// GPU: a sampler that reported nothing must not put an empty "GPU 0%" on the
// page.
func TestCollectPublishesTheCounters(t *testing.T) {
	in := newTestInstance(t)
	in.collect()

	snap := in.env.Snapshot()
	byID := map[string]core.Reading{}
	for _, r := range snap.Readings {
		byID[r.ID] = r
	}
	for _, want := range []string{"cpu", "mem", "net", "uptime"} {
		if _, ok := byID[want]; !ok {
			t.Errorf("the page has no %q reading: %v", want, snap.Readings)
		}
	}
	// The two directions share one card: a download card and an upload card
	// side by side would take the room a chart needs.
	for _, gone := range []string{"net-down", "net-up"} {
		if _, ok := byID[gone]; ok {
			t.Errorf("the page still has the separate %q reading: %v", gone, snap.Readings)
		}
	}

	// The machine running the test either has a GPU or does not; what must
	// hold is that the pair agrees with itself.
	_, hasGPU := byID["gpu"]
	_, hasVRAM := byID["vram"]
	if hasVRAM && !hasGPU {
		t.Error("the page shows VRAM without a GPU reading")
	}
	if gpu, ok := byID["gpu"]; ok {
		if gpu.Value < 0 || gpu.Value > 100 {
			t.Errorf("GPU is %v%%, outside 0..100", gpu.Value)
		}
	}
	if vram, ok := byID["vram"]; ok {
		if vram.Value < 0 || vram.Value > 100 {
			t.Errorf("VRAM is %v%%, outside 0..100", vram.Value)
		}
		if vram.Detail == "" {
			t.Error("the VRAM reading says nothing about how much it is of what")
		}
	}

	// The dashboard's header reads these two: a score in the range the panel
	// draws as a number, and a word for it.
	if snap.Health < 1 || snap.Health > 100 {
		t.Errorf("the machine scores %d, outside 1..100", snap.Health)
	}
	if snap.HealthNote == "" {
		t.Error("the score has no word to go with it")
	}
}

// TestFactsArePublished checks the header's pills come out of the sample rather
// than out of the panel: the pane cannot know what processor this is.
func TestFactsArePublished(t *testing.T) {
	in := newTestInstance(t)
	in.collect()
	snap := in.env.Snapshot()

	if len(snap.Facts) == 0 {
		t.Fatalf("the header has no pills at all: %v", snap.Readings)
	}
	for _, f := range snap.Facts {
		if f.Value == "" {
			t.Errorf("the header holds an empty pill: %+v", snap.Facts)
		}
	}
	// The uptime pill is the last one and is written with the label: the page
	// reads "已运行 2天2小时".
	last := snap.Facts[len(snap.Facts)-1]
	if last.Label == "" {
		t.Errorf("the last pill is unlabelled: %+v", snap.Facts)
	}
}

// TestNetworkReadingsAreRates checks the network card reads as a rate: a byte
// count with "per second" on it, which is what the reading's words carry, and
// the upload rate underneath.
func TestNetworkReadingsAreRates(t *testing.T) {
	in := newTestInstance(t)
	in.collect()
	snap := in.env.Snapshot()

	net, ok := snap.Reading("net")
	if !ok {
		t.Fatalf("the page has no network reading: %v", snap.Readings)
	}
	if net.Text == "" {
		t.Error("the network card shows a number, want words with a unit")
	}
	if len(net.Text) < 3 || net.Text[len(net.Text)-2:] != "/s" {
		t.Errorf("the network reads %q, want something per second", net.Text)
	}
	if net.Badge == "" {
		t.Error("the network card does not say which link carried the traffic")
	}
	if !strings.HasPrefix(net.Detail, "上传 ") {
		t.Errorf("the upload line reads %q", net.Detail)
	}
}

// TestAChosenCardThatIsGoneIsSaidSo checks the settings page's GPU choice is
// obeyed when the card leaves: the page says the card is not there rather than
// quietly counting another one, which would be a number about the wrong
// processor.
func TestAChosenCardThatIsGoneIsSaidSo(t *testing.T) {
	in := newTestInstance(t)
	in.env.Settings().Set("gpu", "FFFF:0000#0")
	in.collect()

	snap := in.env.Snapshot()
	gpu, ok := snap.Reading("gpu")
	if !ok {
		t.Fatalf("the page says nothing about the card it was told to watch: %v", snap.Readings)
	}
	if gpu.Text != "不可用" || gpu.Severity != core.SeverityOffline {
		t.Errorf("the missing card reads %q / %v, want 不可用 / offline", gpu.Text, gpu.Severity)
	}
	if gpu.Detail == "" {
		t.Error("the page does not say why the card reports nothing")
	}
	if snap.Status != core.StatusWarn {
		t.Errorf("the page is %v, want a warning", snap.Status)
	}
}

// TestUptimeIsWrittenAsASpan checks the uptime card and the header's pill use
// the dashboard's wording rather than the milliseconds the sampler measures: a
// machine up for two days reads "2天2小时", not "49.0 h".
func TestUptimeIsWrittenAsASpan(t *testing.T) {
	m := metrics{Hostname: "TUF", Uptime: 49*time.Hour + 59*time.Minute}
	snap := snapshotOf(m)
	up, ok := snap.Reading("uptime")
	if !ok {
		t.Fatalf("no uptime reading: %v", snap.Readings)
	}
	if up.Text != "2天1小时" {
		t.Errorf("uptime reads %q, want 2天1小时", up.Text)
	}
	last := snap.Facts[len(snap.Facts)-1]
	if last.Value != "2天1小时" {
		t.Errorf("the header's uptime is %q", last.Value)
	}
}

// snapshotOf wraps a sample the way collect does, without a machine.
func snapshotOf(m metrics) core.Snapshot {
	return core.Snapshot{Readings: readings(m), Facts: facts(m)}
}
