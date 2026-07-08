package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
	if m.view == networkView {
		return m.networksView()
	}
	if m.view == wanView {
		return m.wansView()
	}
	if m.view == dataView {
		return m.dataUsageView()
	}

	var b strings.Builder
	b.WriteString(m.headerView())
	b.WriteString("\n")
	b.WriteString(m.tabBar())
	b.WriteString("\n\n")
	b.WriteString(m.listView())
	b.WriteString("\n")
	b.WriteString(m.footerView())
	return b.String()
}

// loadingLine renders the animated spinner next to "loading…", shown while any
// view's data is in flight.
func (m Model) loadingLine() string {
	return "  " + m.spinner.View() + m.styles.Subtle.Render("loading…")
}

// tabBar renders the view switcher with the active view highlighted.
func (m Model) tabBar() string {
	tabs := []struct {
		v     viewMode
		label string
	}{
		{deviceView, "devices"},
		{ruleView, "rules"},
		{alarmView, "alarms"},
		{networkView, "networks"},
		{wanView, "wan"},
		{dataView, "data"},
	}
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		style := m.styles.TabInactive
		if t.v == m.view {
			style = m.styles.TabActive
		}
		parts[i] = style.Render(" " + t.label + " ")
	}
	return strings.Join(parts, m.styles.TabInactive.Render("│"))
}

// viewHeader renders a non-device view's title line plus the tab bar.
func (m Model) viewHeader(suffix string) string {
	return m.styles.TitleBadge.Render("🔥 fire") + m.styles.Subtle.Render(suffix) + "\n" + m.tabBar()
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
	title := m.styles.TitleBadge.Render("🔥 fire") + m.styles.Subtle.Render("  "+host)
	countText := fmt.Sprintf("%d devices • %d online", len(m.devices), online)
	if m.onlineOnly {
		countText += " • online only"
	}
	return title + "   " + m.styles.Subtle.Render(countText)
}

func (m Model) listView() string {
	if m.loading && len(m.devices) == 0 {
		return m.loadingLine()
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

// confirmBar renders the staged-action confirmation line shown in any view,
// led by a CONFIRM mode pill (LazyVim-style statusline).
func (m Model) confirmBar() string {
	return m.styles.ModePill.Render(" CONFIRM ") + " " +
		m.styles.Status.Render(m.pending.prompt) +
		m.styles.Footer.Render("   y confirm • n cancel")
}

// statusBar renders a LazyVim-style footer: a mode pill on the left, an
// optional transient status message, and the right-aligned help hint, padded
// to the window width.
func (m Model) statusBar(help string) string {
	mode := "NORMAL"
	if m.searching {
		mode = "SEARCH"
	}
	left := m.styles.ModePill.Render(" " + mode + " ")
	if m.status != "" {
		left += " " + m.styles.Status.Render(m.status)
	}
	right := m.styles.Footer.Render(help)
	w := m.width
	if w < 20 {
		w = 20
	}
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// viewFooter is the shared footer for every view: the confirm bar when an
// action is staged, otherwise the mode-pill status line with the view's help.
func (m Model) viewFooter(help string) string {
	if m.pending != nil {
		return m.confirmBar()
	}
	if m.searching {
		return m.statusBar(m.keys.SearchHelp())
	}
	return m.statusBar(help)
}

func (m Model) footerView() string {
	return m.viewFooter(m.keys.ShortHelp())
}

// detailView renders the pane opened with enter. Non-device items show a
// generic label/value field list; a device adds its live traffic + rules.
func (m Model) detailView() string {
	if !m.detail.isDevice {
		return m.fieldsDetailView()
	}
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
		b.WriteString(m.loadingLine())
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

	// Rules targeting this device (only meaningful once the load completes).
	if !m.detail.loading && m.detail.err == nil {
		b.WriteString("\n")
		b.WriteString(m.styles.Header.Render("  Rules"))
		b.WriteString("\n")
		if len(m.detail.rules) == 0 {
			b.WriteString(m.styles.Subtle.Render("  none"))
		} else {
			for _, r := range m.detail.rules {
				state, stStyle := "on ", m.styles.Online
				if r.Disabled {
					state, stStyle = "off", m.styles.Offline
				}
				fmt.Fprintf(&b, "  %s %-6s %-6s %s\n",
					stStyle.Render(state), r.ID, r.Action, r.Type)
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(m.viewFooter("b block • u unblock • esc back • q close"))
	return b.String()
}

// fieldsDetailView renders the generic label/value detail pane used by every
// non-device item (rules, alarms, networks, wan).
func (m Model) fieldsDetailView() string {
	var b strings.Builder
	b.WriteString(m.styles.TitleBadge.Render("🔥 fire"))
	b.WriteString(m.styles.Subtle.Render("  " + m.detail.title))
	b.WriteString("\n\n")
	for _, f := range m.detail.fields {
		v := f[1]
		if v == "" {
			v = "—"
		}
		fmt.Fprintf(&b, "  %s %s\n", m.styles.Subtle.Render(fmt.Sprintf("%-11s", f[0])), v)
	}
	b.WriteString("\n")
	b.WriteString(m.viewFooter("esc back • q close"))
	return b.String()
}

// ruleFields is the full field list for a rule's detail pane.
func ruleFields(r firewalla.Rule, now time.Time) [][2]string {
	state := "enabled"
	if r.Disabled {
		state = "disabled"
	}
	return [][2]string{
		{"id", r.ID},
		{"action", r.Action},
		{"type", r.Type},
		{"target", r.Target},
		{"direction", r.Direction},
		{"scope", r.Scope},
		{"state", state},
		{"hits", fmt.Sprintf("%d", r.HitCount)},
		{"last hit", lastSeen(r.LastHit, now)},
		{"created", lastSeen(r.Created, now)},
		{"notes", r.Notes},
	}
}

// alarmFields is the full field list for an alarm's detail pane.
func alarmFields(a firewalla.Alarm, now time.Time) [][2]string {
	return [][2]string{
		{"id", a.ID},
		{"type", a.Type},
		{"when", lastSeen(a.Time, now)},
		{"device", a.Device},
		{"mac", a.MAC},
		{"state", a.State},
		{"message", a.Message},
	}
}

// networkFields is the full field list for a network's detail pane.
func networkFields(n firewalla.Network) [][2]string {
	vlan := "—"
	if n.VLANID > 0 {
		vlan = fmt.Sprintf("%d", n.VLANID)
	}
	return [][2]string{
		{"name", n.Name},
		{"type", n.Type},
		{"interface", n.Interface},
		{"parent", n.Parent},
		{"vlan", vlan},
		{"conn", n.ConnType},
		{"subnet", n.Subnet},
		{"gateway", n.Gateway},
		{"dns", strings.Join(n.DNS, ", ")},
		{"uuid", n.UUID},
	}
}

// wanFields is the full field list for a WAN uplink's detail pane. A method
// because the health label is derived via wanHealth.
func (m Model) wanFields(w firewalla.WAN) [][2]string {
	health, _ := m.wanHealth(w)
	yesno := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}
	return [][2]string{
		{"name", w.Name},
		{"interface", w.Interface},
		{"role", w.Role},
		{"mode", w.Mode},
		{"active", yesno(w.Active)},
		{"health", health},
		{"carrier", yesno(w.Carrier)},
		{"ping", yesno(w.Ping)},
		{"dns", yesno(w.DNS)},
		{"uuid", w.UUID},
	}
}

// listBody renders the windowed rows for the active list view from the shared
// visible/cursor; row renders the underlying item at index idx.
func (m Model) listBody(reserve int, row func(idx int, selected bool) string) string {
	maxRows := m.height - reserve
	if maxRows < 1 {
		maxRows = 1
	}
	start := 0
	if m.cursor >= maxRows {
		start = m.cursor - maxRows + 1
	}
	end := min(start+maxRows, len(m.visible))
	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		rows = append(rows, row(m.visible[i], i == m.cursor))
	}
	return strings.Join(rows, "\n")
}

// searchLine renders the live filter input when searching.
func (m Model) searchLine() string {
	if m.searching {
		return "\n" + m.search.View()
	}
	return ""
}

// rulesView renders the firewall-rules list.
func (m Model) rulesView() string {
	var b strings.Builder
	b.WriteString(m.viewHeader(fmt.Sprintf("  rules (%d)", len(m.rules))))
	b.WriteString("\n\n")

	switch {
	case m.rulesLoading && len(m.rules) == 0:
		b.WriteString(m.loadingLine())
	case m.err != nil:
		b.WriteString(m.styles.ErrText.Render("  error: " + m.err.Error()))
	case len(m.rules) == 0:
		b.WriteString(m.styles.Subtle.Render("  no rules"))
	case len(m.visible) == 0:
		b.WriteString(m.styles.Subtle.Render("  no rules match the filter"))
	default:
		b.WriteString(m.listBody(4, func(i int, sel bool) string { return m.ruleRow(m.rules[i], sel) }))
	}

	b.WriteString(m.searchLine())
	b.WriteString("\n")
	b.WriteString(m.viewFooter(m.keys.RulesHelp()))
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
	b.WriteString(m.viewHeader(fmt.Sprintf("  alarms (%d)", len(m.alarms))))
	b.WriteString("\n\n")

	now := m.now()
	switch {
	case m.alarmsLoading && len(m.alarms) == 0:
		b.WriteString(m.loadingLine())
	case m.err != nil:
		b.WriteString(m.styles.ErrText.Render("  error: " + m.err.Error()))
	case len(m.alarms) == 0:
		b.WriteString(m.styles.Subtle.Render("  no alarms"))
	case len(m.visible) == 0:
		b.WriteString(m.styles.Subtle.Render("  no alarms match the filter"))
	default:
		b.WriteString(m.listBody(4, func(i int, sel bool) string { return m.alarmRow(m.alarms[i], sel, now) }))
	}

	b.WriteString(m.searchLine())
	b.WriteString("\n")
	b.WriteString(m.viewFooter(m.keys.AlarmsHelp()))
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

// networksView renders the configured networks/VLANs (read-only).
func (m Model) networksView() string {
	var b strings.Builder
	b.WriteString(m.viewHeader(fmt.Sprintf("  networks (%d)", len(m.networks))))
	b.WriteString("\n\n")

	switch {
	case m.networksLoading && len(m.networks) == 0:
		b.WriteString(m.loadingLine())
	case m.err != nil:
		b.WriteString(m.styles.ErrText.Render("  error: " + m.err.Error()))
	case len(m.networks) == 0:
		b.WriteString(m.styles.Subtle.Render("  no networks"))
	case len(m.visible) == 0:
		b.WriteString(m.styles.Subtle.Render("  no networks match the filter"))
	default:
		b.WriteString(m.listBody(5, func(i int, sel bool) string { return m.networkRow(m.networks[i], sel) }))
	}

	b.WriteString(m.searchLine())
	b.WriteString("\n")
	b.WriteString(m.viewFooter(m.keys.NetworksHelp()))
	return b.String()
}

func (m Model) networkRow(n firewalla.Network, selected bool) string {
	vlan := ""
	if n.VLANID > 0 {
		vlan = fmt.Sprintf("vlan %d", n.VLANID)
	}
	line := fmt.Sprintf("%-16s %-5s %-8s %-18s %s",
		truncate(n.Name, 16), n.Type, vlan, n.Subnet, n.Interface)
	if selected {
		return m.styles.Title.Render("❯ ") + m.styles.Selected.Render(line)
	}
	return "  " + line
}

// wansView renders the internet uplinks and their live health (read-only).
func (m Model) wansView() string {
	var b strings.Builder
	b.WriteString(m.viewHeader(fmt.Sprintf("  wan (%d)", len(m.wans))))
	b.WriteString("\n\n")

	switch {
	case m.wansLoading && len(m.wans) == 0:
		b.WriteString(m.loadingLine())
	case m.err != nil:
		b.WriteString(m.styles.ErrText.Render("  error: " + m.err.Error()))
	case len(m.wans) == 0:
		b.WriteString(m.styles.Subtle.Render("  no uplinks"))
	case len(m.visible) == 0:
		b.WriteString(m.styles.Subtle.Render("  no uplinks match the filter"))
	default:
		b.WriteString(m.listBody(5, func(i int, sel bool) string { return m.wanRow(m.wans[i], sel) }))
	}

	b.WriteString(m.searchLine())
	b.WriteString("\n")
	b.WriteString(m.viewFooter(m.keys.WANHelp()))
	return b.String()
}

func (m Model) wanRow(w firewalla.WAN, selected bool) string {
	state, stStyle := m.wanHealth(w)
	inUse := " "
	if w.Active {
		inUse = "●"
	}
	line := fmt.Sprintf("%-16s %-8s %-9s %-16s", truncate(w.Name, 16), w.Interface, w.Role, state)
	if selected {
		return m.styles.Title.Render("❯ ") + stStyle.Render(inUse) + " " + m.styles.Selected.Render(line)
	}
	return "  " + stStyle.Render(inUse) + " " + line
}

// wanHealth summarizes a WAN's carrier/ping/dns checks as a label + style.
func (m Model) wanHealth(w firewalla.WAN) (string, lipgloss.Style) {
	switch {
	case !w.Carrier:
		return "down", m.styles.Offline
	case w.Ping && w.DNS:
		return "healthy", m.styles.Online
	case w.Ping || w.DNS:
		return "degraded", m.styles.Status
	default:
		return "no connectivity", m.styles.Offline
	}
}

// dataUsageView renders the data-plan summary and per-WAN usage (read-only).
func (m Model) dataUsageView() string {
	var b strings.Builder
	b.WriteString(m.viewHeader("  data usage"))
	b.WriteString("\n\n")

	switch {
	case m.dataLoading && len(m.data.WANs) == 0 && m.data.PlanTotal == 0:
		b.WriteString(m.loadingLine())
	case m.err != nil:
		b.WriteString(m.styles.ErrText.Render("  error: " + m.err.Error()))
	default:
		total := m.data.Total()
		if m.data.PlanTotal > 0 {
			pct := float64(total) / float64(m.data.PlanTotal) * 100
			fmt.Fprintf(&b, "  %s of %s plan  %s, resets day %d\n",
				m.styles.Header.Render(humanizeBytes(total)),
				humanizeBytes(m.data.PlanTotal),
				m.styles.Status.Render(fmt.Sprintf("(%.1f%%)", pct)),
				m.data.ResetDay)
		} else {
			fmt.Fprintf(&b, "  %s used\n", m.styles.Header.Render(humanizeBytes(total)))
		}
		b.WriteString("\n")
		if len(m.data.WANs) == 0 {
			b.WriteString(m.styles.Subtle.Render("  no per-WAN usage"))
		} else {
			for _, w := range m.data.WANs {
				name := m.dataNames[w.UUID]
				if name == "" {
					name = w.UUID
				}
				fmt.Fprintf(&b, "  %-16s  ↑%s ↓%s  (%s)\n",
					truncate(name, 16), humanizeBytes(w.Upload), humanizeBytes(w.Download),
					humanizeBytes(w.Bytes()))
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(m.viewFooter(m.keys.DataHelp()))
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
