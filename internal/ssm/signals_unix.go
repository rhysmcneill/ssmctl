//go:build !windows

package ssm

import (
	"os"
	"syscall"
)

// sessionSignals are the user-entered terminal control signals that ssmctl
// must ignore while session-manager-plugin is the active foreground process.
//
// Rationale (see issue #85):
//   - SIGINT  (Ctrl-C) — When the plugin is connected to an interactive shell
//     it puts the terminal into raw mode, so Ctrl-C is sent through stdin as
//     byte 0x03 and the remote PTY generates SIGINT for the remote process.
//     If ssmctl (the parent) also receives SIGINT and exits, the plugin loses
//     its controlling parent and stdin returns EIO, terminating the session
//     instead of the long-running remote command.
//   - SIGQUIT (Ctrl-\) — Same failure mode as SIGINT for an interactive
//     session; must be delivered to the remote PTY only.
//   - SIGTSTP (Ctrl-Z) — Suspending ssmctl while the plugin is attached to
//     our TTY would orphan the plugin in the foreground process group and
//     break stdin reads. We let the plugin manage suspension semantics.
//
// This mirrors aws-cli's `ignore_user_entered_signals()` in
// awscli/customizations/sessionmanager.py.
var sessionSignals = []os.Signal{
	syscall.SIGINT,
	syscall.SIGQUIT,
	syscall.SIGTSTP,
}
