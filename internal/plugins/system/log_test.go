package system

import (
	"fmt"
	"strings"
	"testing"

	"seeall/internal/core"
	"seeall/internal/plugin"
)

// captureLog makes an instance whose log lines are collected, so that a test
// can say what reached the log rather than what the page shows.
func captureLog(t *testing.T, id string) (*instance, *[]string) {
	t.Helper()
	m := Manifest()
	env := plugin.NewEnv(id, m, plugin.NewSettings(m.Settings))
	lines := &[]string{}
	env.SetLog(func(format string, args ...any) {
		*lines = append(*lines, fmt.Sprintf(format, args...))
	})
	in := &instance{env: env}
	in.sampler.logf = env.Logf
	return in, lines
}

// TestAGpuProblemIsLogged checks a card the settings name and the machine no
// longer has is recorded, not only drawn: the page says "找不到这张显卡" in a
// card the user can see, and nothing about it reached the log at all.
func TestAGpuProblemIsLogged(t *testing.T) {
	in, lines := captureLog(t, "system")
	// A card no machine has.
	in.env.Settings().Set("gpu", "FFFF:FFFF#0")
	in.collect()

	snap := in.env.Snapshot()
	gpu, ok := snap.Reading("gpu")
	if !ok {
		t.Fatalf("no GPU reading: %v", snap.Readings)
	}
	if gpu.Severity != core.SeverityOffline {
		t.Fatalf("the missing card reads %q, want it offline", gpu.Text)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, missingCard) {
		t.Errorf("log says %q, want it to name the missing card", joined)
	}

	// The condition lasts, so the line must not: two more samples say nothing.
	before := len(*lines)
	in.collect()
	in.collect()
	if len(*lines) != before {
		t.Errorf("the same problem was logged again: %v", (*lines)[before:])
	}

	// The card coming back and going again is a new event.
	in.env.Settings().Set("gpu", gpuAuto)
	in.collect()
	in.env.Settings().Set("gpu", "FFFF:FFFF#0")
	in.collect()
	if len(*lines) == before {
		t.Error("the card went missing a second time and nothing was logged")
	}
}
