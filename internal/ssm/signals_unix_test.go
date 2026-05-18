//go:build !windows

package ssm

import (
	"os"
	"os/signal"
	"slices"
	"syscall"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ignoreSessionSignals — signal-delivery tests (POSIX only)
// syscall.Kill and syscall.Getpid are not available on Windows.
// ---------------------------------------------------------------------------

// TestIgnoreSessionSignals_IgnoresSIGINT verifies that after the helper runs,
// a SIGINT sent to the test process is silently discarded rather than killing
// it. This is the precise scenario from issue #85: ssmctl must survive the
// Ctrl-C that the user types while the plugin is forwarding signals to the
// remote PTY.
//
// Mechanism: signal.Ignore installs SIG_IGN at the OS level, so the kernel
// drops the signal at delivery time. Sending SIGINT to ourselves under
// SIG_IGN is a true no-op, making this test safe to run in-process.
func TestIgnoreSessionSignals_IgnoresSIGINT(t *testing.T) {
	restore := ignoreSessionSignals()
	defer restore()

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("kill self with SIGINT: %v", err)
	}

	// Give the kernel a beat to deliver the signal. If the helper failed to
	// install SIG_IGN, the Go runtime's default SIGINT handler would have
	// terminated the test binary by now.
	time.Sleep(50 * time.Millisecond)
}

// TestIgnoreSessionSignals_RestoreAllowsDelivery verifies that after the
// returned restore function is called, the default handler is reinstated so
// subsequent signals are delivered normally.
//
// We assert this without letting SIGINT actually kill the test by registering
// a Notify channel AFTER restore — if restore re-enabled signal delivery,
// our channel will receive the signal.
func TestIgnoreSessionSignals_RestoreAllowsDelivery(t *testing.T) {
	restore := ignoreSessionSignals()
	restore()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT)
	t.Cleanup(func() { signal.Stop(ch) })

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("kill self with SIGINT: %v", err)
	}

	select {
	case got := <-ch:
		if got != syscall.SIGINT {
			t.Fatalf("received %v, want SIGINT", got)
		}
	case <-time.After(time.Second):
		t.Fatal("SIGINT was not delivered after restore — signal handler was not reset")
	}
}

// ---------------------------------------------------------------------------
// sessionSignals list — POSIX-specific membership checks
// ---------------------------------------------------------------------------

// TestSessionSignals_UnixIncludesQuitAndTstp pins the POSIX signal list so
// future edits cannot silently drop a signal that aws-cli's reference
// implementation handles.
func TestSessionSignals_UnixIncludesQuitAndTstp(t *testing.T) {
	want := []os.Signal{
		os.Signal(syscall.SIGINT),
		os.Signal(syscall.SIGQUIT),
		os.Signal(syscall.SIGTSTP),
	}
	for _, s := range want {
		if !slices.Contains(sessionSignals, s) {
			t.Errorf("sessionSignals missing %v on POSIX; got %v", s, sessionSignals)
		}
	}
}

// TestIgnoreSessionSignals_IgnoresSIGQUIT verifies the Ctrl-\ case. SIGQUIT's
// default action is "core dump and terminate", so without SIG_IGN this test
// would not finish.
func TestIgnoreSessionSignals_IgnoresSIGQUIT(t *testing.T) {
	restore := ignoreSessionSignals()
	defer restore()

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGQUIT); err != nil {
		t.Fatalf("kill self with SIGQUIT: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
}

// TestIgnoreSessionSignals_IgnoresSIGTSTP verifies the Ctrl-Z case. SIGTSTP's
// default action is "stop the process" — under SIG_IGN the test continues,
// without it the test runner would hang. This is the strongest in-process
// signal we can self-deliver to prove the helper is installed correctly.
func TestIgnoreSessionSignals_IgnoresSIGTSTP(t *testing.T) {
	restore := ignoreSessionSignals()
	defer restore()

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTSTP); err != nil {
		t.Fatalf("kill self with SIGTSTP: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
}
