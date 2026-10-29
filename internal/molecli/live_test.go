package molecli

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLiveCLI really calls the mo on this machine and only reads. It skips by
// default: it starts a few powershell processes and takes seconds, which has no
// business slowing down an everyday go test.
//
// To run it: SEEALL_MO_LIVE=1 go test ./internal/molecli -run TestLiveCLI -v
func TestLiveCLI(t *testing.T) {
	if os.Getenv("SEEALL_MO_LIVE") != "1" {
		t.Skip("live CLI smoke test: set SEEALL_MO_LIVE=1 to run it")
	}
	client := New()
	if !client.Available() {
		t.Fatalf("no mo on this machine (looked in PATH, %%LOCALAPPDATA%%\\Programs\\mole-cli and the scoop shims)")
	}
	t.Logf("mo is at %s", client.Path())

	// One read command gets up to two minutes: an analyze over the whole disk
	// takes seconds on a real machine, and the ceiling is only there to fail
	// early when something hangs.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	start := time.Now()
	out, err := client.run(ctx, "--version")
	if err != nil {
		t.Fatalf("mo --version failed: %v", err)
	}
	if !strings.Contains(stripANSI(string(out)), "Mole version") {
		t.Errorf("--version printed no version line: %q", tailText(string(out)))
	}
	t.Logf("mo --version took %v", time.Since(start).Round(time.Millisecond))

	start = time.Now()
	analyze, err := client.Analyze(ctx)
	if err != nil {
		t.Fatalf("mo analyze --json failed: %v", err)
	}
	t.Logf("mo analyze took %v, %d entries, %d bytes in total, partial=%v",
		time.Since(start).Round(time.Millisecond), len(analyze.Entries), analyze.Total, analyze.Partial)
	if len(analyze.Entries) == 0 {
		t.Error("analyze returned no entries")
	}
	if analyze.At.IsZero() {
		t.Error("analyze was not stamped")
	}

	start = time.Now()
	history, err := client.History(ctx, 3)
	if err != nil {
		t.Fatalf("mo history --json failed: %v", err)
	}
	t.Logf("mo history took %v, %d sessions and %d operations",
		time.Since(start).Round(time.Millisecond), len(history.Sessions), len(history.Operations))
}
