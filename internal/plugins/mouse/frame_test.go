package mouse

import (
	"testing"

	"seeall/internal/core"
)

// TestParseFrame reads the state broadcast the VT9 PRO sends, byte by byte,
// as the old app recorded it from the hardware.
func TestParseFrame(t *testing.T) {
	raw := []byte{0xBB, 0xB0, 0x41, 0xB0, 0x04, 0x02, 0x64}
	f, ok := ParseFrame(raw)
	if !ok {
		t.Fatal("a valid state frame was rejected")
	}
	if f.Battery != 100 {
		t.Errorf("battery = %d, want 100", f.Battery)
	}
	if f.DPI != 0x04B0 {
		t.Errorf("dpi = %d, want 1200", f.DPI)
	}
	if !f.Wired() {
		t.Errorf("link 0x%02X should be wired", f.Link)
	}
	if len(f.Raw) != len(raw) || f.Raw[0] != raw[0] {
		t.Errorf("raw copy = % x, want % x", f.Raw, raw)
	}
}

// TestParseFrameRejects drops what is not a state broadcast, so a mouse's
// ordinary reports never become a battery level.
func TestParseFrameRejects(t *testing.T) {
	cases := map[string][]byte{
		"short":        {0xBB, 0xB0, 0x01},
		"other report": {0xBB, 0x01, 0x41, 0xB0, 0x04, 0x02, 0x64},
		"bad report":   {0x02, 0xB0, 0x41, 0xB0, 0x04, 0x02, 0x64},
		"level > 100":  {0xBB, 0xB0, 0x41, 0xB0, 0x04, 0x02, 0xC8},
	}
	for name, raw := range cases {
		if _, ok := ParseFrame(raw); ok {
			t.Errorf("%s: parsed as a state frame", name)
		}
	}
}

// TestBatterySeverity pins the levels the panel colors by: a mouse at 15%
// has to look alarming, one at 90% must not.
func TestBatterySeverity(t *testing.T) {
	cases := []struct {
		level int
		want  core.Severity
	}{
		{5, core.SeverityCritical},
		{10, core.SeverityCritical},
		{11, core.SeverityWarn},
		{30, core.SeverityWarn},
		{31, core.SeverityOK},
		{90, core.SeverityOK},
	}
	for _, c := range cases {
		if got := batterySeverity(c.level); got != c.want {
			t.Errorf("batterySeverity(%d) = %v, want %v", c.level, got, c.want)
		}
	}
}
