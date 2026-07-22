package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestProgressWriterFormatsByteCounts(t *testing.T) {
	var buf bytes.Buffer
	progress := newProgressWriter(&buf, "Uploading")

	progress(0, 2048)
	progress(1024, 2048)
	progress(4096, 2048)
	progress(2048, 2048)
	progress(2048, 2048)

	out := buf.String()
	for _, want := range []string{
		"Uploading ... 0 B / 2.0 KB (0%)",
		"Uploading ... 1.0 KB / 2.0 KB (50%)",
		"Uploading ... 4.0 KB / 2.0 KB (100%)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("progress output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "2.0 KB / 2.0 KB") {
		t.Fatalf("progress output changed after completion:\n%s", out)
	}
}

func TestProgressWriterFormatsZeroByteTransfer(t *testing.T) {
	var buf bytes.Buffer
	progress := newProgressWriter(&buf, "Uploading")

	progress(0, 0)

	if got, want := buf.String(), "\rUploading ... 0 B / 0 B (0%)\n"; got != want {
		t.Fatalf("progress output = %q, want %q", got, want)
	}
}

func TestProgressWriterFormatsUnknownAndLargeByteCounts(t *testing.T) {
	var buf bytes.Buffer
	progress := newProgressWriter(&buf, "Downloading")

	progress(1536, -1)
	progress(1<<50, 1<<50)

	out := buf.String()
	for _, want := range []string{
		"Downloading ... 1.5 KB",
		"Downloading ... 1.0 PB / 1.0 PB (100%)\n",
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

func TestProgressReporterWritesToStderrForTTYTextOutput(t *testing.T) {
	tty, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("open /dev/null: %v", err)
	}
	defer func() { _ = tty.Close() }()

	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(tty)
	cmd.SetErr(&stderr)

	progress := progressReporter(cmd, "text", "Uploading")
	if progress == nil {
		t.Fatal("progressReporter returned nil for TTY text output")
	}
	progress(1, 1)

	if !strings.Contains(stderr.String(), "Uploading ... 1 B / 1 B (100%)") {
		t.Fatalf("stderr progress output = %q", stderr.String())
	}
}

func TestIsCharDevice(t *testing.T) {
	if isCharDevice(&bytes.Buffer{}) {
		t.Fatal("bytes.Buffer reported as character device")
	}

	tty, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("open /dev/null: %v", err)
	}
	defer func() { _ = tty.Close() }()
	if !isCharDevice(tty) {
		t.Fatal("/dev/null did not report as a character device")
	}

	closed, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if isCharDevice(closed) {
		t.Fatal("closed file reported as character device")
	}
}
