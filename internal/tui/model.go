// Package tui is an interactive Bubble Tea dashboard for a Firewalla box: a
// searchable device list with inline block/unblock, built on the same typed
// firewalla data the CLI commands use. The Model is a value type; Update
// returns (tea.Model, tea.Cmd) so it can be driven directly in tests.
package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stefanpenner/fire.cli/internal/firewalla"
)

// DataSource is the slice of the firewalla client the dashboard needs.
// *firewalla.Client satisfies it; tests supply a fake.
type DataSource interface {
	Host() string
	ListDevices(ctx context.Context) ([]firewalla.Device, error)
	Traffic(ctx context.Context, mac string) ([]firewalla.Peer, error)
	CreateRule(ctx context.Context, spec firewalla.RuleSpec) (string, error)
	DeleteMatching(ctx context.Context, spec firewalla.RuleSpec) (int, error)
	ListRules(ctx context.Context) ([]firewalla.Rule, error)
	SetRuleDisabled(ctx context.Context, id string, disabled bool) error
	DeleteRule(ctx context.Context, id string) error
	ListAlarms(ctx context.Context, limit int) ([]firewalla.Alarm, error)
	ArchiveAlarm(ctx context.Context, id string) error
	DeleteAlarm(ctx context.Context, id string) error
	ListNetworks(ctx context.Context) ([]firewalla.Network, error)
	ListWANs(ctx context.Context) ([]firewalla.WAN, error)
	DataUsage(ctx context.Context) (firewalla.DataUsageReport, error)
}

// alarmViewLimit caps how many alarms the alarms view loads.
const alarmViewLimit = 50

// viewMode selects which list the dashboard shows.
type viewMode int

const (
	deviceView viewMode = iota
	ruleView
	alarmView
	networkView
	wanView
	dataView
)

// devicesMsg carries the result of a device (re)load.
type devicesMsg struct {
	devices []firewalla.Device
	err     error
}

// rulesMsg carries the result of a rules (re)load.
type rulesMsg struct {
	rules []firewalla.Rule
	err   error
}

// alarmsMsg carries the result of an alarms (re)load.
type alarmsMsg struct {
	alarms []firewalla.Alarm
	err    error
}

// networksMsg carries the result of a networks (re)load.
type networksMsg struct {
	networks []firewalla.Network
	err      error
}

// wansMsg carries the result of a WAN (re)load.
type wansMsg struct {
	wans []firewalla.WAN
	err  error
}

// dataMsg carries the result of a data-usage (re)load, with WAN uuid→name
// resolved from the network list.
type dataMsg struct {
	report firewalla.DataUsageReport
	names  map[string]string
	err    error
}

// actionMsg carries the result of a confirmed mutation.
type actionMsg struct {
	text string
	err  error
}

// detailMsg carries the loaded traffic for a device's detail pane.
type detailMsg struct {
	mac   string
	peers []firewalla.Peer
	rules []firewalla.Rule // rules targeting this device (supplementary)
	err   error
}

const onlineWindow = 5 * time.Minute

// Model is the dashboard state.
type Model struct {
	ds     DataSource
	now    func() time.Time
	keys   KeyMap
	styles Styles

	width, height int

	view viewMode // which list is showing

	devices    []firewalla.Device // sorted: online first, then most-recent
	visible    []int              // indices into devices after the active filter
	cursor     int                // index into visible
	onlineOnly bool               // when set, hide offline devices

	rules        []firewalla.Rule
	ruleCursor   int
	rulesLoading bool

	alarms        []firewalla.Alarm
	alarmCursor   int
	alarmsLoading bool

	networks        []firewalla.Network
	networkCursor   int
	networksLoading bool

	wans        []firewalla.WAN
	wanCursor   int
	wansLoading bool

	data        firewalla.DataUsageReport
	dataNames   map[string]string
	dataLoading bool

	search    textinput.Model
	searching bool
	spinner   spinner.Model

	showHelp bool
	loading  bool
	status   string         // transient feedback (e.g. "blocked Phone")
	pending  *pendingAction // a mutation awaiting y/n confirmation
	detail   *detailState   // open device detail pane, or nil
	err      error
}

// detailState backs the detail pane opened with enter in any list view. Every
// item shows a title + label/value fields; a device additionally loads its top
// traffic peers and the rules targeting it (the async extras).
type detailState struct {
	title  string
	fields [][2]string

	isDevice bool
	device   firewalla.Device
	peers    []firewalla.Peer
	rules    []firewalla.Rule
	loading  bool
	err      error
}

// pendingAction is a destructive action staged for confirmation, mirroring the
// CLI's --confirm gate so the dashboard never mutates on a single keypress. The
// prompt is shown in the footer; cmd fires on y/enter.
type pendingAction struct {
	prompt string
	cmd    tea.Cmd
}

// NewModel builds a dashboard over ds. now defaults to time.Now when nil.
func NewModel(ds DataSource, now func() time.Time) Model {
	if now == nil {
		now = time.Now
	}
	ti := textinput.New()
	ti.Placeholder = "filter by name, IP, or MAC…"
	ti.Prompt = "/"
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	return Model{
		ds:      ds,
		now:     now,
		keys:    DefaultKeyMap(),
		styles:  DefaultStyles(),
		width:   80,
		height:  24,
		search:  ti,
		spinner: sp,
		loading: true,
	}
}

// WithColor selects colored or plain (NO_COLOR-friendly) styles. Chainable on
// the constructor: NewModel(ds, now).WithColor(render.ColorEnabled(...)).
func (m Model) WithColor(enabled bool) Model {
	if enabled {
		m.styles = DefaultStyles()
	} else {
		m.styles = PlainStyles()
	}
	return m
}

// viewOrder is the left-to-right tab order, shared by the tab bar and the
// tab-cycling navigation (tab/⇧tab, h/l, ←/→).
var viewOrder = []viewMode{deviceView, ruleView, alarmView, networkView, wanView, dataView}

// cycleView moves delta tabs from the current view, wrapping around, and
// switches to it.
func (m Model) cycleView(delta int) (tea.Model, tea.Cmd) {
	idx := 0
	for i, v := range viewOrder {
		if v == m.view {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(viewOrder)) % len(viewOrder)
	return m.switchTo(viewOrder[idx])
}

// switchTo changes the active view and kicks off its (re)load, clearing any
// transient status/error. Shared by the tab-cycling nav, the letter shortcuts
// (R/A/N/W/D), and the number keys (1–6).
func (m Model) switchTo(v viewMode) (tea.Model, tea.Cmd) {
	m.view = v
	m.status, m.err = "", nil
	switch v {
	case ruleView:
		m.rulesLoading = true
		return m, m.loadRulesCmd()
	case alarmView:
		m.alarmsLoading = true
		return m, m.loadAlarmsCmd()
	case networkView:
		m.networksLoading = true
		return m, m.loadNetworksCmd()
	case wanView:
		m.wansLoading = true
		return m, m.loadWansCmd()
	case dataView:
		m.dataLoading = true
		return m, m.loadDataCmd()
	default:
		m.loading = true
		return m, m.loadCmd()
	}
}

// Init kicks off the first device load and starts the spinner animation.
func (m Model) Init() tea.Cmd { return tea.Batch(m.loadCmd(), m.spinner.Tick) }

// loadCmd fetches devices off the UI goroutine.
func (m Model) loadCmd() tea.Cmd {
	ds := m.ds
	return func() tea.Msg {
		devs, err := ds.ListDevices(context.Background())
		return devicesMsg{devices: devs, err: err}
	}
}

// Update handles messages and key input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	// List loads carry no view tag; the msg type identifies the view. A result
	// that arrives after the user switched away is stale and dropped, so it
	// cannot clobber the current view's state. See specs/TuiLoad.tla.
	case devicesMsg:
		if m.view != deviceView {
			return m, nil
		}
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.setDevices(msg.devices)
		}
		return m, nil

	case rulesMsg:
		if m.view != ruleView {
			return m, nil
		}
		m.rulesLoading = false
		m.err = msg.err
		if msg.err == nil {
			m.rules = msg.rules
			m.ruleCursor = clampIndex(m.ruleCursor, len(m.rules))
		}
		return m, nil

	case alarmsMsg:
		if m.view != alarmView {
			return m, nil
		}
		m.alarmsLoading = false
		m.err = msg.err
		if msg.err == nil {
			m.alarms = msg.alarms
			m.alarmCursor = clampIndex(m.alarmCursor, len(m.alarms))
		}
		return m, nil

	case networksMsg:
		if m.view != networkView {
			return m, nil
		}
		m.networksLoading = false
		m.err = msg.err
		if msg.err == nil {
			m.networks = msg.networks
			m.networkCursor = clampIndex(m.networkCursor, len(m.networks))
		}
		return m, nil

	case wansMsg:
		if m.view != wanView {
			return m, nil
		}
		m.wansLoading = false
		m.err = msg.err
		if msg.err == nil {
			m.wans = msg.wans
			m.wanCursor = clampIndex(m.wanCursor, len(m.wans))
		}
		return m, nil

	case dataMsg:
		if m.view != dataView {
			return m, nil
		}
		m.dataLoading = false
		m.err = msg.err
		if msg.err == nil {
			m.data, m.dataNames = msg.report, msg.names
		}
		return m, nil

	case actionMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.status = msg.text
		}
		// Refresh whichever list is showing so it reflects the change.
		switch m.view {
		case ruleView:
			m.rulesLoading = true
			return m, m.loadRulesCmd()
		case alarmView:
			m.alarmsLoading = true
			return m, m.loadAlarmsCmd()
		default:
			m.loading = true
			return m, m.loadCmd()
		}

	case detailMsg:
		if m.detail != nil && m.detail.device.MAC == msg.mac {
			m.detail.loading = false
			m.detail.peers, m.detail.rules, m.detail.err = msg.peers, msg.rules, msg.err
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward anything else (e.g. blink) to the search input when active.
	if m.searching {
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.refilter()
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// A staged block/unblock awaits y/n before anything else.
	if m.pending != nil {
		return m.confirmKey(msg)
	}

	// Detail pane: esc/q/enter closes. block/unblock still work, but only on a
	// device detail (they are meaningless for a rule/alarm/network/wan).
	if m.detail != nil {
		switch {
		case key.Matches(msg, m.keys.Cancel), key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Enter):
			m.detail = nil
		case m.detail.isDevice && key.Matches(msg, m.keys.Block):
			m.stageAction(true)
		case m.detail.isDevice && key.Matches(msg, m.keys.Unblock):
			m.stageAction(false)
		}
		return m, nil
	}

	// Search mode swallows most keys for the text input.
	if m.searching {
		switch {
		case key.Matches(msg, m.keys.Enter):
			m.searching = false
			m.search.Blur()
			return m, nil
		case key.Matches(msg, m.keys.Cancel):
			m.searching = false
			m.search.Blur()
			m.search.SetValue("")
			m.refilter()
			return m, nil
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.refilter()
		return m, cmd
	}

	// Help modal: any of help/quit/cancel closes it.
	if m.showHelp {
		switch {
		case key.Matches(msg, m.keys.Help), key.Matches(msg, m.keys.Cancel), key.Matches(msg, m.keys.Quit):
			m.showHelp = false
		}
		return m, nil
	}

	// Keys common to every list view.
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
		return m, nil
	}
	// View navigation works from ANY view, before per-view dispatch, so it is
	// consistent everywhere: tab/⇧tab (and h/l, ←/→) cycle adjacent tabs, the
	// number keys jump directly, and each view's letter jumps straight to it.
	switch {
	case key.Matches(msg, m.keys.NextTab):
		return m.cycleView(1)
	case key.Matches(msg, m.keys.PrevTab):
		return m.cycleView(-1)
	case key.Matches(msg, m.keys.Rules):
		return m.switchTo(ruleView)
	case key.Matches(msg, m.keys.Alarms):
		return m.switchTo(alarmView)
	case key.Matches(msg, m.keys.Networks):
		return m.switchTo(networkView)
	case key.Matches(msg, m.keys.WAN):
		return m.switchTo(wanView)
	case key.Matches(msg, m.keys.Data):
		return m.switchTo(dataView)
	}
	switch msg.String() {
	case "1":
		return m.switchTo(deviceView)
	case "2":
		return m.switchTo(ruleView)
	case "3":
		return m.switchTo(alarmView)
	case "4":
		return m.switchTo(networkView)
	case "5":
		return m.switchTo(wanView)
	case "6":
		return m.switchTo(dataView)
	}

	switch m.view {
	case ruleView:
		return m.handleRuleKey(msg)
	case alarmView:
		return m.handleAlarmKey(msg)
	case networkView:
		return m.handleNetworkKey(msg)
	case wanView:
		return m.handleWanKey(msg)
	case dataView:
		return m.handleDataKey(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Up):
		m.moveCursor(-1)
	case key.Matches(msg, m.keys.Down):
		m.moveCursor(1)
	case key.Matches(msg, m.keys.GoTop):
		m.cursor = 0
	case key.Matches(msg, m.keys.GoBot):
		m.cursor = max(len(m.visible)-1, 0)
	case key.Matches(msg, m.keys.OnlineOnly):
		m.onlineOnly = !m.onlineOnly
		m.refilter()
	case key.Matches(msg, m.keys.Search):
		m.searching = true
		m.status = ""
		m.search.Focus()
		return m, textinput.Blink
	case key.Matches(msg, m.keys.Reload):
		m.loading, m.status, m.err = true, "", nil
		return m, m.loadCmd()
	case key.Matches(msg, m.keys.Enter):
		if d, ok := m.SelectedDevice(); ok {
			m.detail = &detailState{isDevice: true, device: d, title: deviceLabel(d), loading: true}
			m.status = ""
			return m, m.detailCmd(d)
		}
	case key.Matches(msg, m.keys.Block):
		m.stageAction(true)
	case key.Matches(msg, m.keys.Unblock):
		m.stageAction(false)
	}
	return m, nil
}

// detailCmd loads the selected device's traffic peers (and the rules targeting
// it) off the UI goroutine.
func (m Model) detailCmd(d firewalla.Device) tea.Cmd {
	ds, mac := m.ds, d.MAC
	return func() tea.Msg {
		peers, err := ds.Traffic(context.Background(), mac)
		if err != nil {
			return detailMsg{mac: mac, err: err}
		}
		// Rules are supplementary; ignore their error so a rules hiccup doesn't
		// blank the whole pane.
		rules, _ := ds.ListRules(context.Background())
		return detailMsg{mac: mac, peers: peers, rules: rulesForMAC(rules, mac)}
	}
}

// rulesForMAC returns the rules that target the given device MAC.
func rulesForMAC(rules []firewalla.Rule, mac string) []firewalla.Rule {
	var out []firewalla.Rule
	for _, r := range rules {
		if strings.EqualFold(r.Target, mac) {
			out = append(out, r)
		}
	}
	return out
}

// stageAction records a block/unblock for the selected device, to be confirmed
// with y. A no-op when the (filtered) list is empty.
func (m *Model) stageAction(block bool) {
	d, ok := m.SelectedDevice()
	if !ok {
		return
	}
	verb := "Block"
	if !block {
		verb = "Unblock"
	}
	m.status = ""
	m.pending = &pendingAction{
		prompt: fmt.Sprintf("%s %s?", verb, deviceLabel(d)),
		cmd:    m.blockCmd(block, deviceLabel(d), d.MAC),
	}
}

// blockCmd builds the command that blocks/unblocks a device by MAC.
func (m Model) blockCmd(block bool, label, mac string) tea.Cmd {
	ds := m.ds
	spec := firewalla.RuleSpec{Action: "block", Type: "mac", Target: mac, Notes: "via fire tui"}
	return func() tea.Msg {
		if block {
			if _, err := ds.CreateRule(context.Background(), spec); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{text: "blocked " + label}
		}
		n, err := ds.DeleteMatching(context.Background(), spec)
		if err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: fmt.Sprintf("unblocked %s (%d rule(s))", label, n)}
	}
}

// confirmKey handles y/n (and esc) while an action is staged.
func (m Model) confirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		cmd := m.pending.cmd
		m.pending = nil
		return m, cmd
	case "n", "N", "esc", "q", "ctrl+c":
		m.pending = nil
	}
	return m, nil
}

// ---- rules view ----

// handleRuleKey handles keys while the rules list is showing.
func (m Model) handleRuleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.view = deviceView
	case key.Matches(msg, m.keys.Up):
		m.ruleCursor = clampIndex(m.ruleCursor-1, len(m.rules))
	case key.Matches(msg, m.keys.Down):
		m.ruleCursor = clampIndex(m.ruleCursor+1, len(m.rules))
	case key.Matches(msg, m.keys.GoTop):
		m.ruleCursor = 0
	case key.Matches(msg, m.keys.GoBot):
		m.ruleCursor = max(len(m.rules)-1, 0)
	case key.Matches(msg, m.keys.Reload):
		m.rulesLoading, m.status, m.err = true, "", nil
		return m, m.loadRulesCmd()
	case key.Matches(msg, m.keys.Enter):
		if r, ok := m.selectedRule(); ok {
			return m.openDetail("rule "+r.ID, ruleFields(r, m.now()))
		}
	case key.Matches(msg, m.keys.RuleEnable):
		m.stageRule("enable")
	case key.Matches(msg, m.keys.RuleDisable):
		m.stageRule("disable")
	case key.Matches(msg, m.keys.RuleDelete):
		m.stageRule("delete")
	}
	return m, nil
}

// openDetail stages a non-device detail pane (a label/value field list) for the
// selected item. Returns m unchanged so it composes in the key switches.
func (m Model) openDetail(title string, fields [][2]string) (tea.Model, tea.Cmd) {
	m.status = ""
	m.detail = &detailState{title: title, fields: fields}
	return m, nil
}

// clampIndex keeps i within [0, n) (returning 0 when n == 0).
func clampIndex(i, n int) int {
	if i >= n {
		i = n - 1
	}
	if i < 0 {
		i = 0
	}
	return i
}

// loadRulesCmd fetches rules off the UI goroutine.
func (m Model) loadRulesCmd() tea.Cmd {
	ds := m.ds
	return func() tea.Msg {
		rules, err := ds.ListRules(context.Background())
		return rulesMsg{rules: rules, err: err}
	}
}

// selectedRule returns the rule under the rules cursor.
func (m Model) selectedRule() (firewalla.Rule, bool) {
	if m.ruleCursor < 0 || m.ruleCursor >= len(m.rules) {
		return firewalla.Rule{}, false
	}
	return m.rules[m.ruleCursor], true
}

// ruleVerbs maps an action kind to its display verb and result participle.
var ruleVerbs = map[string][2]string{
	"enable":  {"Enable", "enabled"},
	"disable": {"Disable", "disabled"},
	"delete":  {"Delete", "deleted"},
}

// stageRule stages an enable/disable/delete for the selected rule.
func (m *Model) stageRule(kind string) {
	r, ok := m.selectedRule()
	if !ok {
		return
	}
	m.status = ""
	m.pending = &pendingAction{
		prompt: fmt.Sprintf("%s rule %s?", ruleVerbs[kind][0], r.ID),
		cmd:    m.ruleCmd(kind, r.ID),
	}
}

// ruleCmd builds the command for a rule mutation.
func (m Model) ruleCmd(kind, id string) tea.Cmd {
	ds := m.ds
	return func() tea.Msg {
		var err error
		switch kind {
		case "enable":
			err = ds.SetRuleDisabled(context.Background(), id, false)
		case "disable":
			err = ds.SetRuleDisabled(context.Background(), id, true)
		case "delete":
			err = ds.DeleteRule(context.Background(), id)
		}
		if err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: ruleVerbs[kind][1] + " rule " + id}
	}
}

// ---- alarms view ----

// handleAlarmKey handles keys while the alarms list is showing.
func (m Model) handleAlarmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.view = deviceView
	case key.Matches(msg, m.keys.Up):
		m.alarmCursor = clampIndex(m.alarmCursor-1, len(m.alarms))
	case key.Matches(msg, m.keys.Down):
		m.alarmCursor = clampIndex(m.alarmCursor+1, len(m.alarms))
	case key.Matches(msg, m.keys.GoTop):
		m.alarmCursor = 0
	case key.Matches(msg, m.keys.GoBot):
		m.alarmCursor = max(len(m.alarms)-1, 0)
	case key.Matches(msg, m.keys.Reload):
		m.alarmsLoading, m.status, m.err = true, "", nil
		return m, m.loadAlarmsCmd()
	case key.Matches(msg, m.keys.Enter):
		if a, ok := m.selectedAlarm(); ok {
			return m.openDetail("alarm "+a.ID, alarmFields(a, m.now()))
		}
	case key.Matches(msg, m.keys.AlarmArchive):
		m.stageAlarm("archive")
	case key.Matches(msg, m.keys.RuleDelete): // 'x' deletes in any list view
		m.stageAlarm("delete")
	}
	return m, nil
}

// loadAlarmsCmd fetches recent alarms off the UI goroutine.
func (m Model) loadAlarmsCmd() tea.Cmd {
	ds := m.ds
	return func() tea.Msg {
		alarms, err := ds.ListAlarms(context.Background(), alarmViewLimit)
		return alarmsMsg{alarms: alarms, err: err}
	}
}

// selectedAlarm returns the alarm under the alarms cursor.
func (m Model) selectedAlarm() (firewalla.Alarm, bool) {
	if m.alarmCursor < 0 || m.alarmCursor >= len(m.alarms) {
		return firewalla.Alarm{}, false
	}
	return m.alarms[m.alarmCursor], true
}

// alarmVerbs maps an action kind to its display verb and result participle.
var alarmVerbs = map[string][2]string{
	"archive": {"Archive", "archived"},
	"delete":  {"Delete", "deleted"},
}

// stageAlarm stages an archive/delete for the selected alarm.
func (m *Model) stageAlarm(kind string) {
	a, ok := m.selectedAlarm()
	if !ok {
		return
	}
	m.status = ""
	m.pending = &pendingAction{
		prompt: fmt.Sprintf("%s alarm %s?", alarmVerbs[kind][0], a.ID),
		cmd:    m.alarmCmd(kind, a.ID),
	}
}

// alarmCmd builds the command for an alarm mutation.
func (m Model) alarmCmd(kind, id string) tea.Cmd {
	ds := m.ds
	return func() tea.Msg {
		var err error
		switch kind {
		case "archive":
			err = ds.ArchiveAlarm(context.Background(), id)
		case "delete":
			err = ds.DeleteAlarm(context.Background(), id)
		}
		if err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: alarmVerbs[kind][1] + " alarm " + id}
	}
}

// ---- networks view (read-only) ----

// handleNetworkKey handles keys while the networks list is showing.
func (m Model) handleNetworkKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.view = deviceView
	case key.Matches(msg, m.keys.Up):
		m.networkCursor = clampIndex(m.networkCursor-1, len(m.networks))
	case key.Matches(msg, m.keys.Down):
		m.networkCursor = clampIndex(m.networkCursor+1, len(m.networks))
	case key.Matches(msg, m.keys.GoTop):
		m.networkCursor = 0
	case key.Matches(msg, m.keys.GoBot):
		m.networkCursor = max(len(m.networks)-1, 0)
	case key.Matches(msg, m.keys.Reload):
		m.networksLoading, m.status, m.err = true, "", nil
		return m, m.loadNetworksCmd()
	case key.Matches(msg, m.keys.Enter):
		if m.networkCursor >= 0 && m.networkCursor < len(m.networks) {
			n := m.networks[m.networkCursor]
			return m.openDetail("network "+n.Name, networkFields(n))
		}
	}
	return m, nil
}

// loadNetworksCmd fetches networks off the UI goroutine.
func (m Model) loadNetworksCmd() tea.Cmd {
	ds := m.ds
	return func() tea.Msg {
		nets, err := ds.ListNetworks(context.Background())
		return networksMsg{networks: nets, err: err}
	}
}

// ---- WAN view (read-only) ----

// handleWanKey handles keys while the WAN list is showing.
func (m Model) handleWanKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.view = deviceView
	case key.Matches(msg, m.keys.Up):
		m.wanCursor = clampIndex(m.wanCursor-1, len(m.wans))
	case key.Matches(msg, m.keys.Down):
		m.wanCursor = clampIndex(m.wanCursor+1, len(m.wans))
	case key.Matches(msg, m.keys.GoTop):
		m.wanCursor = 0
	case key.Matches(msg, m.keys.GoBot):
		m.wanCursor = max(len(m.wans)-1, 0)
	case key.Matches(msg, m.keys.Reload):
		m.wansLoading, m.status, m.err = true, "", nil
		return m, m.loadWansCmd()
	case key.Matches(msg, m.keys.Enter):
		if m.wanCursor >= 0 && m.wanCursor < len(m.wans) {
			w := m.wans[m.wanCursor]
			return m.openDetail("wan "+w.Name, m.wanFields(w))
		}
	}
	return m, nil
}

// loadWansCmd fetches the internet uplinks off the UI goroutine.
func (m Model) loadWansCmd() tea.Cmd {
	ds := m.ds
	return func() tea.Msg {
		wans, err := ds.ListWANs(context.Background())
		return wansMsg{wans: wans, err: err}
	}
}

// ---- data-usage view (read-only summary) ----

// handleDataKey handles keys while the data-usage summary is showing.
func (m Model) handleDataKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.view = deviceView
	case key.Matches(msg, m.keys.Reload):
		m.dataLoading, m.status, m.err = true, "", nil
		return m, m.loadDataCmd()
	}
	return m, nil
}

// loadDataCmd fetches the data-usage report and resolves WAN uuid→name from the
// network list (best effort), off the UI goroutine.
func (m Model) loadDataCmd() tea.Cmd {
	ds := m.ds
	return func() tea.Msg {
		report, err := ds.DataUsage(context.Background())
		if err != nil {
			return dataMsg{err: err}
		}
		names := map[string]string{}
		if nets, nerr := ds.ListNetworks(context.Background()); nerr == nil {
			for _, n := range nets {
				if n.UUID != "" {
					names[n.UUID] = n.Name
				}
			}
		}
		return dataMsg{report: report, names: names}
	}
}

// setDevices sorts (online first, then most-recently-active) and reindexes.
func (m *Model) setDevices(devs []firewalla.Device) {
	now := m.now()
	sort.SliceStable(devs, func(i, j int) bool {
		oi, oj := devs[i].SeenWithin(onlineWindow, now), devs[j].SeenWithin(onlineWindow, now)
		if oi != oj {
			return oi // online first
		}
		return devs[i].LastActive.After(devs[j].LastActive)
	})
	m.devices = devs
	m.refilter()
}

// refilter recomputes visible indices from the current search query and clamps
// the cursor.
func (m *Model) refilter() {
	q := strings.ToLower(strings.TrimSpace(m.search.Value()))
	now := m.now()
	m.visible = m.visible[:0]
	for i, d := range m.devices {
		if m.onlineOnly && !d.SeenWithin(onlineWindow, now) {
			continue
		}
		if q == "" || matchesQuery(d, q) {
			m.visible = append(m.visible, i)
		}
	}
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) moveCursor(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// SelectedDevice returns the device under the cursor, or ok=false when the
// (filtered) list is empty.
func (m Model) SelectedDevice() (firewalla.Device, bool) {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return firewalla.Device{}, false
	}
	return m.devices[m.visible[m.cursor]], true
}

// matchesQuery reports whether a device matches a lowercased query substring
// across its name, IP, and MAC.
func matchesQuery(d firewalla.Device, q string) bool {
	return strings.Contains(strings.ToLower(d.Name), q) ||
		strings.Contains(strings.ToLower(d.IP), q) ||
		strings.Contains(strings.ToLower(d.MAC), q)
}

// deviceLabel is the friendly display name, falling back to IP then MAC.
func deviceLabel(d firewalla.Device) string {
	if d.Name != "" {
		return d.Name
	}
	if d.IP != "" {
		return d.IP
	}
	return d.MAC
}
