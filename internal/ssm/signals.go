// TODO(refactor): this file and its platform siblings (signals_unix.go,
// signals_windows.go) are OS/process concerns that do not belong in the ssm
// package, whose stated purpose is AWS SSM API interaction. If
// runSessionManagerPlugin ever grows — reconnect logic, output multiplexing,
// plugin version detection — consider extracting it and these helpers into an
// internal/plugin package.

package ssm

import "os/signal"

// ignoreSessionSignalsFn is a package-level variable so tests can substitute
// the real signal-handling block with a recording fake. Production code must
// not reassign this outside of test files.
var ignoreSessionSignalsFn = ignoreSessionSignals

// ignoreSessionSignals installs SIG_IGN for the platform-specific
// sessionSignals so that terminal control signals (Ctrl-C, Ctrl-\, Ctrl-Z on
// POSIX; Ctrl-C and Ctrl-Break on Windows) are delivered only to the
// session-manager-plugin subprocess and not to ssmctl itself.
//
// It returns a restore function that resets the default handlers for those
// signals. Callers must invoke the returned function (typically via defer)
// before returning, otherwise the signals will remain ignored for the rest
// of the process lifetime.
//
// Without this guard ssmctl exits on Ctrl-C alongside the plugin, the TTY
// state is left inconsistent, and the plugin's stdin read fails with EIO —
// see GitHub issue #85.
func ignoreSessionSignals() (restore func()) {
	signal.Ignore(sessionSignals...)
	return func() {
		signal.Reset(sessionSignals...)
	}
}
