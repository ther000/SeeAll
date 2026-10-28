package core

import (
	"fmt"
	"time"
)

// The dashboard needs two things a reading cannot say: the pills across its
// top, which describe the machine rather than measure it, and the list of
// programs behind the numbers. Both are part of the snapshot, so a plugin
// reports them the way it reports any other value and the panel does not know
// where they came from.

// Fact is one pill in the dashboard's header: a short label and the value it
// stands for.
type Fact struct {
	// Label names the fact ("已运行", "系统"). Empty when the value speaks
	// for itself, as a processor's name does.
	Label string
	// Value is the fact itself: "Ryzen 7 6800H", "2天2小时".
	Value string
}

// Text is how the dashboard writes a fact: the value alone, or the label and
// the value with one space between them.
func (f Fact) Text() string {
	if f.Label == "" {
		return f.Value
	}
	return f.Label + " " + f.Value
}

// Process is one program the machine runs.
type Process struct {
	// Name is what the program calls itself: "QQ", "Everything".
	Name string
	// Mem is its working set in bytes, which is what the dashboard sorts by.
	Mem float64
	// CPU is its share of the whole machine, in percent, as Task Manager
	// counts it: 100 means every core busy.
	CPU float64
}

// FormatSpan writes how long something has lasted the way the dashboard's
// uptime pill does: "2天2小时", "5小时12分", "18分钟". It is not
// FormatDuration: "已运行 2天前" would say when the machine started, not how
// long it has been up.
func FormatSpan(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%d天%d小时", days, hours)
	case hours > 0:
		return fmt.Sprintf("%d小时%d分", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%d分钟", minutes)
	default:
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	}
}
