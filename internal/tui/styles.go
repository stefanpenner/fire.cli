package tui

import "github.com/charmbracelet/lipgloss"

// Styles holds the lipgloss styles for the dashboard. Kept in one struct so a
// future theme/no-color variant can swap the whole set.
type Styles struct {
	Title    lipgloss.Style
	Subtle   lipgloss.Style
	Selected lipgloss.Style
	Online   lipgloss.Style
	Offline  lipgloss.Style
	Header   lipgloss.Style
	Footer   lipgloss.Style
	Status   lipgloss.Style
	ErrText  lipgloss.Style
	Modal    lipgloss.Style
}

// DefaultStyles returns the standard color styles.
func DefaultStyles() Styles {
	return Styles{
		Title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")),
		Subtle:   lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).Background(lipgloss.Color("63")),
		Online:   lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		Offline:  lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Header:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250")),
		Footer:   lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Status:   lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
		ErrText:  lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
		Modal:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("63")),
	}
}
