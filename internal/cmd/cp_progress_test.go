package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestProgressWriterFormatsByteCounts(t *testing.T) {
	var buf bytes.Buffer
	progress := newProgressWriter(&buf, "Uploading")

	progress(0, 2048)
	progress(1024, 2048)
	progress(2048, 2048)

	out := buf.String()
	for _, want := range []string{
		"Uploading ... 0 B / 2.0 KB (0%)",
		"Uploading ... 1.0 KB / 2.0 KB (50%)",
		"Uploading ... 2.0 KB / 2.0 KB (100%)\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("progress output missing %q:\n%s", want, out)
		}
	}
}

func TestProgressReporterSuppressesJSONAndNonTTYOutput(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})

	if got := progressReporter(cmd, "json", "Uploading"); got != nil {
		t.Fatal("progressReporter returned a reporter for JSON output")
	}
	if got := progressReporter(cmd, "text", "Uploading"); got != nil {
		t.Fatal("progressReporter returned a reporter for non-TTY stdout")
	}
}
