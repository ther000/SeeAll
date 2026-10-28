//go:build windows

package system

import (
	"syscall"
	"unsafe"
)

// Performance counters through pdh.dll, with no cgo: the network's byte rates
// and the graphics processor's load and memory are only told there.
//
// A counter object is opened once and its counters refreshed on every sample.
// PDH needs two reads of a rate counter before it has a rate, so the first
// sample of a session reports nothing and the numbers settle a tick later.

var pdh = syscall.NewLazyDLL("pdh.dll")

var (
	procPdhOpenQuery                = pdh.NewProc("PdhOpenQueryW")
	procPdhCloseQuery               = pdh.NewProc("PdhCloseQuery")
	procPdhAddCounter               = pdh.NewProc("PdhAddCounterW")
	procPdhAddEnglishCounter        = pdh.NewProc("PdhAddEnglishCounterW")
	procPdhCollectQueryData         = pdh.NewProc("PdhCollectQueryData")
	procPdhGetFormattedCounterValue = pdh.NewProc("PdhGetFormattedCounterValue")
	procPdhGetFormattedCounterArray = pdh.NewProc("PdhGetFormattedCounterArrayW")
)

const (
	pdhFmtDouble = 0x00000200
	pdhMoreData  = 0x800007D2
	pdhNoData    = 0x800007D5
)

// pdhCounter is one counter in a query.
type pdhCounter struct {
	handle uintptr
	path   string
	// array asks for every instance's value rather than the counter's own.
	array bool
}

// pdhQuery is a set of counters read together.
type pdhQuery struct {
	handle   uintptr
	counters []*pdhCounter
}

// pdhInstance is one instance of a counter: the network interface or the
// graphics adapter it belongs to, and what it read.
type pdhInstance struct {
	Name  string
	Value float64
}

// add collects path into a query, and reports whether it worked. A counter
// the machine does not have leaves the query open with the others: a machine
// without a GPU still reports its network.
func (q *pdhQuery) add(path string, array bool) bool {
	return q.addWith(procPdhAddCounter, path, array)
}

// addEnglish collects a path whose counter is named in English, and falls back
// to the localized name. A Chinese Windows does not answer to "Bytes
// Received/sec" when the counter is added by that name, only when it is added
// as an English counter; a machine whose PDH predates that API is the other
// way round.
func (q *pdhQuery) addEnglish(path string, array bool) bool {
	if procPdhAddEnglishCounter.Find() == nil && q.addWith(procPdhAddEnglishCounter, path, array) {
		return true
	}
	return q.addWith(procPdhAddCounter, path, array)
}

// addWith is add through one of the two entry points.
func (q *pdhQuery) addWith(proc *syscall.LazyProc, path string, array bool) bool {
	var c uintptr
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	if status, _, _ := proc.Call(q.handle, uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&c))); status != 0 {
		return false
	}
	q.counters = append(q.counters, &pdhCounter{handle: c, path: path, array: array})
	return true
}

func openQuery() *pdhQuery {
	var h uintptr
	if status, _, _ := procPdhOpenQuery.Call(0, 0, uintptr(unsafe.Pointer(&h))); status != 0 {
		return nil
	}
	return &pdhQuery{handle: h}
}

func (q *pdhQuery) close() {
	if q != nil && q.handle != 0 {
		procPdhCloseQuery.Call(q.handle)
		q.handle = 0
	}
}

// refresh reads every counter once, so that the next pass has values.
func (q *pdhQuery) refresh() bool {
	if q == nil || q.handle == 0 {
		return false
	}
	status, _, _ := procPdhCollectQueryData.Call(q.handle)
	return status == 0
}

// value reads one counter's last value.
func (q *pdhQuery) value(c *pdhCounter) (float64, bool) {
	if c == nil || c.array {
		return 0, false
	}
	var v struct {
		Status uint32
		Value  float64
	}
	status, _, _ := procPdhGetFormattedCounterValue.Call(
		c.handle, pdhFmtDouble, 0, uintptr(unsafe.Pointer(&v)))
	if status != 0 || v.Status != 0 {
		return 0, false
	}
	return v.Value, true
}

// instanceValues reads one counter for each of its instances, named: an
// adapter's counters have to be told from another's.
func (q *pdhQuery) instanceValues(c *pdhCounter) []pdhInstance {
	if c == nil || !c.array {
		return nil
	}
	var size uint32
	var count uint32
	status, _, _ := procPdhGetFormattedCounterArray.Call(c.handle, pdhFmtDouble,
		uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), 0)
	if uint32(status) != pdhMoreData {
		return nil
	}
	if size == 0 || count == 0 {
		return nil
	}
	buf := make([]byte, size)
	status, _, _ = procPdhGetFormattedCounterArray.Call(c.handle, pdhFmtDouble,
		uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&buf[0])))
	if status != 0 {
		return nil
	}

	// Each item is a pointer to its name, then its value.
	type item struct {
		name   *uint16
		status uint32
		value  float64
	}
	stride := int(unsafe.Sizeof(item{}))
	out := make([]pdhInstance, 0, count)
	for i := 0; i < int(count); i++ {
		it := (*item)(unsafe.Pointer(&buf[i*stride]))
		if it.status != 0 || it.name == nil {
			continue
		}
		name := syscall.UTF16ToString(unsafe.Slice(it.name, 256))
		out = append(out, pdhInstance{Name: name, Value: it.value})
	}
	return out
}
