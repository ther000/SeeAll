package molecli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// runCommand starts one process, captures its output, and waits for it to end
// or for ctx to be cancelled.
//
// It is not exec.CommandContext: that path kills only the direct child and
// leaves the status.exe under powershell alive, so a cancel has to kill the
// whole tree itself (see killTree). The package sets no deadline of its own;
// timeouts are whatever ctx the caller passes.
func runCommand(ctx context.Context, name string, argv []string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	cmd := exec.Command(name, argv...)
	cmd.SysProcAttr = sysProcAttr()

	// Output is captured as bytes: the JSON is UTF-8 and the clean progress
	// line mixes in ANSI, so both are stored first and read later.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("起不来 %s：%w", name, err)
	}
	done := make(chan error, 1)
	// finished closes when Wait returns, so the cancel branch can tell "it
	// ended on its own" from "it is still running" without reading
	// cmd.ProcessState, which Wait writes from the other goroutine.
	finished := make(chan struct{})
	go func() {
		err := cmd.Wait()
		close(finished)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			return stdout.Bytes(), nil
		}
		return nil, failed(err, outputOf(stdout, stderr))
	case <-ctx.Done():
		select {
		case <-finished:
			// It ended between the two cases: there is no tree left to
			// kill, and its PID may already belong to something else.
		default:
			killTree(cmd)
		}
		// Wait for Wait to come back, or the output buffers may still be
		// written to — but never forever: a taskkill that is refused and a
		// Kill that does nothing would otherwise hang the panel's quit.
		select {
		case <-done:
		case <-time.After(cancelGrace):
		}
		return nil, failed(ctx.Err(), outputOf(stdout, stderr))
	}
}

// cancelGrace is how long a cancelled command gets to go away after its tree
// was told to die.
const cancelGrace = 5 * time.Second

// outputOf joins the two streams. mo reports some failures on stdout and others
// on stderr, and both belong in the message.
func outputOf(stdout, stderr bytes.Buffer) string {
	if stderr.Len() == 0 {
		return stdout.String()
	}
	if stdout.Len() == 0 {
		return stderr.String()
	}
	return stdout.String() + "\n" + stderr.String()
}

// failed joins a cause and the tail of the output into an error that can be
// debugged.
//
// "exit status 1" on its own says nothing; what mo actually said is in its last
// few lines, so the ANSI-free tail, cut to a few hundred bytes, rides along.
func failed(cause error, output string) error {
	if tail := tailText(output); tail != "" {
		return fmt.Errorf("%w：%s", cause, tail)
	}
	return fmt.Errorf("%w", cause)
}
