//go:build windows

package system

import "testing"

// TestGraphicsCardsAreNamedOnce checks what the settings page offers on this
// machine: every card the counters can report on, once, after "automatic".
// A machine with no graphics card has nothing to choose, and the page shows no
// such row.
func TestGraphicsCardsAreNamedOnce(t *testing.T) {
	cards := graphicsAdapters()
	if len(cards) == 0 {
		if len(gpuChoices()) != 1 {
			t.Errorf("a machine with no card still offers %v", gpuChoices())
		}
		t.Skip("this machine has no graphics card to choose")
	}

	// Every card offered must be one the counters report on, or the page
	// would be counting a processor that is not there.
	if known := engineLuids(); len(known) > 0 {
		for _, a := range cards {
			if !known[a.Luid] {
				t.Errorf("%s (%#x) is offered but no engine reports on it", a.Name, a.Luid)
			}
		}
	}

	choices := gpuChoices()
	if len(choices) != len(cards)+1 {
		t.Fatalf("%d choices for %d cards, plus automatic", len(choices), len(cards))
	}
	if choices[0].Value != gpuAuto {
		t.Errorf("the first choice is %q, want automatic", choices[0].Value)
	}
	// Two cards of the same model — a laptop lists its 3060 twice — are told
	// apart by a number, so that the row can be read.
	labels := map[string]bool{}
	for _, o := range choices[1:] {
		if labels[o.Label] {
			t.Errorf("two cards are called %q", o.Label)
		}
		labels[o.Label] = true
	}
}
