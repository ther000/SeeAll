package awake

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestTheHoldReachesWindows checks a request actually sets the flags on the
// thread: Windows answers each call with the state that stood before it, so
// asking again after a hold shows what the hold left behind. A keeper that
// only remembers what it was told would pass every other test in this file and
// fail this one.
func TestTheHoldReachesWindows(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer apply(false)

	if err := apply(true); err != nil {
		t.Fatalf("the machine refused to stay awake: %v", err)
	}
	// Ask with no request at all: the answer is the state the hold left.
	prev, _, _ := procSetThreadExecutionState.Call(uintptr(continuous))
	if prev&(systemRequired|displayRequired) != systemRequired|displayRequired {
		t.Errorf("the thread holds 0x%x after a hold, want the display and the system required", prev)
	}
}

// TestSetHoldsAndReleases checks the keeper's own account of what it asked
// for, which is what the panel draws its button from.
func TestSetHoldsAndReleases(t *testing.T) {
	k := New()
	defer k.Close()

	if k.On() {
		t.Fatal("a new keeper holds the machine awake")
	}
	if err := k.Set(true); err != nil {
		t.Fatalf("holding the machine awake: %v", err)
	}
	if !k.On() {
		t.Error("the keeper does not report the hold it was asked for")
	}
	if err := k.Set(false); err != nil {
		t.Fatalf("letting the power plan have the machine back: %v", err)
	}
	if k.On() {
		t.Error("the keeper still holds the machine awake after being asked to let go")
	}
}

// TestCloseLetsGo checks quitting releases the hold and that a closed keeper
// refuses to pretend: the thread that held the request is gone, and a second
// hold would be a different thread's.
func TestCloseLetsGo(t *testing.T) {
	k := New()
	if err := k.Set(true); err != nil {
		t.Fatalf("holding the machine awake: %v", err)
	}
	k.Close()
	if k.On() {
		t.Error("the keeper still reports a hold after it was closed")
	}
	if err := k.Set(true); err != ErrClosed {
		t.Errorf("a closed keeper answered %v, want ErrClosed", err)
	}
	// Quitting twice is normal: the quit path and a deferred Close both run.
	k.Close()
}

// TestManyAskersEndWhereTheLastOneAsked checks the keeper is safe from the
// goroutines the app asks it from: a page drawing and a tray menu can both
// want the machine awake at the same moment, and neither may be lost in a
// half-applied state.
func TestManyAskersEndWhereTheLastOneAsked(t *testing.T) {
	k := New()
	defer k.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(on bool) {
			defer wg.Done()
			if err := k.Set(on); err != nil {
				errs <- err
			}
		}(i%2 == 0)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the keeper stopped answering: a request waits forever")
	}
	close(errs)
	for err := range errs {
		t.Errorf("a request failed: %v", err)
	}

	if err := k.Set(false); err != nil {
		t.Fatalf("letting go after the storm: %v", err)
	}
	if k.On() {
		t.Error("the last word was to let go, and the keeper still holds on")
	}
}
