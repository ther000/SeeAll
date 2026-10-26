//go:build !windows

package hid

import "time"

// HID device batteries are read on Windows only for now. On other systems
// the app still builds and the device plugins report that the platform has
// nothing to read.

// Enumerate lists HID collections; elsewhere it returns none.
func Enumerate(filter func(Interface) bool) ([]Interface, error) { return nil, nil }

// Reader listens to one collection; elsewhere there is none.
type Reader struct{}

// OpenReader fails on systems without HID support.
func OpenReader(intf Interface) (*Reader, error) { return &Reader{}, nil }

// Read never returns a report.
func (r *Reader) Read(timeout time.Duration) ([]byte, error) { return nil, nil }

// Close releases the reader.
func (r *Reader) Close() {}

// Path returns the collection the reader listens to.
func (r *Reader) Path() string { return "" }

// WriteConfig fails on systems without HID support.
func WriteConfig(intf Interface, payload []byte) error { return nil }

// RapooReportID returns 0: no Rapoo collections exist here.
func RapooReportID(outputReportByteLength int) byte { return 0 }
