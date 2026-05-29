package tui

import (
	"fmt"
	"strings"

	"github.com/stefanpenner/fire.cli/internal/firewalla"
)

// View renders the dashboard.
func (m Model) View() string {
	if m.showHelp {
		return m.helpView()
	}

	var b strings.Builder
	b.WriteString(m.headerView())
	b.WriteString("\n\n")
	b.WriteString(m.listView())
	b.WriteString("\n")
	b.WriteString(m.footerView())
	return b.String()
}

func (m Model) headerView() string {
	host := "fire"
	if m.ds != nil {
		host = m.ds.Host()
	}
	online := 0
	now := m.now()
	for _, d := range m.devices {
		if d.SeenWithin(onlineWindow, now) {
			online++
		}
	}
	title := m.styles.Title.Render("🔥 fire") + m.styles.Subtle.Render("  "+host)
	counts := m.styles.Subtle.Render(fmt.Sprintf("%d devices • %d online", len(m.devices), online))
	return title + "   " + counts
}

func (m Model) listView() string {
	if m.loading && len(m.devices) == 0 {
		return m.styles.Subtle.Render("  loading…")
	}
	if m.err != nil {
		return m.styles.ErrText.Render("  error: " + m.err.Error())
	}
	if len(m.devices) == 0 {
		return m.styles.Subtle.Render("  no devices found")
	}
	if len(m.visible) == 0 {
		return m.styles.Subtle.Render("  no devices match the filter")
	}

	// Reserve rows for header, blank, footer, and (when searching) the input.
	maxRows := m.height - 4
	if m.searching {
		maxRows--
	}
	if maxRows < 1 {
		maxRows = 1
	}

	start := 0
	if m.cursor >= maxRows {
		start = m.cursor - maxRows + 1
	}
	end := min(start+maxRows, len(m.visible))

	var b strings.Builder
	for i := start; i < end; i++ {
		d := m.devices[m.visible[i]]
		b.WriteString(m.deviceRow(d, i == m.cursor))
		b.WriteString("\n")
	}
	if m.searching {
		b.WriteString(m.search.View())
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) deviceRow(d firewalla.Device, selected bool) string {
	dot, statusStyle := "○", m.styles.Offline
	if d.SeenWithin(onlineWindow, m.now()) {
		dot, statusStyle = "●", m.styles.Online
	}
	name := deviceLabel(d)
	if len(name) > 28 {
		name = name[:27] + "…"
	}
	ip := d.IP
	line := fmt.Sprintf("%-28s  %-15s  %s", name, ip, d.MAC)
	cursor := "  "
	if selected {
		cursor = m.styles.Title.Render("❯ ")
		return cursor + statusStyle.Render(dot) + " " + m.styles.Selected.Render(line)
	}
	return cursor + statusStyle.Render(dot) + " " + line
}

func (m Model) footerView() string {
	if m.err != nil {
		return m.styles.ErrText.Render(m.err.Error())
	}
	if m.searching {
		return m.styles.Footer.Render(m.keys.SearchHelp())
	}
	if m.status != "" {
		return m.styles.Status.Render(m.status) + m.styles.Footer.Render("   "+m.keys.ShortHelp())
	}
	return m.styles.Footer.Render(m.keys.ShortHelp())
}

func (m Model) helpView() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render("Keyboard Shortcuts"))
	b.WriteString("\n\n")
	for _, row := range m.keys.FullHelp() {
		fmt.Fprintf(&b, "  %-14s  %s\n", row[0], m.styles.Subtle.Render(row[1]))
	}
	b.WriteString("\n")
	b.WriteString(m.styles.Footer.Render("? or esc to close"))
	return m.styles.Modal.Render(b.String())
}
