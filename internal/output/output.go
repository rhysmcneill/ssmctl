package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Printer struct {
	Format string
}

func (p *Printer) Print(v any) error {
	return p.Fprint(os.Stdout, v)
}

func (p *Printer) Fprint(w io.Writer, v any) error {
	switch p.Format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case "text":
		_, err := fmt.Fprintf(w, "%v\n", v)
		return err
	default:
		return fmt.Errorf("unknown output format: %s", p.Format)
	}
}
