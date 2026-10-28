// Package mouse watches the battery and settings of a wireless mouse that
// broadcasts its state over a vendor HID collection, as the Rapoo VT9 PRO
// does through its 2.4G receiver.
package mouse

import (
	"seeall/internal/core"
	"seeall/internal/hid"
)

// The Rapoo state frame, verified on a VT9 PRO (VID:PID 24AE:185A and
// 24AE:418A):
//
//	BB B0 41 B0 04 02 64
//	│  │  │  └─┬─┘ │  └─ battery 0..100
//	│  │  │    │   └─── link flag: 0x01 wireless, 0x02 wired
//	│  │  │    └─────── DPI, u16 little endian
//	│  │  └──────────── model / channel tag
//	│  └─────────────── frame type: state broadcast
//	└────────────────── report ID
const (
	// batteryUsagePage is the vendor page the state broadcasts on.
	batteryUsagePage uint16 = 0xFF00
	// batteryUsage is the usage of the collecting that broadcasts.
	batteryUsage uint16 = 0x0002
	// frameLength is the smallest state frame the panel parses.
	frameLength = 7
	// batteryOffset is where the percentage sits.
	batteryOffset = 6
)

// IsBatteryChannel reports whether a HID collection is one a wireless mouse
// broadcasts its state on.
//
// The usage page and usage belong to the protocol, not to a brand, so the
// vendor ID is deliberately not part of the test: a receiver nobody has typed
// a number for is found the same way as the one this package was written
// against. A collection that matches but speaks another dialect never parses
// into a frame, which is what keeps this from picking up the wrong device.
func IsBatteryChannel(i hid.Interface) bool {
	return i.UsagePage == batteryUsagePage && i.Usage == batteryUsage
}

// Frame is one parsed state broadcast.
type Frame struct {
	// Battery is the percentage, 0..100.
	Battery int
	// DPI is the current resolution, from the frame itself: this receiver
	// has no read-back channel, so the frame is the only trustworthy
	// source for it.
	DPI int
	// DeviceTag is byte 2, which changes between models and channels.
	DeviceTag byte
	// Link is byte 5: 0x01 wireless, 0x02 wired.
	Link byte
	// Raw is the report the frame came from.
	Raw []byte
}

// Wired reports whether the mouse is charging over its cable.
func (f Frame) Wired() bool { return f.Link == 0x02 }

// LinkText names the link for people.
func (f Frame) LinkText() string {
	switch f.Link {
	case 0x01:
		return "无线"
	case 0x02:
		return "有线"
	default:
		return "未知连接"
	}
}

// ParseFrame reads one report, or returns false when it is not a state
// broadcast.
func ParseFrame(report []byte) (Frame, bool) {
	if len(report) < frameLength {
		return Frame{}, false
	}
	if report[0] != 0xBB || report[1] != 0xB0 {
		return Frame{}, false
	}
	level := int(report[batteryOffset])
	if level < 0 || level > 100 {
		return Frame{}, false
	}
	return Frame{
		Battery:   level,
		DPI:       int(report[3]) | int(report[4])<<8,
		DeviceTag: report[2],
		Link:      report[5],
		Raw:       append([]byte(nil), report...),
	}, true
}

// batterySeverity colors a battery level: below 10% it is critical, below
// 30% it is worth a look.
func batterySeverity(level int) core.Severity {
	switch {
	case level <= 10:
		return core.SeverityCritical
	case level <= 30:
		return core.SeverityWarn
	default:
		return core.SeverityOK
	}
}

// mouseHealth is the number the dashboard leads with: the charge itself, and
// the one word that says what it means for a mouse you have to pick up. Zero is
// reserved in core for "this source does not score itself", which a mouse with
// no frame already is, so a reported empty battery scores 1 rather than 0.
func mouseHealth(level int) (int, string) {
	switch {
	case level < 1:
		level = 1
	case level > 100:
		level = 100
	}
	switch {
	case level >= 60:
		return level, "电量充足"
	case level >= 30:
		return level, "电量偏低"
	}
	return level, "需要充电"
}
