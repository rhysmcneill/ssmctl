package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/version"
)

func executeVersionCmd(args []string, buf *bytes.Buffer) error {
	root := &cobra.Command{Use: "ssmctl", SilenceErrors: true, SilenceUsage: true}
	root.PersistentFlags().String("output", "text", "output format (text|json)")
	root.PersistentFlags().Duration("timeout", 30*time.Second, "timeout")
	root.AddCommand(versionCmd())
	root.SetArgs(args)
	root.SetOut(buf)
	return root.Execute() //nolint:wrapcheck
}

func TestVersionCmd_TextOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := executeVersionCmd([]string{"version"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, version.Version) {
		t.Errorf("output does not contain version %q: %s", version.Version, out)
	}
	if !strings.Contains(out, version.Commit) {
		t.Errorf("output does not contain commit %q: %s", version.Commit, out)
	}
	if !strings.Contains(out, version.BuildDate) {
		t.Errorf("output does not contain buildDate %q: %s", version.BuildDate, out)
	}
}

func TestVersionCmd_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := executeVersionCmd([]string{"--output", "json", "version"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got versionOutput
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if got.Version != version.Version {
		t.Errorf("version = %q, want %q", got.Version, version.Version)
	}
	if got.Commit != version.Commit {
		t.Errorf("commit = %q, want %q", got.Commit, version.Commit)
	}
	if got.BuildDate != version.BuildDate {
		t.Errorf("buildDate = %q, want %q", got.BuildDate, version.BuildDate)
	}
}
