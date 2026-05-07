// Package output provides utilities for formatting and printing command output
// in different formats (text and JSON).
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Printer formats and writes output in the specified format (text or json).
// Out is the writer used by Print; if nil, os.Stdout is used.
type Printer struct {
	Format string
	Out    io.Writer
}

func (p *Printer) writer() io.Writer {
	if p.Out != nil {
		return p.Out
	}
	return os.Stdout
}

// Print writes the value to p.Out (or os.Stdout when nil) using the configured format.
func (p *Printer) Print(v any) error {
	return p.Fprint(p.writer(), v)
}

// Fprint writes the value to the given writer using the configured format.
// It supports "json" format with indentation and "text" format.
func (p *Printer) Fprint(w io.Writer, v any) error {
	switch p.Format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v); err != nil {
			return fmt.Errorf("encode JSON: %w", err)
		}
		return nil
	case "text":
		_, err := fmt.Fprintf(w, "%v\n", v)
		if err != nil {
			return fmt.Errorf("write text: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown output format: %s", p.Format)
	}
}
