package system

import (
	"testing"

	"seeall/internal/core"
	"seeall/internal/plugin"
)

// newTestInstance makes a system monitor with its own environment, so that a
// test can collect without a host or a window.
func newTestInstance(t *testing.T) *instance {
	t.Helper()
	m := Manifest()
	env := plugin.NewEnv("system", "本机", m, plugin.NewSettings(m.Settings))
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
	for _, want := range []string{"cpu", "mem", "net-down", "net-up", "uptime"} {
		if _, ok := byID[want]; !ok {
			t.Errorf("the page has no %q reading: %v", want, snap.Readings)
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
}

// TestNetworkReadingsAreRates checks the network readings read as rates: a
// byte count with "per second" on it, which is what the reading's words carry.
func TestNetworkReadingsAreRates(t *testing.T) {
	in := newTestInstance(t)
	in.collect()
	snap := in.env.Snapshot()

	for _, id := range []string{"net-down", "net-up"} {
		var found bool
		for _, r := range snap.Readings {
			if r.ID != id {
				continue
			}
			found = true
			if r.Text == "" {
				t.Errorf("%s shows a number, want words with a unit", id)
			}
			if len(r.Text) < 3 || r.Text[len(r.Text)-2:] != "/s" {
				t.Errorf("%s reads %q, want something per second", id, r.Text)
			}
		}
		if !found {
			t.Errorf("no %s reading", id)
		}
	}
}
