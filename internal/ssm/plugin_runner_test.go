//go:build !windows

package ssm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// installFakePlugin writes a POSIX shell script masquerading as
// session-manager-plugin into a temp directory, prepends that directory to
// PATH for the duration of the test, and returns the directory path.
//
// `body` is the shell body the fake plugin executes. Use "exit 0" for a
// successful no-op, "exit N" to simulate failure, or "sleep 0.2; exit 0" to
// keep the subprocess running long enough to send signals from the test.
func installFakePlugin(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session-manager-plugin")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil { // #nosec G306 -- fake plugin must be executable for the test
		t.Fatalf("write fake plugin: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

// ---------------------------------------------------------------------------
// runSessionManagerPlugin — happy path
// ---------------------------------------------------------------------------

func TestRunSessionManagerPlugin_SuccessfulExitReturnsNil(t *testing.T) {
	installFakePlugin(t, "exit 0")

	err := runSessionManagerPlugin(context.Background(), "{}", "us-east-1", "", "{}")
	if err != nil {
		t.Fatalf("expected nil from clean plugin exit, got %v", err)
	}
}

func TestRunSessionManagerPlugin_NonZeroExitIsWrapped(t *testing.T) {
	installFakePlugin(t, "exit 7")

	err := runSessionManagerPlugin(context.Background(), "{}", "us-east-1", "", "{}")
	if err == nil {
		t.Fatal("expected error from non-zero plugin exit, got nil")
	}
	if !strings.Contains(err.Error(), "session-manager-plugin exited with error") {
		t.Errorf("error message lost wrap context: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runSessionManagerPlugin — error paths must not touch signal handlers
// ---------------------------------------------------------------------------

// TestRunSessionManagerPlugin_LookPathFailureSkipsSignalBlock verifies that
// when session-manager-plugin is missing from PATH, we return early WITHOUT
// installing the signal-ignore handlers. Installing them and then returning
// could leave the parent process unable to receive Ctrl-C if the restore
// defer is somehow skipped.
func TestRunSessionManagerPlugin_LookPathFailureSkipsSignalBlock(t *testing.T) {
	t.Setenv("PATH", "/nonexistent-directory-for-test")

	var ignoreCalled bool
	prev := ignoreSessionSignalsFn
	ignoreSessionSignalsFn = func() func() {
		ignoreCalled = true
		return func() {}
	}
	t.Cleanup(func() { ignoreSessionSignalsFn = prev })

	err := runSessionManagerPlugin(context.Background(), "{}", "us-east-1", "", "{}")
	if err == nil {
		t.Fatal("expected error when plugin is not on PATH")
	}
	if !strings.Contains(err.Error(), "session-manager-plugin not found") {
		t.Errorf("error did not describe the missing plugin: %v", err)
	}
	if ignoreCalled {
		t.Error("signal handlers should not be touched when plugin lookup fails")
	}
}

// ---------------------------------------------------------------------------
// runSessionManagerPlugin — signal-handling integration
// ---------------------------------------------------------------------------

// TestRunSessionManagerPlugin_InstallsAndRestoresSignalHook proves the
// invariant that every call to runSessionManagerPlugin installs the signal
// hook exactly once and restores it exactly once, regardless of plugin
// outcome.
func TestRunSessionManagerPlugin_InstallsAndRestoresSignalHook(t *testing.T) {
	installFakePlugin(t, "exit 0")

	var ignoreCalls, restoreCalls int
	prev := ignoreSessionSignalsFn
	ignoreSessionSignalsFn = func() func() {
		ignoreCalls++
		return func() { restoreCalls++ }
	}
	t.Cleanup(func() { ignoreSessionSignalsFn = prev })

	if err := runSessionManagerPlugin(context.Background(), "{}", "us-east-1", "", "{}"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ignoreCalls != 1 {
		t.Errorf("ignoreSessionSignalsFn called %d times, want 1", ignoreCalls)
	}
	if restoreCalls != 1 {
		t.Errorf("restore func called %d times, want 1", restoreCalls)
	}
}

// TestRunSessionManagerPlugin_RestoresOnPluginFailure ensures the restore
// defer fires even when the plugin exits non-zero. Without this guarantee a
// failed connect attempt would leave the rest of the ssmctl process unable
// to react to Ctrl-C.
func TestRunSessionManagerPlugin_RestoresOnPluginFailure(t *testing.T) {
	installFakePlugin(t, "exit 9")

	var restoreCalls int
	prev := ignoreSessionSignalsFn
	ignoreSessionSignalsFn = func() func() {
		return func() { restoreCalls++ }
	}
	t.Cleanup(func() { ignoreSessionSignalsFn = prev })

	if err := runSessionManagerPlugin(context.Background(), "{}", "us-east-1", "", "{}"); err == nil {
		t.Fatal("expected error from failing fake plugin")
	}
	if restoreCalls != 1 {
		t.Errorf("restore func called %d times, want 1 even on plugin failure", restoreCalls)
	}
}

// TestRunSessionManagerPlugin_SIGINTDuringRunDoesNotKillProcess is the
// end-to-end regression test for issue #85. We start the real
// runSessionManagerPlugin against a fake plugin that sleeps long enough for
// us to send SIGINT to ourselves. If the fix is broken, the Go runtime's
// default SIGINT handler will terminate this test binary and CI will report
// the test as having failed catastrophically (no PASS line).
//
// Notes for future maintainers:
//   - We use the REAL ignoreSessionSignalsFn here (no mock) because the
//     point is to verify that SIG_IGN is genuinely installed at the OS
//     level.
//   - We send SIGINT via syscall.Kill(getpid, …) rather than via a process
//     group because the test runner is not the process-group leader of a
//     controlling terminal.
//   - 50ms is a comfortable margin for the goroutine to reach the signal
//     block before we send the signal; the fake plugin sleeps for 200ms
//     so cmd.Run is still in flight when we deliver SIGINT.
func TestRunSessionManagerPlugin_SIGINTDuringRunDoesNotKillProcess(t *testing.T) {
	installFakePlugin(t, "sleep 0.2; exit 0")

	done := make(chan error, 1)
	go func() {
		done <- runSessionManagerPlugin(context.Background(), "{}", "us-east-1", "", "{}")
	}()

	time.Sleep(50 * time.Millisecond)

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("kill self with SIGINT: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runSessionManagerPlugin returned error after SIGINT: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runSessionManagerPlugin did not return after fake plugin should have exited")
	}
}
