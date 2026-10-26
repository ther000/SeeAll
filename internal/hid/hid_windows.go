//go:build windows

package hid

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// Windows HID enumeration and reads, through setupapi.dll, hid.dll and
// kernel32.dll, with no cgo.
//
// Two facts from the hardware shape this file:
//   - probing a collection needs no access (access=0), but reading reports
//     needs GENERIC_READ: a handle opened without it never delivers data;
//   - the Rapoo receiver stops broadcasting once a write handle stays open,
//     so WriteConfig closes its handle as soon as the report is sent.

var (
	setupapi = syscall.NewLazyDLL("setupapi.dll")
	hid      = syscall.NewLazyDLL("hid.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procSetupDiGetClassDevsW             = setupapi.NewProc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceInterfaces      = setupapi.NewProc("SetupDiEnumDeviceInterfaces")
	procSetupDiGetDeviceInterfaceDetailW = setupapi.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procSetupDiDestroyDeviceInfoList     = setupapi.NewProc("SetupDiDestroyDeviceInfoList")

	procHidDGetHidGuid            = hid.NewProc("HidD_GetHidGuid")
	procHidDGetAttributes         = hid.NewProc("HidD_GetAttributes")
	procHidDGetPreparsedData      = hid.NewProc("HidD_GetPreparsedData")
	procHidDFreePreparsedData     = hid.NewProc("HidD_FreePreparsedData")
	procHidDGetProductString      = hid.NewProc("HidD_GetProductString")
	procHidDGetManufacturerString = hid.NewProc("HidD_GetManufacturerString")
	procHidPGetCaps               = hid.NewProc("HidP_GetCaps")
	procHidDSetOutputReport       = hid.NewProc("HidD_SetOutputReport")

	procCreateFileW         = kernel32.NewProc("CreateFileW")
	procReadFile            = kernel32.NewProc("ReadFile")
	procCancelIoEx          = kernel32.NewProc("CancelIoEx")
	procCreateEventW        = kernel32.NewProc("CreateEventW")
	procWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	procGetOverlappedResult = kernel32.NewProc("GetOverlappedResult")
)

const (
	digcfPresent         = 0x02
	digcfDeviceInterface = 0x10

	genericRead  uint32 = 0x8000_0000
	genericWrite uint32 = 0x4000_0000

	fileShareRead      uint32 = 0x0000_0001
	fileShareWrite     uint32 = 0x0000_0002
	openExisting       uint32 = 3
	fileFlagOverlapped uint32 = 0x4000_0000

	errorIOPending               = 997
	errorOperationAborted        = 995
	errorAccessDenied            = 5
	waitTimeout           uint32 = 258
	hidpStatusSuccess            = 0x0011_0000
)

// spDeviceInterfaceData mirrors SP_DEVICE_INTERFACE_DATA.
type spDeviceInterfaceData struct {
	Size      uint32
	ClassGUID syscall.GUID
	Flags     uint32
	Reserved  uintptr
}

// hiddAttributes mirrors HIDD_ATTRIBUTES.
type hiddAttributes struct {
	Size          uint32
	VendorID      uint16
	ProductID     uint16
	VersionNumber uint16
}

// hidpCaps mirrors HIDP_CAPS. The struct's layout must match hid.dll's, so
// the Reserved array is explicit rather than implied.
type hidpCaps struct {
	Usage                     uint16
	UsagePage                 uint16
	InputReportByteLength     uint16
	OutputReportByteLength    uint16
	FeatureReportByteLength   uint16
	Reserved                  [17]uint16
	NumberLinkCollectionNodes uint16
	NumberInputButtonCaps     uint16
	NumberInputValueCaps      uint16
	NumberInputDataIndices    uint16
	NumberOutputButtonCaps    uint16
	NumberOutputValueCaps     uint16
	NumberOutputDataIndices   uint16
	NumberFeatureButtonCaps   uint16
	NumberFeatureValueCaps    uint16
	NumberFeatureDataIndices  uint16
}

// overlapped mirrors the fields of OVERLAPPED this package needs.
type overlapped struct {
	Internal     uintptr
	InternalHigh uintptr
	Offset       uint32
	OffsetHigh   uint32
	HEvent       syscall.Handle
}

// Enumerate lists the HID collections that match filter, or all of them when
// filter is nil.
func Enumerate(filter func(Interface) bool) ([]Interface, error) {
	var guid syscall.GUID
	procHidDGetHidGuid.Call(uintptr(unsafe.Pointer(&guid)))

	set, _, err := procSetupDiGetClassDevsW.Call(
		uintptr(unsafe.Pointer(&guid)), 0, 0,
		uintptr(digcfPresent|digcfDeviceInterface),
	)
	if set == 0 || set == uintptr(^uintptr(0)) {
		return nil, fmt.Errorf("setupapi：枚举设备失败（%v）", err)
	}
	defer procSetupDiDestroyDeviceInfoList.Call(set)

	var out []Interface
	data := spDeviceInterfaceData{Size: uint32(unsafe.Sizeof(spDeviceInterfaceData{}))}
	for index := uint32(0); ; index++ {
		ok, _, _ := procSetupDiEnumDeviceInterfaces.Call(
			set, 0, uintptr(unsafe.Pointer(&guid)), uintptr(index), uintptr(unsafe.Pointer(&data)),
		)
		if ok == 0 {
			break
		}
		path, err := interfacePath(set, &data)
		if err != nil || path == "" {
			continue
		}
		intf, err := probe(path)
		if err != nil {
			// Collections the system owns cannot be opened; skip them.
			continue
		}
		if filter == nil || filter(intf) {
			out = append(out, intf)
		}
	}
	return out, nil
}

// interfacePath reads the device path of one enumerated interface.
func interfacePath(set uintptr, data *spDeviceInterfaceData) (string, error) {
	var required uint32
	procSetupDiGetDeviceInterfaceDetailW.Call(
		set, uintptr(unsafe.Pointer(data)), 0, 0, uintptr(unsafe.Pointer(&required)), 0,
	)
	if required == 0 {
		return "", fmt.Errorf("setupapi：读不到路径长度")
	}
	buf := make([]byte, required)
	// SP_DEVICE_INTERFACE_DETAIL_DATA_W.cbSize is 6 on 32-bit Windows and 8
	// on 64-bit, not what unsafe.Sizeof reports. The path itself always
	// starts at offset 4: its WCHAR needs two bytes of alignment, whatever
	// the pointer size. Reading it at the pointer size's offset gives a
	// truncated "?\hid#…".
	cb := uint32(6)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		cb = 8
	}
	*(*uint32)(unsafe.Pointer(&buf[0])) = cb
	const offset = 4
	ok, _, _ := procSetupDiGetDeviceInterfaceDetailW.Call(
		set, uintptr(unsafe.Pointer(data)), uintptr(unsafe.Pointer(&buf[0])),
		uintptr(required), uintptr(unsafe.Pointer(&required)), 0,
	)
	if ok == 0 {
		return "", fmt.Errorf("setupapi：读不到路径")
	}
	utf16 := (*[1 << 14]uint16)(unsafe.Pointer(&buf[offset]))
	n := 0
	for n < len(utf16) && utf16[n] != 0 {
		n++
	}
	return syscall.UTF16ToString(utf16[:n]), nil
}

// probe opens a collection without reading or writing, and asks hid.dll what
// it is.
func probe(path string) (Interface, error) {
	handle, err := openDevice(path, 0, 0)
	if err != nil {
		return Interface{}, err
	}
	defer syscall.CloseHandle(handle)

	var attrs hiddAttributes
	attrs.Size = uint32(unsafe.Sizeof(attrs))
	if ok, _, _ := procHidDGetAttributes.Call(uintptr(handle), uintptr(unsafe.Pointer(&attrs))); ok == 0 {
		return Interface{}, fmt.Errorf("hid：读不到设备属性")
	}

	var caps hidpCaps
	var preparsed uintptr
	ok, _, _ := procHidDGetPreparsedData.Call(uintptr(handle), uintptr(unsafe.Pointer(&preparsed)))
	if ok == 0 || preparsed == 0 {
		return Interface{}, fmt.Errorf("hid：读不到 preparsed data")
	}
	status, _, _ := procHidPGetCaps.Call(preparsed, uintptr(unsafe.Pointer(&caps)))
	procHidDFreePreparsedData.Call(preparsed)
	if status != hidpStatusSuccess {
		return Interface{}, fmt.Errorf("hid：读不到 caps")
	}

	return Interface{
		Path:                    path,
		VendorID:                attrs.VendorID,
		ProductID:               attrs.ProductID,
		UsagePage:               caps.UsagePage,
		Usage:                   caps.Usage,
		InputReportByteLength:   caps.InputReportByteLength,
		OutputReportByteLength:  caps.OutputReportByteLength,
		FeatureReportByteLength: caps.FeatureReportByteLength,
		Product:                 hidString(procHidDGetProductString, handle),
		Manufacturer:            hidString(procHidDGetManufacturerString, handle),
	}, nil
}

func hidString(proc *syscall.LazyProc, handle syscall.Handle) string {
	buf := make([]uint16, 128)
	ok, _, _ := proc.Call(uintptr(handle), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2))
	if ok == 0 {
		return ""
	}
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	return syscall.UTF16ToString(buf[:n])
}

func openDevice(path string, access, flags uint32) (syscall.Handle, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	handle, _, callErr := procCreateFileW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(access),
		uintptr(fileShareRead|fileShareWrite),
		0,
		uintptr(openExisting),
		uintptr(flags),
		0,
	)
	if handle == uintptr(syscall.InvalidHandle) {
		return 0, callErr
	}
	return syscall.Handle(handle), nil
}

// Reader listens to one collection's input reports.
type Reader struct {
	path    string
	length  int
	handle  syscall.Handle
	event   syscall.Handle
	buf     []byte
	overlap overlapped
	pending bool
}

// OpenReader starts listening to a collection.
func OpenReader(intf Interface) (*Reader, error) {
	handle, err := openDevice(intf.Path, genericRead, fileFlagOverlapped)
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == errorAccessDenied {
			return nil, fmt.Errorf("被其他程序占用")
		}
		return nil, fmt.Errorf("打开失败（%v）", err)
	}
	event, _, callErr := procCreateEventW.Call(0, 0, 0, 0)
	if event == 0 {
		syscall.CloseHandle(handle)
		return nil, fmt.Errorf("创建完成事件失败（%v）", callErr)
	}
	length := int(intf.InputReportByteLength)
	if length <= 0 {
		length = 64
	}
	return &Reader{
		path:   intf.Path,
		length: length,
		handle: handle,
		event:  syscall.Handle(event),
		buf:    make([]byte, length),
	}, nil
}

// Path returns the collection the reader listens to.
func (r *Reader) Path() string { return r.path }

// Read waits up to timeout for a report. It returns nil, nil on timeout, and
// an error when the collection has to be reopened.
func (r *Reader) Read(timeout time.Duration) ([]byte, error) {
	if r.handle == 0 {
		return nil, fmt.Errorf("通道已关闭")
	}
	if !r.pending {
		r.overlap = overlapped{HEvent: r.event}
		ok, _, callErr := procReadFile.Call(
			uintptr(r.handle),
			uintptr(unsafe.Pointer(&r.buf[0])),
			uintptr(len(r.buf)),
			0,
			uintptr(unsafe.Pointer(&r.overlap)),
		)
		if ok == 0 {
			if errno, isErrno := callErr.(syscall.Errno); !isErrno || errno != errorIOPending {
				return nil, fmt.Errorf("读报文失败（%v）", callErr)
			}
		}
		r.pending = true
	}

	millis := uint32(timeout / time.Millisecond)
	if millis == 0 {
		millis = 1
	}
	wait, _, _ := procWaitForSingleObject.Call(uintptr(r.event), uintptr(millis))
	if uint32(wait) == waitTimeout {
		return nil, nil
	}
	if wait != 0 {
		r.cancel()
		return nil, fmt.Errorf("读报文被打断")
	}

	var transferred uint32
	ok, _, callErr := procGetOverlappedResult.Call(
		uintptr(r.handle), uintptr(unsafe.Pointer(&r.overlap)), uintptr(unsafe.Pointer(&transferred)), 0,
	)
	r.pending = false
	if ok == 0 {
		if errno, isErrno := callErr.(syscall.Errno); isErrno && errno == errorOperationAborted {
			return nil, nil
		}
		return nil, fmt.Errorf("读报文失败（%v）", callErr)
	}
	if transferred == 0 {
		return nil, nil
	}
	out := make([]byte, transferred)
	copy(out, r.buf[:transferred])
	return out, nil
}

// Close stops listening and releases the collection.
func (r *Reader) Close() {
	if r.handle != 0 {
		r.cancel()
		syscall.CloseHandle(r.handle)
		r.handle = 0
	}
	if r.event != 0 {
		syscall.CloseHandle(r.event)
		r.event = 0
	}
}

func (r *Reader) cancel() {
	if r.handle == 0 || !r.pending {
		return
	}
	procCancelIoEx.Call(uintptr(r.handle), uintptr(unsafe.Pointer(&r.overlap)))
	procWaitForSingleObject.Call(uintptr(r.event), 500)
	r.pending = false
}

// WriteConfig sends one output report to a collection.
//
// Rapoo receivers take a vendor command whose first byte is the report ID,
// then a payload. The handle is closed as soon as the write returns: a
// receiver holding its write handle stops broadcasting the state frames the
// battery comes from.
func WriteConfig(intf Interface, payload []byte) error {
	reportID := RapooReportID(int(intf.OutputReportByteLength))
	if reportID == 0 {
		return fmt.Errorf("这个 collection 不接受写入")
	}
	buf := make([]byte, 0, int(intf.OutputReportByteLength))
	buf = append(buf, reportID)
	buf = append(buf, payload...)
	for len(buf) < int(intf.OutputReportByteLength) {
		buf = append(buf, 0)
	}

	handle, err := openDevice(intf.Path, genericWrite, 0)
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == errorAccessDenied {
			return fmt.Errorf("写入通道被占用（%v）", err)
		}
		return fmt.Errorf("打开写入通道失败（%v）", err)
	}
	defer syscall.CloseHandle(handle)

	if ok, _, callErr := procHidDSetOutputReport.Call(
		uintptr(handle), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
	); ok == 0 {
		return fmt.Errorf("下发命令失败（%v）", callErr)
	}
	return nil
}

// RapooReportID is the report ID of a Rapoo vendor collection, by the length
// of its output buffer: 32 bytes is 0xBA, 12 bytes 0xBC, 7 bytes 0xBB.
func RapooReportID(outputReportByteLength int) byte {
	switch {
	case outputReportByteLength >= 32:
		return 0xBA
	case outputReportByteLength >= 12:
		return 0xBC
	case outputReportByteLength >= 7:
		return 0xBB
	}
	return 0
}
