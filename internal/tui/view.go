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
	if m.detail != nil {
		return m.detailView()
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
	if m.pending != nil {
		verb := "Block"
		if !m.pending.block {
			verb = "Unblock"
		}
		return m.styles.Status.Render(fmt.Sprintf("%s %s?", verb, m.pending.label)) +
			m.styles.Footer.Render("   y confirm • n cancel")
	}
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

// detailView renders the per-device pane: identity, status, and top peers.
func (m Model) detailView() string {
	d := m.detail.device
	now := m.now()
	status := "offline"
	statusStyle := m.styles.Offline
	if d.SeenWithin(onlineWindow, now) {
		status, statusStyle = "online", m.styles.Online
	}

	var b strings.Builder
	b.WriteString(m.styles.Title.Render(deviceLabel(d)))
	b.WriteString("  ")
	b.WriteString(statusStyle.Render(status))
	b.WriteString("\n\n")
	field := func(k, v string) {
		if v == "" {
			v = "—"
		}
		fmt.Fprintf(&b, "  %-10s %s\n", m.styles.Subtle.Render(k), v)
	}
	field("ip", d.IP)
	field("mac", d.MAC)
	field("vendor", d.Vendor)
	field("type", d.Type)
	field("last seen", lastSeen(d.LastActive, now))

	b.WriteString("\n")
	b.WriteString(m.styles.Header.Render("  Top traffic"))
	b.WriteString("\n")
	switch {
	case m.detail.loading:
		b.WriteString(m.styles.Subtle.Render("  loading…"))
	case m.detail.err != nil:
		b.WriteString(m.styles.ErrText.Render("  error: " + m.detail.err.Error()))
	case len(m.detail.peers) == 0:
		b.WriteString(m.styles.Subtle.Render("  no recent traffic"))
	default:
		peers := topPeers(m.detail.peers, 10)
		for _, p := range peers {
			label := p.Label
			if p.Kind == "device" && label == "" {
				label = p.PeerMAC
			}
			if len(label) > 34 {
				label = label[:33] + "…"
			}
			fmt.Fprintf(&b, "  %-34s  ↓%s ↑%s\n", label,
				humanizeBytes(p.Download), humanizeBytes(p.Upload))
		}
	}

	b.WriteString("\n")
	b.WriteString(m.styles.Footer.Render("b block • u unblock • esc back • q close"))
	return b.String()
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
