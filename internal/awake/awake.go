// Package awake keeps the screen on and the machine out of sleep while the
// user asks for it.
//
// Windows keeps the request on the thread that made it rather than on the
// process: SetThreadExecutionState sets flags for the calling thread, and they
// stand until that thread calls it again or ends. So every call here comes
// from one goroutine pinned to an OS thread of its own, and the request
// outlives whoever asked for it — made on the thread the panel happened to
// draw on, the hold would be gone the first time the runtime reused it.
package awake

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
)

// The execution states this package asks for. CONTINUOUS is what makes them
// stand until the next call rather than for one activity, and it goes with
// either request: the power plan takes them back the moment it is asked to.
const (
	continuous      = 0x80000000
	systemRequired  = 0x00000001
	displayRequired = 0x00000002
)

var procSetThreadExecutionState = syscall.NewLazyDLL("kernel32.dll").NewProc("SetThreadExecutionState")

// ErrClosed is what Set reports on a keeper that has been closed: the thread
// that held the request is gone, and a second one would be a different hold.
var ErrClosed = errors.New("awake: keeper is closed")

// Keeper holds the screen on and the machine awake while it is on. Make one
// with New, and Close it before the app quits.
type Keeper struct {
	requests chan request
	quit     chan struct{}
	done     chan struct{}
	once     sync.Once

	mu sync.Mutex
	on bool
}

type request struct {
	on   bool
	done chan error
}

// New starts the thread that holds the request.
func New() *Keeper {
	k := &Keeper{
		requests: make(chan request),
		quit:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go k.hold()
	return k
}

// hold is the keeper's thread: it makes every call, so the flags Windows keeps
// for it are the whole story of what this app asked for.
func (k *Keeper) hold() {
	runtime.LockOSThread()
	defer close(k.done)
	for {
		select {
		case req := <-k.requests:
			err := apply(req.on)
			if err == nil {
				k.mu.Lock()
				k.on = req.on
				k.mu.Unlock()
			}
			req.done <- err
		case <-k.quit:
			// The thread is about to end, which releases the flags
			// anyway; letting go first is what a reader of the code
			// expects, and costs one call.
			apply(false)
			return
		}
	}
}

// apply sets or clears the flags on the calling thread. The call answers with
// the state that stood before it, which is zero both on failure and on a
// thread that held nothing, so the state it should have left is read back
// rather than trusted to the return value.
func apply(on bool) error {
	flags := uintptr(continuous)
	var want uintptr
	if on {
		flags |= systemRequired | displayRequired
		want = systemRequired | displayRequired
	}
	if _, _, err := procSetThreadExecutionState.Call(flags); err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno != 0 {
			return errno
		}
	}
	prev, _, err := procSetThreadExecutionState.Call(flags)
	if prev == 0 {
		if errno, ok := err.(syscall.Errno); ok && errno != 0 {
			return errno
		}
		return errors.New("awake: SetThreadExecutionState 没有生效")
	}
	if prev&(systemRequired|displayRequired) != want {
		return fmt.Errorf("awake: 系统没有接受常亮请求（0x%x）", prev)
	}
	return nil
}

// Set asks for the screen and the machine to stay awake (on), or lets the
// power plan have them back (off). It waits for Windows to answer, so that a
// machine which refused says so instead of a button that looks on all night.
func (k *Keeper) Set(on bool) error {
	req := request{on: on, done: make(chan error, 1)}
	select {
	case k.requests <- req:
	case <-k.done:
		return ErrClosed
	}
	select {
	case err := <-req.done:
		return err
	case <-k.done:
		return ErrClosed
	}
}

// On is whether the hold is on: the last request that Windows accepted.
func (k *Keeper) On() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.on
}

// Close releases the hold and stops the thread. It is safe to call more than
// once, and on a keeper that never held anything.
func (k *Keeper) Close() {
	k.once.Do(func() { close(k.quit) })
	<-k.done
	k.mu.Lock()
	k.on = false
	k.mu.Unlock()
}
