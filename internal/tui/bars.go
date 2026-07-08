package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// bar renders a horizontal bar `width` cells wide filled to ratio (0..1) with
// block glyphs, styled with fill; the unfilled remainder uses the subtle style.
// Block glyphs degrade cleanly under NO_COLOR (the shape survives ANSI
// stripping), so the same code drives both the color and plain snapshots.
func (m Model) bar(ratio float64, width int, fill lipgloss.Style) string {
	if width < 1 {
		width = 1
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	full := int(ratio*float64(width) + 0.5)
	if full > width {
		full = width
	}
	return fill.Render(strings.Repeat("█", full)) +
		m.styles.Subtle.Render(strings.Repeat("░", width-full))
}

// planBar renders the data-plan usage bar. In color it uses the charmbracelet
// bubbles/progress component (gradient); plain/NO_COLOR falls back to bar() so
// the NO_COLOR contract holds and snapshots stay deterministic.
func (m Model) planBar(ratio float64, width int) string {
	if width < 1 {
		width = 1
	}
	if m.colored {
		p := m.progress
		p.Width = width
		return p.ViewAs(ratio)
	}
	style := m.styles.Online
	if ratio >= 0.9 {
		style = m.styles.ErrText
	} else if ratio >= 0.75 {
		style = m.styles.Status
	}
	return m.bar(ratio, width, style)
}
