// Package hid reads battery levels from HID devices: mice and keyboards that
// report their charge over a vendor collection, as Rapoo's 2.4G receivers do.
//
// The Windows implementation talks to setupapi/hid.dll directly, with no
// cgo. Other systems compile the stubs, so the app keeps building and the
// panel says the platform cannot read device batteries yet.
package hid

import (
	"errors"
	"strings"
	"time"
)

// Interface is one HID collection found on the machine.
type Interface struct {
	// Path is the device interface path CreateFile takes.
	Path string
	// VendorID and ProductID identify the device: 0x24AE and 0x185A for a
	// Rapoo VT9 PRO receiver.
	VendorID  uint16
	ProductID uint16
	// UsagePage and Usage of the collection; 0xFF00/0x0002 is where a
	// Rapoo mouse broadcasts.
	UsagePage uint16
	Usage     uint16
	// InputReportByteLength is the size of the collection's input reports,
	// without the report ID prefix.
	InputReportByteLength   uint16
	OutputReportByteLength  uint16
	FeatureReportByteLength uint16
	// Product is the device's product string, such as "Rapoo Gaming Device".
	Product string
	// Manufacturer is the device's manufacturer string, when it has one.
	Manufacturer string
}

// Describe writes the interface the way the diagnostics page shows it.
func (i Interface) Describe() string {
	return strings.Join([]string{
		hex4(i.VendorID) + ":" + hex4(i.ProductID),
		"usage=" + hex4(i.UsagePage) + "/" + hex4(i.Usage),
	}, " ")
}

func hex4(v uint16) string {
	const digits = "0123456789ABCDEF"
	return string([]byte{
		digits[(v>>12)&0xF], digits[(v>>8)&0xF], digits[(v>>4)&0xF], digits[v&0xF],
	})
}

// ErrReport is read from a device whose stream does not look like a report.
var ErrReport = errors.New("unexpected report")

// Report is one input report read from a collection.
type Report struct {
	// Data is the report, without the report ID prefix.
	Data []byte
	// At is when it was read.
	At time.Time
}
