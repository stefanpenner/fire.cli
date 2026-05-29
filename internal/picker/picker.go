package picker

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

// Interactive reports whether an interactive picker can run: both stdin and the
// chosen output are real terminals.
func Interactive(out io.Writer) bool {
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(f.Fd())
}

var (
	promptStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	matchStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	countStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// ErrCancelled is returned when the user dismisses the picker (Esc/Ctrl-C).
var ErrCancelled = fmt.Errorf("selection cancelled")

// Select runs an interactive fuzzy finder over items, displayed on out (use
// stderr to keep stdout clean for the result). It returns the chosen item's
// index in items, or ErrCancelled. maxRows caps the visible list height.
func Select(out io.Writer, prompt string, items []string, maxRows int) (int, error) {
	if len(items) == 0 {
		return -1, fmt.Errorf("nothing to choose from")
	}
	if maxRows <= 0 {
		maxRows = 10
	}
	ti := textinput.New()
	ti.Placeholder = "type to filter…"
	ti.Focus()
	ti.Prompt = "" // we render our own prompt label

	m := &model{prompt: prompt, items: items, input: ti, maxRows: maxRows, chosen: -1}
	m.refilter()

	p := tea.NewProgram(m, tea.WithOutput(out), tea.WithInput(os.Stdin))
	res, err := p.Run()
	if err != nil {
		return -1, err
	}
	fm := res.(*model)
	if fm.cancelled || fm.chosen < 0 {
		return -1, ErrCancelled
	}
	return fm.chosen, nil
}

type model struct {
	prompt    string
	items     []string
	input     textinput.Model
	matches   []Match
	cursor    int
	maxRows   int
	offset    int
	chosen    int
	cancelled bool
}

func (m *model) Init() tea.Cmd { return textinput.Blink }

func (m *model) refilter() {
	m.matches = Rank(m.items, m.input.Value())
	if m.cursor >= len(m.matches) {
		m.cursor = len(m.matches) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampOffset()
}

func (m *model) clampOffset() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.maxRows {
		m.offset = m.cursor - m.maxRows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if len(m.matches) > 0 {
				m.chosen = m.matches[m.cursor].Index
			}
			return m, tea.Quit
		case "up", "ctrl+p", "ctrl+k":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.clampOffset()
			return m, nil
		case "down", "ctrl+n", "ctrl+j":
			m.cursor++
			if m.cursor >= len(m.matches) {
				m.cursor = len(m.matches) - 1
			}
			m.clampOffset()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refilter()
	return m, cmd
}

func (m *model) View() string {
	var b strings.Builder
	b.WriteString(promptStyle.Render(m.prompt) + " " + m.input.View() + "\n")
	b.WriteString(countStyle.Render(fmt.Sprintf("  %d/%d", len(m.matches), len(m.items))) + "\n")

	end := m.offset + m.maxRows
	if end > len(m.matches) {
		end = len(m.matches)
	}
	for i := m.offset; i < end; i++ {
		match := m.matches[i]
		line := highlight(m.items[match.Index], match.Positions)
		if i == m.cursor {
			b.WriteString(cursorStyle.Render("❯ ") + selectedStyle.Render(stripStyle(m.items[match.Index])) + "\n")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}
	b.WriteString(dimStyle.Render("↑/↓ move • enter select • esc cancel"))
	return b.String()
}

// highlight renders text with matched rune positions emphasized.
func highlight(text string, positions []int) string {
	if len(positions) == 0 {
		return text
	}
	hit := make(map[int]bool, len(positions))
	for _, p := range positions {
		hit[p] = true
	}
	var b strings.Builder
	for i, r := range []rune(text) {
		if hit[i] {
			b.WriteString(matchStyle.Render(string(r)))
		} else {
			b.WriteString(string(r))
		}
	}
	return b.String()
}

// stripStyle returns text unchanged; the selected row is fully styled so we
// don't double-apply match highlighting under the selection color.
func stripStyle(text string) string { return text }
