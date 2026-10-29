package mole

import (
	"context"
	"errors"
	"testing"
	"time"

	"seeall/internal/core"
	"seeall/internal/molecli"
	"seeall/internal/plugin"
)

// fresh is the state a page has after a preview from a minute ago.
func fresh() Data {
	return Data{
		Ready:    true,
		Clean:    &molecli.Clean{Size: "128 MB", Bytes: 128 << 20, Items: 3, DryRun: true},
		CleanAt:  time.Now().Add(-time.Minute),
		Optimize: &molecli.Optimize{Applied: 12, DryRun: true},
		OptAt:    time.Now().Add(-time.Minute),
	}
}

// TestRunRefusesARemovalWithoutAPreview is the page's safety rule in one test:
// no path reaches mo clean or mo optimize unless a preview of that same thing
// was taken this hour. The refusals happen before the CLI is started, so this
// test touches no files.
func TestRunRefusesARemovalWithoutAPreview(t *testing.T) {
	for _, c := range []struct {
		name string
		data Data
		act  Action
		want error
	}{
		{"clean without a preview", Data{Ready: true}, Clean, ErrNoPreview},
		{"clean after it already ran", func() Data {
			d := fresh()
			d.CleanRan = true
			return d
		}(), Clean, ErrNoPreview},
		{"clean with an expired preview", func() Data {
			d := fresh()
			d.CleanAt = time.Now().Add(-2 * PreviewFresh)
			return d
		}(), Clean, ErrStalePreview},
		{"clean with a preview a run already used", func() Data {
			d := fresh()
			d.CleanSpent = true
			return d
		}(), Clean, ErrUsedPreview},
		{"optimize without a preview", Data{Ready: true}, Optimize, ErrNoPreview},
		{"optimize after it already ran", func() Data {
			d := fresh()
			d.OptRan = true
			return d
		}(), Optimize, ErrNoPreview},
		{"optimize with an expired preview", func() Data {
			d := fresh()
			d.OptAt = time.Now().Add(-2 * PreviewFresh)
			return d
		}(), Optimize, ErrStalePreview},
		{"optimize with a preview a run already used", func() Data {
			d := fresh()
			d.OptSpent = true
			return d
		}(), Optimize, ErrUsedPreview},
	} {
		i := &Instance{data: c.data}
		if err := i.Run(c.act); !errors.Is(err, c.want) {
			t.Errorf("%s: Run(%s) = %v, want %v", c.name, c.act, err, c.want)
		}
		if i.data.Running != nil {
			t.Errorf("%s: Run(%s) started something it should have refused", c.name, c.act)
		}
	}
}

// TestARemovalSpendsItsPreview is the difference between "the preview was
// shown" and "the preview is still true": a cleanup that failed or was
// cancelled may have deleted half of what it showed, so it may not stand in for
// the disk a second time.
func TestARemovalSpendsItsPreview(t *testing.T) {
	i := &Instance{data: fresh()}
	i.spend(Clean)
	if !i.State().CleanSpent {
		t.Fatal("starting a cleanup left its preview standing")
	}
	if err := i.Run(Clean); !errors.Is(err, ErrUsedPreview) {
		t.Errorf("a second cleanup on a spent preview = %v, want %v", err, ErrUsedPreview)
	}

	o := &Instance{data: fresh()}
	o.spend(Optimize)
	if err := o.Run(Optimize); !errors.Is(err, ErrUsedPreview) {
		t.Errorf("a second optimization on a spent preview = %v, want %v", err, ErrUsedPreview)
	}

	// A reading command spends nothing: it changed nothing.
	r := &Instance{data: fresh()}
	r.spend(ScanDisk)
	if r.data.CleanSpent || r.data.OptSpent {
		t.Errorf("scanning the disk spent a preview: %+v", r.data)
	}
}

// TestRunRefusesASecondCommand checks one command at a time: Mole keeps one log
// and answers about one disk, so two at once would answer each other.
func TestRunRefusesASecondCommand(t *testing.T) {
	i := &Instance{data: Data{Ready: true, Running: &Job{Action: ScanClean, Started: time.Now()}}}
	if err := i.Run(ScanHistory); !errors.Is(err, ErrBusy) {
		t.Errorf("Run while another command runs = %v, want %v", err, ErrBusy)
	}
	// A removal is refused for the same reason, and not with a preview
	// complaint: the answer has to be about what is actually in the way.
	i.data = fresh()
	i.data.Running = &Job{Action: ScanDisk, Started: time.Now()}
	if err := i.Run(Clean); !errors.Is(err, ErrBusy) {
		t.Errorf("Run(Clean) while another command runs = %v, want %v", err, ErrBusy)
	}
}

// TestRunRefusesWithoutMole checks the page on a machine that has no CLI: it
// says so rather than starting a command that cannot run.
func TestRunRefusesWithoutMole(t *testing.T) {
	i := &Instance{data: Data{Ready: false}}
	if err := i.Run(ScanHistory); !errors.Is(err, ErrNotInstalled) {
		t.Errorf("Run without Mole = %v, want %v", err, ErrNotInstalled)
	}
}

// TestErrText says what a failed command reads like: a cancel is the user's
// doing and must not read as Mole failing, and a timeout explains itself.
func TestErrText(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()

	for _, c := range []struct {
		name string
		ctx  context.Context
		act  Action
		err  error
		want string
	}{
		{"cancelled", cancelled, ScanClean, errors.New("exit status 1"), "已取消"},
		{"cancelled removal", cancelled, Clean, errors.New("exit status 1"),
			"已取消 · 前面已经删掉的可能要不回来，重新预览一次"},
		{"timed out", expired, ScanDisk, context.DeadlineExceeded, "超时，Mole 没有在限定的时间里结束"},
		{"timed out removal", expired, Optimize, context.DeadlineExceeded,
			"超时 · 可能只删了一部分，重新预览一次"},
		{"failed", context.Background(), ScanHistory, errors.New("mo clean 退出码 1：磁盘只读"),
			"mo clean 退出码 1：磁盘只读"},
	} {
		if got := errText(c.ctx, c.act, c.err); got != c.want {
			t.Errorf("%s: errText = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestPublishIsTheTrayLine checks the one place another part of the app reads
// this plugin: the tray's tooltip says what the page found, in the words a
// single line has room for, and says what is happening instead while a command
// runs.
func TestPublishIsTheTrayLine(t *testing.T) {
	published := func(t *testing.T, d Data) core.Snapshot {
		t.Helper()
		var got core.Snapshot
		env := plugin.NewEnv("mole", Plugin{}.Manifest(), plugin.NewSettings(nil))
		env.SetPublishHook(func(_ string, snap core.Snapshot) { got = snap })
		i := &Instance{env: env, data: d}
		i.publish()
		return got
	}

	// A preview that found space reads as what it would free.
	d := fresh()
	snap := published(t, d)
	if len(snap.Readings) != 1 {
		t.Fatalf("a preview publishes %d readings, want one", len(snap.Readings))
	}
	if r := snap.Readings[0]; r.Label != "可释放" || r.Format() != "128 MB" {
		t.Errorf("the tray line reads %q %q, want 可释放 128 MB", r.Label, r.Format())
	}

	// After the cleanup it says what it freed, not what it would.
	d.CleanRan = true
	if r := published(t, d).Readings[0]; r.Label != "已释放" {
		t.Errorf("after a cleanup the tray line reads %q, want 已释放", r.Label)
	}

	// While a command runs the line is about the command.
	d.Running = &Job{Action: Clean, Started: time.Now()}
	if got := published(t, d).Subtitle; got != "正在清理…" {
		t.Errorf("while cleaning the tray line reads %q, want 正在清理…", got)
	}

	// A machine without Mole says so rather than showing nothing.
	if got := published(t, Data{Ready: false}).Subtitle; got != "没有找到 mo 命令" {
		t.Errorf("without Mole the tray line reads %q, want 没有找到 mo 命令", got)
	}
}

// TestTimeoutGivesTheScanAndTheRemovalTheirOwnRoom checks the two kinds of
// command are bounded as differently as they run: a preview reads the whole
// user profile, a removal does that and then deletes what it found.
func TestTimeoutGivesTheScanAndTheRemovalTheirOwnRoom(t *testing.T) {
	if got, want := timeoutOf(ScanClean), timeoutScan; got != want {
		t.Errorf("timeoutOf(ScanClean) = %v, want %v", got, want)
	}
	if got, want := timeoutOf(Clean), timeoutRemove; got != want {
		t.Errorf("timeoutOf(Clean) = %v, want %v", got, want)
	}
	if timeoutRemove <= timeoutScan {
		t.Errorf("a removal is allowed %v, no longer than the %v preview it deletes",
			timeoutRemove, timeoutScan)
	}
}
