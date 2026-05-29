// Package render turns rows of strings into output: a clean, ANSI-free table
// for pipes and machine consumers, or a lipgloss-styled table for terminals.
// JSON output is always plain. Color is gated behind isatty/NO_COLOR so piped
// output never contains escape codes.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
	ltable "github.com/charmbracelet/lipgloss/table"
	"github.com/mattn/go-isatty"
)

// JSON writes v as indented JSON followed by a newline.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Table writes headers and rows. When color is false it emits a plain,
// tab-aligned, ANSI-free table (safe for pipes and golden tests); when true it
// emits a lipgloss-styled table for terminals.
func Table(w io.Writer, headers []string, rows [][]string, color bool) error {
	if color {
		return styledTable(w, headers, rows)
	}
	return plainTable(w, headers, rows)
}

func plainTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(upper(headers), "\t"))
	for _, r := range rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	return tw.Flush()
}

func styledTable(w io.Writer, headers []string, rows [][]string) error {
	t := ltable.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		Headers(upper(headers)...).
		Rows(rows...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == ltable.HeaderRow {
				return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")).Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})
	_, err := fmt.Fprintln(w, t.Render())
	return err
}

// ColorEnabled reports whether styled/colored output should be used: only when
// not explicitly disabled, NO_COLOR is unset, and w is a real terminal.
func ColorEnabled(w io.Writer, noColor bool) bool {
	if noColor || os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

func upper(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToUpper(s)
	}
	return out
}
