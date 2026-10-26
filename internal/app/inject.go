package app

import (
	"time"

	"seeall/internal/core"
)

// Inject publishes a snapshot as if the plugin reported it, and returns
// whether the instance exists. The preview tool shows a panel this way on
// machines without the hardware.
func (h *Host) Inject(instanceID string, snap core.Snapshot) bool {
	if h.Instance(instanceID) == nil {
		return false
	}
	if snap.At.IsZero() {
		snap.At = time.Now()
	}
	h.storeSnapshot(instanceID, snap)
	return true
}

// InjectHistory replaces a reading's history, for the preview's sparklines.
func (h *Host) InjectHistory(instanceID, readingID string, values []float64) {
	in := h.Instance(instanceID)
	if in == nil {
		return
	}
	in.mu.Lock()
	if in.history == nil {
		in.history = map[string]*core.Ring{}
	}
	ring := core.NewRing(120)
	for _, v := range values {
		ring.Push(v)
	}
	in.history[readingID] = ring
	in.mu.Unlock()
}

// ClearHistory drops every instance's history, so a preview starts from the
// values it injects and nothing the plugins reported before it took over.
func (h *Host) ClearHistory() {
	for _, in := range h.Instances() {
		in.mu.Lock()
		in.history = map[string]*core.Ring{}
		in.mu.Unlock()
	}
}
