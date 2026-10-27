package app

import "testing"

// TestNotifyDoesNotCallItself is the bug Load had: it pointed OnNotify at a
// function that called Notify again, so the first notification of a low
// battery recursed until the stack ran out. The probe above is what a plugin's
// Notify() call does.
func TestNotifyDoesNotCallItself(t *testing.T) {
	h := NewHost(t.TempDir())
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	defer h.Stop()

	// Load must leave the hook alone: the app sets it once the desktop is
	// there, and until then a notification goes nowhere rather than round.
	h.Notify("鼠标电量低", "只剩 15%")
}

// TestNotificationsCanBeTurnedOff checks the master switch: with it off a
// monitor's own reminder setting is not enough to put anything on the screen.
func TestNotificationsCanBeTurnedOff(t *testing.T) {
	dir := t.TempDir()
	h := NewHost(dir)
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()
	if !h.Notifications() {
		t.Fatal("a fresh install may not notify")
	}

	var posted []string
	h.OnNotify = func(title, body string) { posted = append(posted, title) }

	h.Notify("要弹", "正文")
	if len(posted) != 1 {
		t.Fatalf("posted %v, want the notification through", posted)
	}

	h.SetNotifications(false)
	h.Notify("不该弹", "正文")
	if len(posted) != 1 {
		t.Errorf("posted %v with notifications off, want nothing more", posted)
	}

	// The choice is kept.
	again := NewHost(dir)
	if err := again.Load(); err != nil {
		t.Fatal(err)
	}
	again.Stop()
	if again.Notifications() {
		t.Error("the switch came back on after a restart")
	}
}
