//go:build windows

package ssm

import (
	"os"
	"syscall"
)

// sigBreak is the Go signal value that corresponds to Windows CTRL_BREAK_EVENT.
// Go's runtime maps that console control event to syscall.Signal(0x15) — the
// same value exported by golang.org/x/sys/windows as SIGBREAK. We define it
// inline to avoid pulling in that module just for a single constant.
const sigBreak = syscall.Signal(0x15)

// sessionSignals are the user-entered console control signals that ssmctl
// must ignore while session-manager-plugin is the active foreground process.
//
// On Windows, console control events surface in Go as:
//   - CTRL_C_EVENT     → syscall.SIGINT  (Ctrl-C)
//   - CTRL_BREAK_EVENT → sigBreak        (Ctrl-Break)
//
// SIGQUIT and SIGTSTP are POSIX-only and intentionally omitted.
var sessionSignals = []os.Signal{
	syscall.SIGINT,
	sigBreak,
}
