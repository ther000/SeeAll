// Package core holds the reading model every plugin shares with the app.
package core

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Severity says how a single reading looks: the color of its number and of
// the badge beside it.
type Severity int

const (
	// SeverityNone leaves a reading uncolored.
	SeverityNone Severity = iota
	// SeverityOK is a healthy reading.
	SeverityOK
	// SeverityInfo marks something worth reading, not a problem.
	SeverityInfo
	// SeverityWarn is a value the user should look at soon.
	SeverityWarn
	// SeverityCritical is a value that is wrong now.
	SeverityCritical
	// SeverityOffline is a source that has nothing to report.
	SeverityOffline
)

func (s Severity) String() string {
	switch s {
	case SeverityOK:
		return "ok"
	case SeverityInfo:
		return "info"
	case SeverityWarn:
		return "warn"
	case SeverityCritical:
		return "critical"
	case SeverityOffline:
		return "offline"
	}
	return "none"
}

// Status is the state of a whole source: its dot in the sidebar and the
// headline of its page.
type Status int

const (
	// StatusStarting is a source that has not reported yet.
	StatusStarting Status = iota
	// StatusOK is a source whose last poll succeeded and found no problem.
	StatusOK
	// StatusWarn is a source reporting a degraded value.
	StatusWarn
	// StatusError is a source whose last poll failed.
	StatusError
	// StatusOffline is a source that reports nothing, such as a disconnected
	// device or a source the user turned off.
	StatusOffline
)

func (s Status) String() string {
	switch s {
	case StatusOK:
		return "正常"
	case StatusWarn:
		return "注意"
	case StatusError:
		return "出错"
	case StatusOffline:
		return "离线"
	}
	return "等待中"
}

// Unit is how a reading's number is written.
type Unit string

const (
	UnitNone    Unit = ""
	UnitPercent Unit = "%"
	UnitHz      Unit = "Hz"
	UnitBitmap  Unit = "bit/s"
	UnitBytes   Unit = "B"
	UnitMillis  Unit = "ms"
	UnitSeconds Unit = "s"
	UnitMinutes Unit = "min"
	UnitCelsius Unit = "°C"
	UnitWatt    Unit = "W"
	UnitRPM     Unit = "rpm"
	UnitDay     Unit = "天"
	UnitVolt    Unit = "V"
)

// Reading is one value a source reports, such as a battery level or the
// CPU load.
type Reading struct {
	// ID is stable across polls within its source, such as "battery.level".
	ID string
	// Label names the reading for people: "电量".
	Label string
	// Value is the number, in Unit.
	Value float64
	// Unit says how to write Value.
	Unit Unit
	// Text overrides what people see when the number needs words, such as
	// "已连接" or a device's name.
	Text string
	// Severity colors it.
	Severity Severity
	// Detail is a shorter explanation: "2 秒前", "192.168.1.7".
	Detail string
	// At is when the reading was taken; zero means the snapshot's time.
	At time.Time
}

// Format renders the reading for people.
func (r Reading) Format() string {
	if r.Text != "" {
		return r.Text
	}
	return FormatValue(r.Value, r.Unit)
}

// FormatNumber writes a number with as few decimals as it needs.
func FormatNumber(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "—"
	}
	switch {
	case v == math.Trunc(v) && math.Abs(v) < 1e15:
		return formatInt(v)
	}
	if math.Abs(v) >= 100 {
		return fmt.Sprintf("%.0f", v)
	}
	if math.Abs(v) >= 10 {
		return trimZeros(fmt.Sprintf("%.1f", v))
	}
	return trimZeros(fmt.Sprintf("%.2f", v))
}

func formatInt(v float64) string {
	return fmt.Sprintf("%.0f", v)
}

func trimZeros(s string) string {
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// FormatValue writes a value and its unit the way the panel does: no space
// before a percent sign, a space before a word unit.
func FormatValue(v float64, u Unit) string {
	switch u {
	case UnitNone:
		return FormatNumber(v)
	case UnitPercent:
		return FormatNumber(v) + "%"
	case UnitBytes:
		return FormatBytes(v)
	case UnitBitmap:
		return FormatBytes(v) + "/s"
	case UnitMillis:
		return FormatMillis(v)
	case UnitSeconds:
		return FormatNumber(v) + "s"
	default:
		return FormatNumber(v) + " " + string(u)
	}
}

var byteUnits = []string{"B", "KB", "MB", "GB", "TB", "PB"}

// FormatBytes writes a byte count in the largest unit that keeps it short.
func FormatBytes(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "—"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	i := 0
	for v >= 1024 && i < len(byteUnits)-1 {
		v /= 1024
		i++
	}
	s := FormatNumber(v)
	if i > 0 && v < 10 {
		s = trimZeros(fmt.Sprintf("%.1f", v))
	}
	if neg {
		s = "-" + s
	}
	return s + " " + byteUnits[i]
}

// FormatMillis writes a duration in milliseconds in the unit that reads best.
func FormatMillis(ms float64) string {
	switch {
	case ms < 1000:
		return FormatNumber(ms) + " ms"
	case ms < 60_000:
		return trimZeros(fmt.Sprintf("%.2f", ms/1000)) + " s"
	case ms < 3_600_000:
		return trimZeros(fmt.Sprintf("%.1f", ms/60_000)) + " min"
	default:
		return trimZeros(fmt.Sprintf("%.1f", ms/3_600_000)) + " h"
	}
}

// FormatDuration writes a duration the way the panel's "2 秒前" does.
func FormatDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return "刚刚"
	case d < time.Minute:
		return fmt.Sprintf("%d 秒前", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	default:
		return fmt.Sprintf("%d 天前", int(d.Hours()/24))
	}
}

// Snapshot is what a source reports after one poll: a status, the readings
// behind it, and where they came from.
type Snapshot struct {
	// SourceID names the source the snapshot belongs to.
	SourceID string
	// At is when it was taken.
	At time.Time
	// Status is the source's state as a whole.
	Status Status
	// Readings are the values, in the order the page shows them.
	Readings []Reading
	// Title is the name of what is being watched right now, as the
	// hardware reports it.
	Title string
	// Subtitle explains the title or the state: "2.4G · 90%".
	Subtitle string
	// Primary names the reading the page leads with, by ID. Empty means
	// the most severe reading leads.
	Primary string
	// Err is the last poll's failure, if it failed.
	Err string
}

// Reading finds a reading by ID.
func (s Snapshot) Reading(id string) (Reading, bool) {
	for _, r := range s.Readings {
		if r.ID == id {
			return r, true
		}
	}
	return Reading{}, false
}

// Lead returns the reading the page leads with: the one Primary names, or
// the most severe one.
func (s Snapshot) Lead() (Reading, bool) {
	if len(s.Readings) == 0 {
		return Reading{}, false
	}
	if s.Primary != "" {
		if r, ok := s.Reading(s.Primary); ok {
			return r, true
		}
	}
	best := s.Readings[0]
	for _, r := range s.Readings[1:] {
		if r.Severity.rank() < best.Severity.rank() {
			best = r
		}
	}
	return best, true
}

// rank orders severities, worst first.
func (s Severity) rank() int {
	switch s {
	case SeverityCritical:
		return 0
	case SeverityWarn:
		return 1
	case SeverityOffline:
		return 2
	case SeverityOK:
		return 3
	case SeverityInfo:
		return 4
	}
	return 5
}

// Worst returns the most severe status of the snapshot's readings.
func (s Snapshot) Worst() Status {
	worst := s.Status
	for _, r := range s.Readings {
		switch r.Severity {
		case SeverityCritical:
			if worst < StatusError {
				worst = StatusError
			}
		case SeverityWarn:
			if worst < StatusWarn {
				worst = StatusWarn
			}
		case SeverityOffline:
			if worst < StatusOffline {
				worst = StatusOffline
			}
		}
	}
	return worst
}
