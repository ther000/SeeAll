package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"seeall/internal/core"
)

// TestACardAnInstanceNoLongerReportsKeepsItsPlace is why the saved order is
// NOT pruned against a snapshot: a monitor reports what it can see this second,
// so the snapshot is a partial list. This machine has no battery, the system
// page reports no "battery" reading, and a page that pruned by the snapshot
// would delete the place the user gave that card — and then lose it for good
// when the next snapshot has it again.
//
// The reading a plugin stopped providing, "disk", is skipped when the page is
// drawn (arrangeOrders), which is where it can be known for certain.
func TestACardAnInstanceNoLongerReportsKeepsItsPlace(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	h.SetCardOrder("system", []string{"battery", "cpu", "disk"})

	// A snapshot without battery and without disk: both are missing, and only
	// one of them is gone for good. The host cannot tell which.
	h.Inject("system", core.Snapshot{
		Status: core.StatusOK, Primary: "cpu",
		Readings: []core.Reading{
			{ID: "cpu", Label: "CPU", Value: 20, Unit: core.UnitPercent},
		},
	})
	h.save()

	want := []string{"battery", "cpu", "disk"}
	got := h.CardOrder("system")
	if len(got) != len(want) {
		t.Fatalf("order = %v, want the arranged one %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "seeall.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\"battery\"") {
		t.Errorf("the file lost a card the machine merely lacks:\n%s", data)
	}
}

// TestAnOrderedReadingIsKept checks the pruning does not eat the order the user
// arranged: only readings the page stopped reporting go.
func TestAnOrderedReadingIsKept(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	h.SetCardOrder("system", []string{"gpu", "cpu", "mem"})
	h.Inject("system", core.Snapshot{
		Status: core.StatusOK,
		Readings: []core.Reading{
			{ID: "cpu", Label: "CPU", Value: 20, Unit: core.UnitPercent},
			{ID: "gpu", Label: "GPU", Value: 30, Unit: core.UnitPercent},
			{ID: "mem", Label: "内存", Value: 40, Unit: core.UnitPercent},
		},
	})
	h.save()

	want := []string{"gpu", "cpu", "mem"}
	got := h.CardOrder("system")
	if len(got) != len(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// TestAnOrderOfARemovedInstanceGoesAway keeps the file from collecting orders
// for monitors the user took away.
func TestAnOrderOfARemovedInstanceGoesAway(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	h.SetCardOrder("system", []string{"cpu", "mem"})
	h.Remove("system")
	h.save()

	if got := h.CardOrder("system"); len(got) != 0 {
		t.Errorf("a removed monitor kept its order: %v", got)
	}
	data, err := os.ReadFile(filepath.Join(dir, "seeall.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "\"cpu\"") {
		t.Errorf("the file still orders the removed monitor's cards:\n%s", data)
	}
}
