//go:build windows

package system

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// What the machine is, read once: a processor's name, the Windows release and
// the volume SeeAll runs from do not change while it runs. The registry is
// where Windows keeps all three.

var advapi32 = syscall.NewLazyDLL("advapi32.dll")

var procRegGetValueW = advapi32.NewProc("RegGetValueW")

const (
	hkeyLocalMachine = 0x80000002

	rrfRTRegSz    = 0x00000002
	rrfRTRegDword = 0x00000010

	processorKey = `HARDWARE\DESCRIPTION\System\CentralProcessor\0`
	windowsKey   = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`
)

// inventory is what the machine is, as opposed to what it is doing.
type inventory struct {
	// CPUName is the processor as the firmware names it.
	CPUName string
	// BaseMHz is the clock the processor booted at: the performance counter
	// reports the speed it runs at now against this one.
	BaseMHz float64
	// OSName is "Windows 11 24H2", or "" when the registry cannot be read.
	OSName string
	// Drive names the volume Windows is installed on, "C:", and DrivePath is
	// the path that volume is addressed by.
	Drive     string
	DrivePath string
	// BatteryHealth is the charge the battery holds as a share of the
	// capacity it was built with; HasBatteryHealth is false on a machine
	// without a battery that reports both numbers.
	BatteryHealth    float64
	HasBatteryHealth bool
}

// readInventory reads what the machine says about itself.
func readInventory() inventory {
	inv := inventory{
		CPUName: regString(processorKey, "ProcessorNameString"),
		BaseMHz: float64(regDword(processorKey, "~MHz")),
	}

	family := regString(windowsKey, "ProductName")
	// Windows 10 1809 and earlier call their release "ReleaseId".
	display := regString(windowsKey, "DisplayVersion")
	if display == "" {
		display = regString(windowsKey, "ReleaseId")
	}
	build, err := strconv.Atoi(regString(windowsKey, "CurrentBuildNumber"))
	if err != nil {
		build = regDword(windowsKey, "CurrentBuild")
	}
	inv.OSName = osPill(family, display, build)

	// The system drive is where the page's free-space card points, even on a
	// machine with a second, roomier volume.
	drive := strings.TrimSpace(os.Getenv("SystemDrive"))
	if drive == "" {
		drive = "C:"
	}
	inv.Drive, inv.DrivePath = drive, drive+`\`

	inv.BatteryHealth, inv.HasBatteryHealth = batteryHealth()
	return inv
}

// regString reads a string value, or "" when the key or the value is not
// there. The registry is the machine's own word, and a key that moved between
// releases is a fact the page leaves out rather than one it guesses.
func regString(key, name string) string {
	sub, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		return ""
	}
	value, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return ""
	}
	buf := make([]uint16, 512)
	size := uint32(len(buf) * 2)
	status, _, _ := procRegGetValueW.Call(hkeyLocalMachine,
		uintptr(unsafe.Pointer(sub)), uintptr(unsafe.Pointer(value)),
		rrfRTRegSz, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if status != 0 {
		return ""
	}
	return strings.TrimSpace(syscall.UTF16ToString(buf))
}

// regDword reads a number value, or 0 when it is not there.
func regDword(key, name string) int {
	sub, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		return 0
	}
	value, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0
	}
	var out uint32
	size := uint32(unsafe.Sizeof(out))
	status, _, _ := procRegGetValueW.Call(hkeyLocalMachine,
		uintptr(unsafe.Pointer(sub)), uintptr(unsafe.Pointer(value)),
		rrfRTRegDword, 0,
		uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&size)))
	if status != 0 {
		return 0
	}
	return int(out)
}
