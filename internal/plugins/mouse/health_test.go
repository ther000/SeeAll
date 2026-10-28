package mouse

import (
	"testing"
	"time"

	"seeall/internal/plugin"
)

// TestMouseHealthKeepsTheLevelAndAddsAWord checks the dashboard's headline for a
// mouse: the charge itself, so the big number is the number on the side of the
// mouse, and a word for what that charge means.
func TestMouseHealthKeepsTheLevelAndAddsAWord(t *testing.T) {
	cases := []struct {
		level int
		score int
		note  string
	}{
		{100, 100, "电量充足"},
		{60, 60, "电量充足"},
		{59, 59, "电量偏低"},
		{30, 30, "电量偏低"},
		{29, 29, "需要充电"},
		{1, 1, "需要充电"},
		// A mouse reporting an empty battery is scoring itself, badly. Zero
		// would mean "no score" and the dashboard would draw the mouse's name
		// instead of the bad news.
		{0, 1, "需要充电"},
		{-4, 1, "需要充电"},
		{120, 100, "电量充足"},
	}
	for _, c := range cases {
		score, note := mouseHealth(c.level)
		if score != c.score || note != c.note {
			t.Errorf("mouseHealth(%d) = %d %q, want %d %q", c.level, score, note, c.score, c.note)
		}
	}
}

// TestPublishPutsTheChargeInTheHeader checks the snapshot the dashboard reads:
// a fresh frame's charge is the health, and the pills name the link and the
// resolution rather than repeating the battery level the card already shows.
func TestPublishPutsTheChargeInTheHeader(t *testing.T) {
	manifest := Manifest()
	env := plugin.NewEnv("mouse", manifest, plugin.NewSettings(manifest.Settings))
	in := &instance{
		env: env, hasFrame: true, lastAt: time.Now(),
		frame: Frame{Battery: 82, DPI: 1600, Link: 0x01},
	}
	in.publish()

	snap := env.Snapshot()
	if snap.Health != 82 || snap.HealthNote != "电量充足" {
		t.Errorf("the header reads %d %q, want 82 电量充足", snap.Health, snap.HealthNote)
	}
	if len(snap.Facts) != 2 {
		t.Fatalf("the header has %v", snap.Facts)
	}
	if got := snap.Facts[0].Text(); got != "连接 无线" {
		t.Errorf("the first pill reads %q", got)
	}
	if got := snap.Facts[1].Text(); got != "DPI 1600" {
		t.Errorf("the second pill reads %q", got)
	}
}

// TestAStaleFrameHasNoHealthScore checks a mouse that has stopped reporting: the
// dashboard must not lead with the charge it had an hour ago, so the snapshot
// leaves the score at 0 and the panel draws the device's name instead.
func TestAStaleFrameHasNoHealthScore(t *testing.T) {
	manifest := Manifest()
	env := plugin.NewEnv("mouse", manifest, plugin.NewSettings(manifest.Settings))
	in := &instance{
		env: env, hasFrame: true, lastAt: time.Now().Add(-time.Minute),
		frame: Frame{Battery: 82, DPI: 1600, Link: 0x01},
	}
	in.publish()

	snap := env.Snapshot()
	if snap.Health != 0 || snap.HealthNote != "" {
		t.Errorf("a mouse that went quiet scores %d %q", snap.Health, snap.HealthNote)
	}
	if got := snap.Facts[0].Text(); got != "连接 已断开" {
		t.Errorf("the link pill reads %q", got)
	}
}
