//go:build windows

package system

import (
	"syscall"
	"unsafe"
)

// A volume's free space and its name, through kernel32. Only the volume
// Windows is installed on is watched: the page's card says "how much room is
// left where the system lives", not "how much across every disk".

var (
	procGetDiskFreeSpaceExW   = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetVolumeInformationW = kernel32.NewProc("GetVolumeInformationW")
)

const maxVolumeLabel = 261

// volumeSpace reads how full a volume is and what it is called. The free
// number is what the calling user can still write, which is what Windows'
// own drive view shows.
func volumeSpace(path string) (free, total float64, label string, ok bool) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, "", false
	}
	var available, totalBytes, totalFree uint64
	r, _, _ := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&available)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFree)))
	if r == 0 || totalBytes == 0 {
		return 0, 0, "", false
	}
	free, total = float64(available), float64(totalBytes)

	// The label is decoration: a volume without one still has free space.
	var name [maxVolumeLabel]uint16
	var serial, componentLength, flags uint32
	if got, _, _ := procGetVolumeInformationW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)),
		uintptr(unsafe.Pointer(&serial)),
		uintptr(unsafe.Pointer(&componentLength)),
		uintptr(unsafe.Pointer(&flags)),
		0, 0); got != 0 {
		label = syscall.UTF16ToString(name[:])
	}
	return free, total, label, true
}
