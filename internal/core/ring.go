package core

import "sync"

// Ring keeps the most recent values of one reading for the panel's sparkline
// and its history.
type Ring struct {
	mu   sync.Mutex
	buf  []float64
	next int
	full bool
}

// NewRing makes a ring that keeps at most capacity values.
func NewRing(capacity int) *Ring {
	if capacity <= 0 {
		capacity = 1
	}
	return &Ring{buf: make([]float64, capacity)}
}

// Push adds a value, dropping the oldest once the ring is full.
func (r *Ring) Push(v float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.next] = v
	r.next = (r.next + 1) % len(r.buf)
	if r.next == 0 {
		r.full = true
	}
}

// Values returns the values from the oldest to the newest.
func (r *Ring) Values() []float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		out := make([]float64, r.next)
		copy(out, r.buf[:r.next])
		return out
	}
	out := make([]float64, 0, len(r.buf))
	out = append(out, r.buf[r.next:]...)
	out = append(out, r.buf[:r.next]...)
	return out
}

// Len returns how many values the ring holds.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.full {
		return len(r.buf)
	}
	return r.next
}

// Last returns the newest value, and whether there is one.
func (r *Ring) Last() (float64, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.next == 0 && !r.full {
		return 0, false
	}
	i := r.next - 1
	if i < 0 {
		i = len(r.buf) - 1
	}
	return r.buf[i], true
}

// Clear drops every value.
func (r *Ring) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next = 0
	r.full = false
}
