package panel

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestSettingsNotificationRowIsASwitch is the bug this row had: it was labelled
// 通知 but drew a 全部刷新 button, so the one thing the settings page appeared
// to offer about notifications was a second refresh button.
func TestSettingsNotificationRowIsASwitch(t *testing.T) {
	h, p := fixture(t)
	p.SetSelected(SettingsPage)
	p.SetSize(Fit(1920, 1040, 1))

	tt := ui.NewTester(p.View, DesignWidth, DesignHeight)
	if !tt.HasText("电量低、状态异常时弹系统通知") {
		t.Fatalf("the settings page has no notification row: %v", tt.Texts())
	}
	if tt.HasText("全部刷新") {
		t.Errorf("the notification row still draws a refresh button: %v", tt.Texts())
	}
	if !h.Notifications() {
		t.Fatal("a fresh panel does not notify")
	}

	before, ok := tt.Find("允许通知")
	if !ok {
		t.Fatalf("the row has no switch of its own: %v", tt.Texts())
	}

	if err := tt.Click("允许通知"); err != nil {
		t.Fatal(err)
	}
	if h.Notifications() {
		t.Error("clicking the 通知 switch left notifications on")
	}

	// The next frame draws the switch in its new position, and clicking it
	// again turns notifications back on. It is the same control, in the same
	// place: only its knob moved.
	tt = ui.NewTester(p.View, DesignWidth, DesignHeight)
	after, ok := tt.Find("允许通知")
	if !ok {
		t.Fatalf("the switch went away once it was off: %v", tt.Texts())
	}
	if after.W != before.W || after.H != before.H {
		t.Errorf("the switch changed size when off: %+v, was %+v", after, before)
	}
	if err := tt.Click("允许通知"); err != nil {
		t.Fatal(err)
	}
	if !h.Notifications() {
		t.Error("clicking the 通知 switch again did not turn notifications back on")
	}
}

// TestTheNotificationSwitchHidesEveryMonitorReminder is why the switch exists:
// a monitor's own reminder is a setting of its own and knows nothing about the
// master one, so turning notifications off has to reach it too.
func TestTheNotificationSwitchHidesEveryMonitorReminder(t *testing.T) {
	h, _ := fixture(t)

	var posted []string
	h.OnNotify = func(title, body string) { posted = append(posted, title) }

	// A low battery on the system monitor, with its own reminder on.
	in := h.Instance("system")
	if in == nil {
		t.Fatal("the system monitor is not running")
	}
	in.Settings().Set("notify", "true")

	h.SetNotifications(false)
	h.Notify("电量低", "这台电脑只剩 9%")
	if len(posted) != 0 {
		t.Errorf("posted %v with the master switch off", posted)
	}

	h.SetNotifications(true)
	h.Notify("电量低", "这台电脑只剩 9%")
	if len(posted) != 1 {
		t.Errorf("posted %v with the master switch on, want one", posted)
	}
}
