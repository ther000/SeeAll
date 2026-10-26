package panel

import (
	"reflect"
	"testing"

	"seeall/internal/core"
)

// TestArrangeOrdersPutsTheSavedOrderFirst checks a page uses the order the user
// dragged its cards into, and still shows a reading the plugin started
// reporting afterwards.
func TestArrangeOrdersPutsTheSavedOrderFirst(t *testing.T) {
	readings := []core.Reading{
		{ID: "cpu", Label: "CPU"},
		{ID: "mem", Label: "内存"},
		{ID: "disk", Label: "磁盘"},
	}

	got := orderIDs(arrangeOrders(readings, []string{"disk", "cpu"}))
	if want := []string{"disk", "cpu", "mem"}; !reflect.DeepEqual(got, want) {
		t.Errorf("arranged order = %v, want %v", got, want)
	}

	// A saved ID the plugin no longer reports is skipped rather than shown as
	// an empty card.
	got = orderIDs(arrangeOrders(readings, []string{"gone", "mem"}))
	if want := []string{"mem", "cpu", "disk"}; !reflect.DeepEqual(got, want) {
		t.Errorf("order with a reading that went away = %v, want %v", got, want)
	}

	// Nothing saved leaves the plugin's own order.
	got = orderIDs(arrangeOrders(readings, nil))
	if want := []string{"cpu", "mem", "disk"}; !reflect.DeepEqual(got, want) {
		t.Errorf("order without a saved one = %v, want %v", got, want)
	}
}

// TestMoveCardDragsOneCard checks a card dragged onto another's place takes it,
// and the cards between shift by one.
func TestMoveCardDragsOneCard(t *testing.T) {
	order := []string{"a", "b", "c", "d"}
	cases := []struct {
		name     string
		from, to int
		want     []string
	}{
		{"forwards", 0, 2, []string{"b", "c", "a", "d"}},
		{"backwards", 3, 1, []string{"a", "d", "b", "c"}},
		{"to the end", 0, 4, []string{"b", "c", "d", "a"}},
		{"nowhere", 2, 2, []string{"a", "b", "c", "d"}},
		{"out of range", 0, 9, []string{"a", "b", "c", "d"}},
	}
	for _, c := range cases {
		if got := moveCard(order, c.from, c.to); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: moveCard(%v, %d, %d) = %v, want %v",
				c.name, order, c.from, c.to, got, c.want)
		}
	}

	// The order it was given is left alone: the host's copy is what changes.
	if !reflect.DeepEqual(order, []string{"a", "b", "c", "d"}) {
		t.Errorf("moveCard changed its argument: %v", order)
	}
}
