//go:build windows

package system

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"seeall/internal/plugin"
)

// The graphics cards, through DXGI: what each one is called, how much memory
// it has, and which LUID its performance counters use.
//
// The counters of a card are per adapter — a processor with two graphics
// engines reports two sets of them — so the page has to know which card it
// is about. DXGI is the same list the task manager's Performance tab shows.

const (
	gpuAuto       = "auto"
	gpuEnginePath = `\GPU Engine(*)\Utilization Percentage`
	gpuMemoryPath = `\GPU Adapter Memory(*)\Dedicated Usage`
)

var (
	// cardsMu guards the list of cards, which the panel reads on the main
	// thread while the sampler reads it on its own.
	cardsMu sync.RWMutex
	// cards is the list graphicsAdapters last read, nil until it is asked.
	cards []gpuAdapter

	dxgi                   = syscall.NewLazyDLL("dxgi.dll")
	procCreateDXGIFactory1 = dxgi.NewProc("CreateDXGIFactory1")

	iidDXGIFactory1 = guid{0x770aae78, 0xf26f, 0x4dba,
		[8]byte{0xa8, 0x29, 0x25, 0x3c, 0x83, 0xd1, 0xb3, 0x87}}
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// luid identifies an adapter for as long as the machine is up.
type luid struct {
	Low  uint32
	High uint32
}

// adapterDesc is DXGI_ADAPTER_DESC1: the card's name, its memory and its LUID.
type adapterDesc struct {
	Description          [128]uint16
	VendorID             uint32
	DeviceID             uint32
	SubSysID             uint32
	Revision             uint32
	DedicatedVideoMemory uintptr
	DedicatedSysMemory   uintptr
	SharedSystemMemory   uintptr
	AdapterLuid          luid
	Flags                uint32
}

// gpuAdapter is one card SeeAll can count.
type gpuAdapter struct {
	// Key names the card across restarts, which a LUID cannot: it changes
	// with every boot. A laptop can list one card twice, so the key is the
	// card's IDs and which of them it is.
	Key  string
	Name string
	// Luid is what the performance counters call the card; zero for a
	// sampler that found no card at all.
	Luid uint64
	// Memory is the card's own memory, in bytes.
	Memory float64
}

// isOurs reports whether a counter instance belongs to this card, given its
// name such as "pid_12772_luid_0x00000000_0x0000E3CD_phys_0_eng_0_...".
// Without a card — a machine whose graphics could not be enumerated — every
// instance counts: the page then shows the machine's graphics without
// claiming to know which processor it was.
func (a gpuAdapter) isOurs(name string) bool {
	if a.Luid == 0 {
		return true
	}
	luid, ok := parseLuid(name)
	return ok && luid == a.Luid
}

// graphicsAdapters lists the cards the GPU counters report on. An adapter
// that reports no engine has nothing to measure — a card the machine has
// switched off, and the second half of a pair DXGI lists for one processor —
// so it is not offered in the settings either.
//
// The list is read once and kept: the settings page asks for it on every frame
// it draws. refreshAdapters reads it again, which the sampler does when the
// card the settings name is not in the list — a card that was switched off
// when SeeAll started is worth one second look before the page says it is
// gone.
func graphicsAdapters() []gpuAdapter {
	cardsMu.RLock()
	known := cards
	cardsMu.RUnlock()
	if known != nil {
		return known
	}
	return refreshAdapters()
}

// refreshAdapters reads the machine's cards again, and returns the new list.
func refreshAdapters() []gpuAdapter {
	probed := probeAdapters()
	if probed == nil {
		probed = []gpuAdapter{}
	}
	cardsMu.Lock()
	cards = probed
	cardsMu.Unlock()
	return probed
}

func probeAdapters() []gpuAdapter {
	all := enumAdapters()
	if len(all) == 0 {
		return nil
	}
	known := engineLuids()
	if len(known) == 0 {
		// No engine counters at all: the cards still have memory to report,
		// and hiding them would leave the page with no GPU at all.
		return all
	}
	out := make([]gpuAdapter, 0, len(all))
	for _, a := range all {
		if known[a.Luid] {
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		return all
	}
	return out
}

// gpuChoices is the settings page's list: every card, after "automatic".
func gpuChoices() []plugin.Option {
	out := []plugin.Option{{Value: gpuAuto, Label: "自动"}}
	used := map[string]int{}
	for _, a := range graphicsAdapters() {
		label := shortGPUName(a.Name)
		used[label]++
		if n := used[label]; n > 1 {
			label = fmt.Sprintf("%s %d", label, n)
		}
		out = append(out, plugin.Option{Value: a.Key, Label: label})
	}
	return out
}

// enumAdapters reads every graphics card DXGI knows, in its order.
func enumAdapters() []gpuAdapter {
	var factory unsafe.Pointer
	hr, _, _ := procCreateDXGIFactory1.Call(
		uintptr(unsafe.Pointer(&iidDXGIFactory1)), uintptr(unsafe.Pointer(&factory)))
	if int32(hr) < 0 || factory == nil {
		return nil
	}
	defer comRelease(factory)

	seen := map[string]int{}
	var out []gpuAdapter
	for i := 0; i < 32; i++ {
		var adapter unsafe.Pointer
		if _, err := comCall(factory, 12 /* EnumAdapters1 */, uintptr(i), uintptr(unsafe.Pointer(&adapter))); err != nil || adapter == nil {
			break
		}
		var desc adapterDesc
		_, err := comCall(adapter, 10 /* GetDesc1 */, uintptr(unsafe.Pointer(&desc)))
		comRelease(adapter)
		if err != nil {
			continue
		}
		// DXGI_ADAPTER_FLAG_SOFTWARE: the render driver, not a card.
		if desc.Flags&2 != 0 {
			continue
		}
		name := syscall.UTF16ToString(desc.Description[:])
		if name == "" {
			continue
		}
		pair := fmt.Sprintf("%04X:%04X", desc.VendorID, desc.DeviceID)
		key := fmt.Sprintf("%s#%d", pair, seen[pair])
		seen[pair]++
		out = append(out, gpuAdapter{
			Key:    key,
			Name:   name,
			Luid:   uint64(desc.AdapterLuid.High)<<32 | uint64(desc.AdapterLuid.Low),
			Memory: float64(desc.DedicatedVideoMemory),
		})
	}
	return out
}

// engineLuids reads which adapters have engine counters. A rate counter has
// no instances until it has seen two collects, so the query is read twice with
// a short gap: this runs once per process, and the list of a machine's cards
// does not change under it.
func engineLuids() map[uint64]bool {
	q := openQuery()
	if q == nil {
		return nil
	}
	defer q.close()
	if !q.add(gpuEnginePath, true) {
		return nil
	}
	for attempt := 0; attempt < 3; attempt++ {
		q.refresh()
		if values := q.instanceValues(q.counters[0]); len(values) > 0 {
			out := map[uint64]bool{}
			for _, inst := range values {
				if luid, ok := parseLuid(inst.Name); ok {
					out[luid] = true
				}
			}
			return out
		}
		time.Sleep(120 * time.Millisecond)
	}
	return nil
}

// comCall calls the index'th method of a COM object: a pointer to the object,
// whose first word points at its table of methods.
func comCall(obj unsafe.Pointer, index int, args ...uintptr) (uintptr, error) {
	fn := comMethod(obj, index)
	all := append([]uintptr{uintptr(obj)}, args...)
	r, _, _ := syscall.SyscallN(fn, all...)
	if int32(r) < 0 {
		return r, fmt.Errorf("HRESULT %#x", uint32(r))
	}
	return r, nil
}

// comMethod reads one function pointer out of a COM object's table.
func comMethod(obj unsafe.Pointer, index int) uintptr {
	vtbl := *(**uintptr)(obj)
	return (*[64]uintptr)(unsafe.Pointer(vtbl))[index]
}

// comRelease drops a COM object, which the callers defer.
func comRelease(obj unsafe.Pointer) {
	if obj != nil {
		comCall(obj, 2 /* Release */)
	}
}
