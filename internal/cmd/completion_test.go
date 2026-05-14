package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func executeCompletionCmd(args []string, buf *bytes.Buffer) error {
	root := &cobra.Command{Use: "ssmctl", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(completionCmd())
	root.SetArgs(args)
	root.SetOut(buf)
	return root.Execute() //nolint:wrapcheck
}

func TestCompletionCmd_Bash(t *testing.T) {
	var buf bytes.Buffer
	if err := executeCompletionCmd([]string{"completion", "bash"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ssmctl") {
		t.Errorf("bash completion output does not reference 'ssmctl': %s", out)
	}
	if !strings.Contains(out, "bash") {
		t.Errorf("bash completion output does not look like a bash script: %s", out)
	}
}

func TestCompletionCmd_Zsh(t *testing.T) {
	var buf bytes.Buffer
	if err := executeCompletionCmd([]string{"completion", "zsh"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ssmctl") {
		t.Errorf("zsh completion output does not reference 'ssmctl': %s", out)
	}
	if !strings.Contains(out, "zsh") {
		t.Errorf("zsh completion output does not look like a zsh script: %s", out)
	}
}

func TestCompletionCmd_Fish(t *testing.T) {
	var buf bytes.Buffer
	if err := executeCompletionCmd([]string{"completion", "fish"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ssmctl") {
		t.Errorf("fish completion output does not reference 'ssmctl': %s", out)
	}
}

func TestCompletionCmd_PowerShell(t *testing.T) {
	var buf bytes.Buffer
	if err := executeCompletionCmd([]string{"completion", "powershell"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ssmctl") {
		t.Errorf("powershell completion output does not reference 'ssmctl': %s", out)
	}
}

func TestCompletionCmd_InvalidShell(t *testing.T) {
	var buf bytes.Buffer
	err := executeCompletionCmd([]string{"completion", "nushell"}, &buf)
	if err == nil {
		t.Fatal("expected error for unsupported shell, got nil")
	}
}

func TestCompletionCmd_NoArgs(t *testing.T) {
	var buf bytes.Buffer
	err := executeCompletionCmd([]string{"completion"}, &buf)
	if err == nil {
		t.Fatal("expected error when no shell argument provided, got nil")
	}
}
