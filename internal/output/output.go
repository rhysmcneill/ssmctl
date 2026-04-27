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
type Printer struct {
	Format string
}

// Print writes the value to standard output using the configured format.
func (p *Printer) Print(v any) error {
	return p.Fprint(os.Stdout, v)
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
