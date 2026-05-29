package tui

import (
	"fmt"
	"strings"
	"time"

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
	if m.view == ruleView {
		return m.rulesView()
	}
	if m.view == alarmView {
		return m.alarmsView()
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

// confirmBar renders the staged-action confirmation line shown in any view.
func (m Model) confirmBar() string {
	return m.styles.Status.Render(m.pending.prompt) +
		m.styles.Footer.Render("   y confirm • n cancel")
}

func (m Model) footerView() string {
	if m.pending != nil {
		return m.confirmBar()
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
	if m.pending != nil {
		b.WriteString(m.confirmBar())
	} else {
		b.WriteString(m.styles.Footer.Render("b block • u unblock • esc back • q close"))
	}
	return b.String()
}

// rulesView renders the firewall-rules list.
func (m Model) rulesView() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render("🔥 fire") +
		m.styles.Subtle.Render(fmt.Sprintf("  rules (%d)", len(m.rules))))
	b.WriteString("\n\n")

	switch {
	case m.rulesLoading && len(m.rules) == 0:
		b.WriteString(m.styles.Subtle.Render("  loading…"))
	case m.err != nil:
		b.WriteString(m.styles.ErrText.Render("  error: " + m.err.Error()))
	case len(m.rules) == 0:
		b.WriteString(m.styles.Subtle.Render("  no rules"))
	default:
		maxRows := m.height - 4
		if maxRows < 1 {
			maxRows = 1
		}
		start := 0
		if m.ruleCursor >= maxRows {
			start = m.ruleCursor - maxRows + 1
		}
		end := min(start+maxRows, len(m.rules))
		rows := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			rows = append(rows, m.ruleRow(m.rules[i], i == m.ruleCursor))
		}
		b.WriteString(strings.Join(rows, "\n"))
	}

	b.WriteString("\n")
	switch {
	case m.pending != nil:
		b.WriteString(m.confirmBar())
	case m.status != "":
		b.WriteString(m.styles.Status.Render(m.status) + m.styles.Footer.Render("   "+m.keys.RulesHelp()))
	default:
		b.WriteString(m.styles.Footer.Render(m.keys.RulesHelp()))
	}
	return b.String()
}

func (m Model) ruleRow(r firewalla.Rule, selected bool) string {
	state, stStyle := "on ", m.styles.Online
	if r.Disabled {
		state, stStyle = "off", m.styles.Offline
	}
	target := r.Target
	if len(target) > 30 {
		target = target[:29] + "…"
	}
	line := fmt.Sprintf("%-6s %-6s %-9s %-30s", r.ID, r.Action, r.Type, target)
	if selected {
		return m.styles.Title.Render("❯ ") + stStyle.Render(state) + " " + m.styles.Selected.Render(line)
	}
	return "  " + stStyle.Render(state) + " " + line
}

// alarmsView renders the recent-alarms list.
func (m Model) alarmsView() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render("🔥 fire") +
		m.styles.Subtle.Render(fmt.Sprintf("  alarms (%d)", len(m.alarms))))
	b.WriteString("\n\n")

	now := m.now()
	switch {
	case m.alarmsLoading && len(m.alarms) == 0:
		b.WriteString(m.styles.Subtle.Render("  loading…"))
	case m.err != nil:
		b.WriteString(m.styles.ErrText.Render("  error: " + m.err.Error()))
	case len(m.alarms) == 0:
		b.WriteString(m.styles.Subtle.Render("  no alarms"))
	default:
		maxRows := m.height - 4
		if maxRows < 1 {
			maxRows = 1
		}
		start := 0
		if m.alarmCursor >= maxRows {
			start = m.alarmCursor - maxRows + 1
		}
		end := min(start+maxRows, len(m.alarms))
		rows := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			rows = append(rows, m.alarmRow(m.alarms[i], i == m.alarmCursor, now))
		}
		b.WriteString(strings.Join(rows, "\n"))
	}

	b.WriteString("\n")
	switch {
	case m.pending != nil:
		b.WriteString(m.confirmBar())
	case m.status != "":
		b.WriteString(m.styles.Status.Render(m.status) + m.styles.Footer.Render("   "+m.keys.AlarmsHelp()))
	default:
		b.WriteString(m.styles.Footer.Render(m.keys.AlarmsHelp()))
	}
	return b.String()
}

func (m Model) alarmRow(a firewalla.Alarm, selected bool, now time.Time) string {
	when := lastSeen(a.Time, now)
	desc := a.Message
	if desc == "" {
		desc = a.Device
	}
	if len(desc) > 32 {
		desc = desc[:31] + "…"
	}
	line := fmt.Sprintf("%-8s %-9s %-18s %-32s", a.ID, when, truncate(a.Type, 18), desc)
	if selected {
		return m.styles.Title.Render("❯ ") + m.styles.Selected.Render(line)
	}
	return "  " + line
}

// truncate shortens s to at most n runes, adding an ellipsis when cut.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
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
