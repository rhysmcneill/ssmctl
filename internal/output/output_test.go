package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPrinter_Fprint_Text(t *testing.T) {
	p := &Printer{Format: "text"}
	var buf bytes.Buffer

	if err := p.Fprint(&buf, "hello world"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := buf.String(); got != "hello world\n" {
		t.Errorf("Fprint() = %q, want %q", got, "hello world\n")
	}
}

func TestPrinter_Fprint_JSON(t *testing.T) {
	p := &Printer{Format: "json"}
	var buf bytes.Buffer

	input := map[string]string{"key": "value"}
	if err := p.Fprint(&buf, input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got["key"] != "value" {
		t.Errorf("JSON output key = %q, want %q", got["key"], "value")
	}

	if !strings.Contains(buf.String(), "\n") {
		t.Error("JSON output missing trailing newline")
	}
}

func TestPrinter_Fprint_UnknownFormat(t *testing.T) {
	p := &Printer{Format: "xml"}
	var buf bytes.Buffer

	if err := p.Fprint(&buf, "anything"); err == nil {
		t.Fatal("expected error for unknown format, got nil")
	}
}

func TestPrinter_Print_UsesOut(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Format: "text", Out: &buf}

	if err := p.Print("via Out"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := buf.String(); got != "via Out\n" {
		t.Errorf("Print() = %q, want %q", got, "via Out\n")
	}
}
