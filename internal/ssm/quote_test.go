package ssm

import "testing"

func TestShellQuote_ExportedWrapper(t *testing.T) {
	got := ShellQuote("it's /tmp/file.txt")
	want := `'it'\''s /tmp/file.txt'`
	if got != want {
		t.Fatalf("ShellQuote() = %q, want %q", got, want)
	}
}

func TestPowerShellQuote_ExportedWrapper(t *testing.T) {
	got := PowerShellQuote("it's C:\\Temp\\file.txt")
	want := `'it''s C:\Temp\file.txt'`
	if got != want {
		t.Fatalf("PowerShellQuote() = %q, want %q", got, want)
	}
}
