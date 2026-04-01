package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Format is the output format selected by the user.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// ParseFormat converts a string flag value to a Format, returning an error
// when the value is not recognised.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(s) {
	case "table", "":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "yaml":
		return FormatYAML, nil
	default:
		return "", fmt.Errorf("unknown output format %q (choose: table, json, yaml)", s)
	}
}

// Printer writes structured data to an io.Writer in the requested format.
type Printer struct {
	w      io.Writer
	format Format
}

// New returns a Printer targeting stdout.
func New(format Format) *Printer {
	return &Printer{w: os.Stdout, format: format}
}

// NewWithWriter returns a Printer targeting the given writer (useful in tests).
func NewWithWriter(w io.Writer, format Format) *Printer {
	return &Printer{w: w, format: format}
}

// Print serialises v according to the chosen format.
// v must be JSON-serialisable (a slice of structs works well).
func (p *Printer) Print(v any) error {
	switch p.format {
	case FormatJSON:
		return p.printJSON(v)
	case FormatYAML:
		return p.printYAML(v)
	default:
		// Table is handled per-command via PrintTable for flexibility.
		return p.printJSON(v)
	}
}

// PrintTable renders headers + rows as an aligned terminal table.
// Uses stdlib text/tabwriter — no external dependency.
func (p *Printer) PrintTable(headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(p.w, 0, 0, 2, ' ', 0)

	// Header row — uppercased for visual distinction
	upper := make([]string, len(headers))
	for i, h := range headers {
		upper[i] = strings.ToUpper(h)
	}
	fmt.Fprintln(tw, strings.Join(upper, "\t"))

	// Separator line
	seps := make([]string, len(headers))
	for i, h := range headers {
		seps[i] = strings.Repeat("─", len(h))
	}
	fmt.Fprintln(tw, strings.Join(seps, "\t"))

	// Data rows
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}

	tw.Flush()
}

func (p *Printer) printJSON(v any) error {
	enc := json.NewEncoder(p.w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (p *Printer) printYAML(v any) error {
	return yaml.NewEncoder(p.w).Encode(v)
}