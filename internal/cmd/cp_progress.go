package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

func progressReporter(cmd *cobra.Command, outputFormat, action string) ssmlib.ProgressFunc {
	if outputFormat == "json" || !isTerminal(cmd.OutOrStdout()) {
		return nil
	}

	return newProgressWriter(cmd.ErrOrStderr(), action)
}

func newProgressWriter(w io.Writer, action string) ssmlib.ProgressFunc {
	var lastDone, lastTotal int64 = -1, -2
	var finished bool

	return func(done, total int64) {
		if finished || done == lastDone && total == lastTotal {
			return
		}
		lastDone, lastTotal = done, total

		if total >= 0 {
			percent := int64(100)
			if total > 0 {
				percent = done * 100 / total
			}
			if percent > 100 {
				percent = 100
			}
			_, _ = fmt.Fprintf(w, "\r%s ... %s / %s (%d%%)", action, formatBytes(done), formatBytes(total), percent)
			if done >= total {
				_, _ = fmt.Fprintln(w)
				finished = true
			}
			return
		}

		_, _ = fmt.Fprintf(w, "\r%s ... %s", action, formatBytes(done))
	}
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	stat, err := file.Stat()
	return err == nil && stat.Mode()&os.ModeCharDevice != 0
}

func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}

	value := float64(n)
	for _, unit := range []string{"KB", "MB", "GB", "TB"} {
		value /= 1024
		if value < 1024 {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%.1f PB", value/1024)
}
