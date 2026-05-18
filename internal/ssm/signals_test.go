package ssm

import (
	"os"
	"slices"
	"syscall"
	"testing"
)

// ---------------------------------------------------------------------------
// sessionSignals list — cross-platform
// ---------------------------------------------------------------------------

// TestSessionSignals_AlwaysIncludesSIGINT guards against regressions where a
// future refactor accidentally drops SIGINT from the list. SIGINT is the
// signal that the original bug (#85) was filed against; the others are
// defense in depth.
func TestSessionSignals_AlwaysIncludesSIGINT(t *testing.T) {
	if !slices.Contains(sessionSignals, os.Signal(syscall.SIGINT)) {
		t.Fatalf("sessionSignals must include SIGINT, got %v", sessionSignals)
	}
}

func TestSessionSignals_NonEmpty(t *testing.T) {
	if len(sessionSignals) == 0 {
		t.Fatal("sessionSignals must contain at least one signal")
	}
}

// ---------------------------------------------------------------------------
// ignoreSessionSignals — cross-platform structural tests
// ---------------------------------------------------------------------------

// TestIgnoreSessionSignals_RestoreIsIdempotent ensures the restore closure is
// safe to invoke more than once. Defers in some call paths can fire twice in
// rare error-recovery scenarios, and signal.Reset is documented as a no-op
// for unset signals.
func TestIgnoreSessionSignals_RestoreIsIdempotent(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("calling restore twice panicked: %v", r)
		}
	}()

	restore := ignoreSessionSignals()
	restore()
	restore()
}

// TestIgnoreSessionSignals_HelperReturnsFunction is a tiny structural test
// that catches accidental nil returns from the helper, which would crash
// runSessionManagerPlugin at `defer restore()` time.
func TestIgnoreSessionSignals_HelperReturnsFunction(t *testing.T) {
	restore := ignoreSessionSignals()
	if restore == nil {
		t.Fatal("ignoreSessionSignals returned nil restore func")
	}
	restore()
}
