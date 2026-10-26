package panel

import (
	"seeall/internal/core"
)

// arrangeOrders applies the order a page's cards were dragged into to the
// readings a plugin reports: the readings the user arranged come first, in
// their order, and any the plugin started reporting later follow in the order
// the plugin gave them.
func arrangeOrders(readings []core.Reading, saved []string) []core.Reading {
	if len(saved) == 0 || len(readings) == 0 {
		return readings
	}
	byID := make(map[string]core.Reading, len(readings))
	for _, r := range readings {
		byID[r.ID] = r
	}
	out := make([]core.Reading, 0, len(readings))
	used := make(map[string]bool, len(readings))
	for _, id := range saved {
		if r, ok := byID[id]; ok && !used[id] {
			out = append(out, r)
			used[id] = true
		}
	}
	for _, r := range readings {
		if !used[r.ID] {
			out = append(out, r)
		}
	}
	return out
}

// orderIDs is the reading IDs of a page's cards, in the order they show.
func orderIDs(readings []core.Reading) []string {
	out := make([]string, len(readings))
	for i, r := range readings {
		out[i] = r.ID
	}
	return out
}

// moveCard takes the card at from out of the order and puts it before the card
// at to, as dragging it there does.
func moveCard(order []string, from, to int) []string {
	if from < 0 || from >= len(order) || to < 0 || to > len(order) || from == to {
		return order
	}
	out := make([]string, 0, len(order))
	for i, id := range order {
		if i == from {
			continue
		}
		if len(out) == to {
			out = append(out, order[from])
		}
		out = append(out, id)
	}
	if len(out) < len(order) {
		out = append(out, order[from])
	}
	return out
}
