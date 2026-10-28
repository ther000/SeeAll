package app

import (
	"testing"

	"seeall/internal/core"
)

// TestANumberKeptBehindWordsHasAHistory checks the readings whose value is
// written as words: the network says "1.7 KB/s" and the temperature "59°C",
// and the network's card draws a chart of the number behind those words. A
// reading with no number at all — a link saying "无线" — is not a series and
// must not grow an empty one.
func TestANumberKeptBehindWordsHasAHistory(t *testing.T) {
	h := NewHost(t.TempDir())
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	// The plugins sample once before they are stopped; the test counts only
	// what it injects.
	h.ClearHistory()

	for i := 0; i < 3; i++ {
		h.Inject("system", core.Snapshot{Readings: []core.Reading{
			{ID: "net", Label: "网络", Value: float64(1000 + i*100), Unit: core.UnitBytes, Text: "1 KB/s"},
			{ID: "link", Label: "连接", Text: "无线"},
		}})
	}

	if got := h.Instance("system").History("net"); len(got) != 3 {
		t.Errorf("the network's rate has %d samples, want 3: %v", len(got), got)
	}
	if got := h.Instance("system").History("link"); len(got) != 0 {
		t.Errorf("a reading of words grew a series: %v", got)
	}
}
