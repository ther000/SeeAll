//go:build windows

package system

import (
	"syscall"
	"unsafe"
)

// adapterMemory reads the largest graphics adapter's memory from the registry,
// where the driver records it: WMI's AdapterRAM is a 32-bit field that wraps at
// 4 GB, so a 6 GB laptop GPU reports less than it has.
//
// The value is the adapter's own memory, which is what the GPU memory counters
// report their usage against.
func adapterMemory() float64 {
	const classKey = `SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`

	root, err := syscall.UTF16PtrFromString(classKey)
	if err != nil {
		return 0
	}
	var key syscall.Handle
	if status, _, _ := procRegOpenKeyEx.Call(hkeyLocalMachine,
		uintptr(unsafe.Pointer(root)), 0, keyRead, uintptr(unsafe.Pointer(&key))); status != 0 {
		return 0
	}
	defer procRegCloseKey.Call(uintptr(key))

	var best float64
	for i := 0; ; i++ {
		var nameBuf [256]uint16
		size := uint32(len(nameBuf))
		status, _, _ := procRegEnumKeyEx.Call(uintptr(key), uintptr(i),
			uintptr(unsafe.Pointer(&nameBuf[0])), uintptr(unsafe.Pointer(&size)),
			0, 0, 0, 0)
		if status != 0 {
			break
		}
		name := syscall.UTF16ToString(nameBuf[:size])
		// The class key holds the adapters under four-digit numbers and its
		// own settings under names.
		if len(name) != 4 || name[0] < '0' || name[0] > '9' {
			continue
		}
		if size := adapterMemoryOf(key, name); size > best {
			best = size
		}
	}
	return best
}

// adapterMemoryOf reads one adapter's HardwareInformation.qwMemorySize, a value
// of the adapter's own key rather than a subkey.
func adapterMemoryOf(parent syscall.Handle, sub string) float64 {
	path, err := syscall.UTF16PtrFromString(sub)
	if err != nil {
		return 0
	}
	var key syscall.Handle
	if status, _, _ := procRegOpenKeyEx.Call(uintptr(parent),
		uintptr(unsafe.Pointer(path)), 0, keyRead, uintptr(unsafe.Pointer(&key))); status != 0 {
		return 0
	}
	defer procRegCloseKey.Call(uintptr(key))

	value, err := syscall.UTF16PtrFromString("HardwareInformation.qwMemorySize")
	if err != nil {
		return 0
	}
	var data uint64
	size := uint32(8)
	var kind uint32
	if status, _, _ := procRegQueryValueEx.Call(uintptr(key),
		uintptr(unsafe.Pointer(value)), 0, uintptr(unsafe.Pointer(&kind)),
		uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&size))); status != 0 {
		return 0
	}
	return float64(data)
}
