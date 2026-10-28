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

// TestSayOnceLogsWhenTheProblemChanges checks the sampler's own reporting: a
// counter that is missing on every sample must cost one line, and a counter
// that comes back and goes again must cost a second one. A log that repeats
// itself every two seconds hides the line that mattered.
func TestSayOnceLogsWhenTheProblemChanges(t *testing.T) {
	in, lines := captureLog(t, "system")
	sampler := &in.sampler

	sampler.sayOnce("temp", "system: 读不到温度")
	sampler.sayOnce("temp", "system: 读不到温度")
	if len(*lines) != 1 {
		t.Fatalf("the same problem was written %d times: %v", len(*lines), *lines)
	}
	// A different problem is a different line, even while the first one lasts.
	sampler.sayOnce("disk", "system: 读不到 C: 的剩余空间")
	if len(*lines) != 2 {
		t.Fatalf("a second problem was written as %v", *lines)
	}
	// The temperature counter answers again, then stops again: that is news.
	sampler.sayClear("temp")
	sampler.sayOnce("temp", "system: 读不到温度")
	if len(*lines) != 3 {
		t.Errorf("a problem that came back was not written: %v", *lines)
	}
}
