package tui

import "github.com/charmbracelet/lipgloss"

// Styles holds the lipgloss styles for the dashboard. Kept in one struct so a
// future theme/no-color variant can swap the whole set.
type Styles struct {
	Title       lipgloss.Style // accent (cursor, section headers)
	TitleBadge  lipgloss.Style // floating "🔥 fire" badge
	Subtle      lipgloss.Style
	Selected    lipgloss.Style
	Online      lipgloss.Style
	Offline     lipgloss.Style
	Header      lipgloss.Style
	Footer      lipgloss.Style
	Status      lipgloss.Style
	ErrText     lipgloss.Style
	Modal       lipgloss.Style
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	ModePill    lipgloss.Style
}

// Tokyo Night palette, matching stefanpenner/otel-explorer so the two tools
// share a look.
const (
	tnBlue    = lipgloss.Color("#7aa2f7")
	tnGreen   = lipgloss.Color("#9ece6a")
	tnRed     = lipgloss.Color("#f7768e")
	tnYellow  = lipgloss.Color("#e0af68")
	tnGray    = lipgloss.Color("#565f89")
	tnWhite   = lipgloss.Color("#c0caf5")
	tnPurple  = lipgloss.Color("#bb9af7")
	tnSelBg   = lipgloss.Color("#283457")
	tnSurface = lipgloss.Color("#1a1b26")
)

// PlainStyles returns a no-color style set for NO_COLOR / non-color terminals.
// Hierarchy survives via bold and reverse-video (the selected row) rather than
// hue, so the dashboard stays legible without ANSI colors.
func PlainStyles() Styles {
	bold := lipgloss.NewStyle().Bold(true)
	plain := lipgloss.NewStyle()
	reverse := lipgloss.NewStyle().Reverse(true).Bold(true)
	return Styles{
		Title:       bold,
		TitleBadge:  bold,
		Subtle:      plain,
		Selected:    reverse,
		Online:      bold,
		Offline:     plain,
		Header:      bold,
		Footer:      plain,
		Status:      bold,
		ErrText:     bold,
		Modal:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1),
		TabActive:   reverse,
		TabInactive: plain,
		ModePill:    bold,
	}
}

// DefaultStyles returns the standard Tokyo Night color styles.
func DefaultStyles() Styles {
	return Styles{
		Title:       lipgloss.NewStyle().Bold(true).Foreground(tnBlue),
		TitleBadge:  lipgloss.NewStyle().Bold(true).Foreground(tnSurface).Background(tnBlue),
		Subtle:      lipgloss.NewStyle().Foreground(tnGray),
		Selected:    lipgloss.NewStyle().Bold(true).Foreground(tnWhite).Background(tnSelBg),
		Online:      lipgloss.NewStyle().Foreground(tnGreen),
		Offline:     lipgloss.NewStyle().Foreground(tnGray),
		Header:      lipgloss.NewStyle().Bold(true).Foreground(tnBlue),
		Footer:      lipgloss.NewStyle().Foreground(tnGray),
		Status:      lipgloss.NewStyle().Foreground(tnYellow),
		ErrText:     lipgloss.NewStyle().Foreground(tnRed),
		Modal:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(tnBlue),
		TabActive:   lipgloss.NewStyle().Bold(true).Foreground(tnSurface).Background(tnBlue),
		TabInactive: lipgloss.NewStyle().Foreground(tnGray),
		ModePill:    lipgloss.NewStyle().Bold(true).Foreground(tnSurface).Background(tnPurple),
	}
}
