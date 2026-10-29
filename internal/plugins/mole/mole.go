// Package mole runs the Mole CLI from the panel: what it would free, where the
// disk went, what it cleaned before, and the two commands that change the
// machine.
//
// It is the one plugin whose page does something rather than watch something,
// so it draws a page of its own (Manifest.Standalone) instead of a card. The
// rule the page keeps is Mole's own: preview first, then act. A cleanup needs
// a preview from this hour before it may run, and the panel asks for a second
// click on top of that.
//
// SeeAll never decides what goes away. The paths, the whitelist and the
// coverage are Mole's; this package only starts the command and reads the
// summary it prints.
package mole

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"seeall/internal/core"
	"seeall/internal/molecli"
	"seeall/internal/plugin"
)

// Action is one command the page can start.
type Action string

const (
	// ScanClean previews a cleanup: mo clean --dry-run. It reads the disk,
	// writes the path list and changes nothing.
	ScanClean Action = "scan-clean"
	// Clean removes what the preview showed: mo clean. It deletes for good.
	Clean Action = "clean"
	// ScanOptimize previews the maintenance tasks: mo optimize --dry-run.
	ScanOptimize Action = "scan-optimize"
	// Optimize runs them: mo optimize.
	Optimize Action = "optimize"
	// ScanDisk reads where the space went: mo analyze --json.
	ScanDisk Action = "scan-disk"
	// ScanHistory reads what Mole cleaned before: mo history --json.
	ScanHistory Action = "scan-history"
)

// Running is what the page says while the action runs. A cleanup of a full
// disk is a minute of work rather than a moment, so the page says what it is
// waiting for rather than only that it is waiting.
func (a Action) Running() string {
	switch a {
	case ScanClean:
		return "正在预览清理…"
	case Clean:
		return "正在清理…"
	case ScanOptimize:
		return "正在预览优化…"
	case Optimize:
		return "正在优化…"
	case ScanDisk:
		return "正在扫描磁盘…"
	case ScanHistory:
		return "正在读取历史…"
	}
	return "正在执行…"
}

// Removing says whether the action deletes files: those need a preview and a
// second click, and the page colours them as the dangerous ones.
func (a Action) Removing() bool { return a == Clean || a == Optimize }

// The two refusals a removing action can meet. They are errors rather than a
// disabled button because the button is drawn from a preview that may be a
// minute old by the time it is pressed.
var (
	// ErrBusy is a second command asked for while one runs: Mole reads the
	// disk and keeps one log, so two at once would answer each other.
	ErrBusy = errors.New("还有一条命令在跑，等它结束或先取消")
	// ErrNoPreview is a cleanup asked for before anything was previewed.
	ErrNoPreview = errors.New("先预览一次，看清要删的东西")
	// ErrUsedPreview is a preview a run has already been started with: it may
	// describe a disk that run has partly changed.
	ErrUsedPreview = errors.New("这次预览已经用过了，重新预览一次")
	// ErrStalePreview is a preview older than PreviewFresh, which may no
	// longer be what the disk looks like.
	ErrStalePreview = errors.New("上次预览已经过期，再预览一次")
	// ErrNotInstalled is the page on a machine without Mole.
	ErrNotInstalled = errors.New("没有找到 mo 命令")
)

// PreviewFresh is how long a preview stands in for the disk. A cleanup older
// than this asks for a new preview: the user agreed to a list, and the list is
// what the disk had at that moment.
const PreviewFresh = 30 * time.Minute

// The commands' own limits. Mole scans the whole user profile for a preview
// and reads the Windows services for an optimization, so the two removing
// actions get the room a person at a terminal would give them.
const (
	timeoutScan   = 10 * time.Minute
	timeoutRemove = 20 * time.Minute
)

// Data is what the page draws: the results of the commands it has run. It is
// a value rather than the instance itself so that the view reads one consistent
// picture, on the main thread, without a lock.
type Data struct {
	// Ready is whether Mole is installed. It is probed again on every
	// refresh, so installing Mole while the panel runs needs no restart.
	Ready bool

	// Clean is the last cleanup — the preview it will run, or the run that
	// replaced it. CleanRan says which, and CleanAt when. CleanSpent marks a
	// preview a run has already been started with: a failure or a cancel may
	// have deleted part of it, so it may not stand in for the disk again.
	Clean      *molecli.Clean
	CleanAt    time.Time
	CleanRan   bool
	CleanSpent bool

	// Optimize is the last maintenance, preview or run.
	Optimize *molecli.Optimize
	OptAt    time.Time
	OptRan   bool
	OptSpent bool

	// Disk is where the space went, as mo analyze reports it.
	Disk   *molecli.Analyze
	DiskAt time.Time

	// History is what Mole cleaned before this session.
	History *molecli.History
	HistAt  time.Time

	// Running is the command in flight, if any.
	Running *Job
	// Last is what the last finished command said.
	Last *Result
}

// Job is the command running now.
type Job struct {
	Action  Action
	Started time.Time
}

// Result is one line about the last finished command: what it freed, how many
// optimizations it applied, or why it failed.
type Result struct {
	Action Action
	At     time.Time
	Note   string
	// Err marks a failure, which the page draws in red.
	Err bool
}

// Plugin is the Mole panel plugin.
type Plugin struct{}

// Manifest describes the plugin: it watches nothing, has nothing to set, and
// lives on a page of its own.
func (Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:              "mole",
		Name:            "Mole",
		Summary:         "清理缓存、优化系统、看看磁盘被什么占了",
		Kind:            plugin.KindSystem,
		Icon:            "mole",
		DefaultInstance: "Mole",
		Standalone:      true,
	}
}

// New makes the instance. It only asks whether Mole is installed: a preview
// takes close to a minute, so nothing runs until the user asks for it.
func (Plugin) New(env *plugin.Env) plugin.Instance {
	cli := molecli.New()
	return &Instance{
		env:  env,
		cli:  cli,
		data: Data{Ready: cli.Available()},
	}
}

// Instance is the page's state and the commands it runs.
type Instance struct {
	env *plugin.Env
	cli *molecli.Client

	mu     sync.Mutex
	data   Data
	cancel context.CancelFunc
	wg     sync.WaitGroup
	closed bool
}

// Start publishes the page's first state. Mole runs nothing by itself.
func (i *Instance) Start() { i.publish() }

// Stop cancels the command in flight and waits for it to end, so that quitting
// the app does not leave a cleanup running behind it.
func (i *Instance) Stop() {
	i.mu.Lock()
	i.closed = true
	cancel := i.cancel
	i.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	i.wg.Wait()
}

// Poll only republishes: the page has nothing to sample, and a refresh should
// not start a command the user did not ask for.
//
// It is also where a machine that installed Mole after the panel started picks
// it up: the client caches what it found, so the cached answer is forgotten
// before asking again. Refresh (Ctrl+R) and reopening the panel both land here.
func (i *Instance) Poll() {
	i.mu.Lock()
	missing := !i.data.Ready
	i.mu.Unlock()
	if missing {
		i.cli.Forget()
		ready := i.cli.Available()
		i.mu.Lock()
		i.data.Ready = ready
		i.mu.Unlock()
	}
	i.publish()
}

// State returns what the page draws.
func (i *Instance) State() Data {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.data
}

// Seed fills the page as if the commands had run, for the preview tool and the
// layout tests, which must not touch the machine's disk.
func (i *Instance) Seed(d Data) {
	i.mu.Lock()
	i.data = d
	i.mu.Unlock()
}

// Run starts an action. A removing one needs a fresh preview first; the check
// lives here rather than in the button so that no path reaches mo clean
// without one.
func (i *Instance) Run(a Action) error {
	i.mu.Lock()
	if i.closed {
		i.mu.Unlock()
		return errors.New("面板正在退出")
	}
	if !i.data.Ready {
		i.mu.Unlock()
		return ErrNotInstalled
	}
	if i.data.Running != nil {
		i.mu.Unlock()
		return ErrBusy
	}
	switch a {
	case Clean:
		if err := i.checkPreview(); err != nil {
			i.mu.Unlock()
			return err
		}
	case Optimize:
		if err := i.checkOptimize(); err != nil {
			i.mu.Unlock()
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeoutOf(a))
	i.cancel = cancel
	i.data.Running = &Job{Action: a, Started: time.Now()}
	// A removing command spends the preview that allowed it: it rescans the
	// disk and deletes what it finds, so whether it succeeds, fails or is
	// cancelled, that preview no longer describes the machine.
	i.spend(a)
	i.wg.Add(1)
	i.mu.Unlock()

	i.publish()
	go i.run(ctx, cancel, a)
	return nil
}

// Cancel stops the command in flight, which is what a preview that turned out
// to be slow needs.
func (i *Instance) Cancel() {
	i.mu.Lock()
	cancel := i.cancel
	i.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// spend marks the preview a removing action was allowed by as used. It happens
// when the command starts rather than when it succeeds: a cancelled cleanup may
// already have deleted part of what the preview showed. The caller holds the
// lock.
func (i *Instance) spend(a Action) {
	switch a {
	case Clean:
		i.data.CleanSpent = true
	case Optimize:
		i.data.OptSpent = true
	}
}

// checkPreview says whether a cleanup may go ahead. The caller holds the lock.
func (i *Instance) checkPreview() error {
	if i.data.Clean == nil || i.data.CleanRan {
		return ErrNoPreview
	}
	if i.data.CleanSpent {
		return ErrUsedPreview
	}
	if age := time.Since(i.data.CleanAt); age > PreviewFresh {
		return ErrStalePreview
	}
	return nil
}

// checkOptimize is the same rule for the maintenance tasks. The caller holds
// the lock.
func (i *Instance) checkOptimize() error {
	if i.data.Optimize == nil || i.data.OptRan {
		return ErrNoPreview
	}
	if i.data.OptSpent {
		return ErrUsedPreview
	}
	if age := time.Since(i.data.OptAt); age > PreviewFresh {
		return ErrStalePreview
	}
	return nil
}

// run is one command from start to result. It is the only place the CLI is
// called, so every answer the page shows came through here.
func (i *Instance) run(ctx context.Context, cancel context.CancelFunc, a Action) {
	defer i.wg.Done()
	defer cancel()

	var (
		note string
		err  error
	)
	switch a {
	case ScanClean, Clean:
		var clean molecli.Clean
		if a == ScanClean {
			clean, err = i.cli.PreviewClean(ctx)
		} else {
			clean, err = i.cli.Clean(ctx)
		}
		if err == nil {
			i.mu.Lock()
			i.data.Clean, i.data.CleanAt = &clean, time.Now()
			i.data.CleanRan = a == Clean
			i.mu.Unlock()
			note = cleanNote(clean, a == Clean)
		}
	case ScanOptimize, Optimize:
		var opt molecli.Optimize
		if a == ScanOptimize {
			opt, err = i.cli.PreviewOptimize(ctx)
		} else {
			opt, err = i.cli.Optimize(ctx)
		}
		if err == nil {
			i.mu.Lock()
			i.data.Optimize, i.data.OptAt = &opt, time.Now()
			i.data.OptRan = a == Optimize
			i.mu.Unlock()
			note = optimizeNote(opt, a == Optimize)
		}
	case ScanDisk:
		var disk molecli.Analyze
		disk, err = i.cli.Analyze(ctx)
		if err == nil {
			i.mu.Lock()
			i.data.Disk, i.data.DiskAt = &disk, time.Now()
			i.mu.Unlock()
			note = fmt.Sprintf("%d 项 · 共 %s", len(disk.Entries),
				core.FormatBytes(float64(disk.Total)))
			if disk.Partial {
				note += "（部分目录没数完）"
			}
		}
	case ScanHistory:
		var history molecli.History
		history, err = i.cli.History(ctx, historyLimit)
		if err == nil {
			i.mu.Lock()
			i.data.History, i.data.HistAt = &history, time.Now()
			i.mu.Unlock()
			note = fmt.Sprintf("%d 次清理记录", len(history.Sessions))
		}
	}

	result := &Result{Action: a, At: time.Now(), Note: note}
	if err != nil {
		result.Err = true
		result.Note = errText(ctx, a, err)
		// The page has room for one line; the log gets the whole reason, so a
		// cleanup that failed at three in the morning can be looked up.
		i.env.Logf("Mole %s 失败：%v", a, err)
	}
	i.mu.Lock()
	i.data.Running = nil
	i.cancel = nil
	i.data.Last = result
	i.mu.Unlock()
	i.publish()
}

// historyLimit is how much of Mole's log the page shows. The card lists a few
// sessions; the rest would only push the page's other cards down.
const historyLimit = 20

// errText turns a failed command into the line the page has room for. A cancel
// is not a failure of Mole's, and says so — but a cancelled removal has to say
// more than that: mo clean deletes as it goes, so what it reached before the
// signal is gone.
func errText(ctx context.Context, a Action, err error) string {
	if errors.Is(ctx.Err(), context.Canceled) {
		if a.Removing() {
			return "已取消 · 前面已经删掉的可能要不回来，重新预览一次"
		}
		return "已取消"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		if a.Removing() {
			return "超时 · 可能只删了一部分，重新预览一次"
		}
		return "超时，Mole 没有在限定的时间里结束"
	}
	return err.Error()
}

// cleanNote says what a cleanup did, in the words the page has room for.
func cleanNote(c molecli.Clean, ran bool) string {
	if c.Bytes <= 0 {
		if ran {
			return "本来就很干净，没有释放空间"
		}
		return "没有可以清理的东西"
	}
	if ran {
		return fmt.Sprintf("已清理 %s · %d 项", c.Size, c.Items)
	}
	return fmt.Sprintf("可释放 %s · %d 项", c.Size, c.Items)
}

// optimizeNote says what the maintenance found or did.
func optimizeNote(o molecli.Optimize, ran bool) string {
	if ran {
		note := fmt.Sprintf("已应用 %d 项优化", o.Applied)
		if o.Issues > 0 {
			note += fmt.Sprintf(" · 发现 %d 个问题", o.Issues)
		}
		return note
	}
	return fmt.Sprintf("将优化 %d 项", o.Applied)
}

// timeoutOf is how long an action may take before it is taken as hung.
func timeoutOf(a Action) time.Duration {
	switch a {
	case ScanClean, ScanOptimize:
		return timeoutScan
	case Clean, Optimize:
		return timeoutRemove
	}
	return timeoutScan
}

// publish hands the page's state to the tray and the panel: a line for the
// tray's tooltip, and a repaint for the page. It is a snapshot because that is
// how a plugin reports, not because the dashboard draws one.
func (i *Instance) publish() {
	d := i.State()
	snap := core.Snapshot{At: time.Now(), Status: core.StatusOK, Title: "Mole"}
	switch {
	case !d.Ready:
		snap.Subtitle = "没有找到 mo 命令"
	case d.Running != nil:
		snap.Subtitle = d.Running.Action.Running()
	case d.Clean != nil && d.Clean.Bytes > 0:
		verb := "可释放"
		if d.CleanRan {
			verb = "已释放"
		}
		snap.Readings = []core.Reading{{
			ID: "cleanable", Label: verb, Value: float64(d.Clean.Bytes),
			Unit: core.UnitBytes, Detail: d.CleanAt.Format("15:04"),
		}}
	case d.History != nil && len(d.History.Sessions) > 0:
		snap.Subtitle = fmt.Sprintf("%d 次清理记录", len(d.History.Sessions))
	}
	i.env.Publish(snap)
}
