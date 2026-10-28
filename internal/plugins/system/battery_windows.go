//go:build windows

package system

import (
	"syscall"
	"unsafe"
)

// A battery's health is the charge it holds against the capacity it was built
// with, and only the battery class driver knows both: the power status API
// reports the charge, never the wear.
//
// The query is one device's information, read once when SeeAll starts: a
// battery does not wear noticeably while the program runs.

var cfgmgr32 = syscall.NewLazyDLL("cfgmgr32.dll")

var (
	procCMGetDeviceInterfaceListSize = cfgmgr32.NewProc("CM_Get_Device_Interface_List_SizeW")
	procCMGetDeviceInterfaceList     = cfgmgr32.NewProc("CM_Get_Device_Interface_ListW")
	procDeviceIoControl              = kernel32.NewProc("DeviceIoControl")
	procCreateFileW                  = kernel32.NewProc("CreateFileW")
)

// guidBattery is GUID_DEVINTERFACE_BATTERY: the class every battery belongs to.
var guidBattery = guid{0x72631E54, 0x78A4, 0x11D0,
	[8]byte{0xBC, 0xF7, 0x00, 0xAA, 0x00, 0xB7, 0xB3, 0x2A}}

const (
	cmGetDeviceInterfaceListPresent = 0

	ioctlBatteryQueryTag         = 0x294040
	ioctlBatteryQueryInformation = 0x294044

	batteryInformationLevel = 0

	genericRead        = 0x80000000
	fileShareReadWrite = 0x00000003
	openExisting       = 3
)

// batteryQueryInformation is the input of IOCTL_BATTERY_QUERY_INFORMATION.
type batteryQueryInformation struct {
	BatteryTag       uint32
	InformationLevel uint32
	AtRate           int32
}

// batteryInformation is the driver's answer, with capacities in mWh.
type batteryInformation struct {
	Capabilities        uint32
	Technology          byte
	Reserved            [3]byte
	Chemistry           [4]byte
	DesignedCapacity    uint32
	FullChargedCapacity uint32
	DefaultAlert1       uint32
	DefaultAlert2       uint32
	CriticalBias        uint32
	CyclesCount         uint32
}

// batteryHealth reads how much of its original capacity the battery still
// holds, and reports false when the machine has no battery that says.
func batteryHealth() (float64, bool) {
	for _, path := range batteryInterfaces() {
		if health, ok := batteryHealthAt(path); ok {
			return health, true
		}
	}
	return 0, false
}

// batteryInterfaces lists the battery device interfaces the machine has.
func batteryInterfaces() []string {
	var size uint32
	if status, _, _ := procCMGetDeviceInterfaceListSize.Call(
		uintptr(unsafe.Pointer(&size)),
		uintptr(unsafe.Pointer(&guidBattery)), 0,
		cmGetDeviceInterfaceListPresent); status != 0 || size == 0 {
		return nil
	}
	buf := make([]uint16, size)
	if status, _, _ := procCMGetDeviceInterfaceList.Call(
		uintptr(unsafe.Pointer(&guidBattery)), 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(size),
		cmGetDeviceInterfaceListPresent); status != 0 {
		return nil
	}
	// The list is paths, each ended by a null, and ended twice.
	var out []string
	for i := 0; i < len(buf); {
		end := i
		for end < len(buf) && buf[end] != 0 {
			end++
		}
		if end == i {
			break
		}
		out = append(out, syscall.UTF16ToString(buf[i:end]))
		i = end + 1
	}
	return out
}

// batteryHealthAt opens one battery and asks it for the pair of capacities.
func batteryHealthAt(path string) (float64, bool) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	handle, _, _ := procCreateFileW.Call(uintptr(unsafe.Pointer(p)), genericRead,
		fileShareReadWrite, 0, openExisting, 0, 0)
	if handle == 0 || handle == invalidHandleValue {
		return 0, false
	}
	defer procCloseHandle.Call(handle)

	// The tag names the battery's current state; the information query below
	// is refused without it. A zero timeout asks for the state now rather
	// than waiting for the battery to change.
	var returned uint32
	var tag, noWait uint32
	if ok, _, _ := procDeviceIoControl.Call(handle, ioctlBatteryQueryTag,
		uintptr(unsafe.Pointer(&noWait)), unsafe.Sizeof(noWait),
		uintptr(unsafe.Pointer(&tag)), unsafe.Sizeof(tag),
		uintptr(unsafe.Pointer(&returned)), 0); ok == 0 {
		return 0, false
	}

	query := batteryQueryInformation{BatteryTag: tag, InformationLevel: batteryInformationLevel}
	var info batteryInformation
	if ok, _, _ := procDeviceIoControl.Call(handle, ioctlBatteryQueryInformation,
		uintptr(unsafe.Pointer(&query)), unsafe.Sizeof(query),
		uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info),
		uintptr(unsafe.Pointer(&returned)), 0); ok == 0 {
		return 0, false
	}
	if info.DesignedCapacity == 0 || info.FullChargedCapacity == 0 {
		return 0, false
	}
	health := 100 * float64(info.FullChargedCapacity) / float64(info.DesignedCapacity)
	// A driver that reports the two capacities the wrong way round would make
	// the battery look newer than it is; past 150% the numbers are not a
	// health reading.
	if health <= 0 || health > 150 {
		return 0, false
	}
	return health, true
}
