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
}

// viewMode selects which list the dashboard shows.
type viewMode int

const (
	deviceView viewMode = iota
	ruleView
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

// actionMsg carries the result of a confirmed mutation.
type actionMsg struct {
	text string
	err  error
}

// detailMsg carries the loaded traffic for a device's detail pane.
type detailMsg struct {
	mac   string
	peers []firewalla.Peer
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

	devices []firewalla.Device // sorted: online first, then most-recent
	visible []int              // indices into devices after the active filter
	cursor  int                // index into visible

	rules        []firewalla.Rule
	ruleCursor   int
	rulesLoading bool

	search    textinput.Model
	searching bool

	showHelp bool
	loading  bool
	status   string         // transient feedback (e.g. "blocked Phone")
	pending  *pendingAction // a mutation awaiting y/n confirmation
	detail   *detailState   // open device detail pane, or nil
	err      error
}

// detailState backs the per-device detail pane (its top traffic peers).
type detailState struct {
	device  firewalla.Device
	peers   []firewalla.Peer
	loading bool
	err     error
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
	return Model{
		ds:      ds,
		now:     now,
		keys:    DefaultKeyMap(),
		styles:  DefaultStyles(),
		width:   80,
		height:  24,
		search:  ti,
		loading: true,
	}
}

// Init kicks off the first device load.
func (m Model) Init() tea.Cmd { return m.loadCmd() }

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

	case devicesMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.setDevices(msg.devices)
		}
		return m, nil

	case rulesMsg:
		m.rulesLoading = false
		m.err = msg.err
		if msg.err == nil {
			m.rules = msg.rules
			m.ruleCursor = clampIndex(m.ruleCursor, len(m.rules))
		}
		return m, nil

	case actionMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.status = msg.text
		}
		// Refresh whichever list is showing so it reflects the change.
		if m.view == ruleView {
			m.rulesLoading = true
			return m, m.loadRulesCmd()
		}
		m.loading = true
		return m, m.loadCmd()

	case detailMsg:
		if m.detail != nil && m.detail.device.MAC == msg.mac {
			m.detail.loading = false
			m.detail.peers, m.detail.err = msg.peers, msg.err
		}
		return m, nil

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

	// Detail pane: esc/q/enter closes; block/unblock still work on its device.
	if m.detail != nil {
		switch {
		case key.Matches(msg, m.keys.Cancel), key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Enter):
			m.detail = nil
		case key.Matches(msg, m.keys.Block):
			m.stageAction(true)
		case key.Matches(msg, m.keys.Unblock):
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

	if m.view == ruleView {
		return m.handleRuleKey(msg)
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
	case key.Matches(msg, m.keys.Rules):
		m.view = ruleView
		m.rulesLoading, m.status, m.err = true, "", nil
		return m, m.loadRulesCmd()
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
			m.detail = &detailState{device: d, loading: true}
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

// detailCmd loads the selected device's traffic peers off the UI goroutine.
func (m Model) detailCmd(d firewalla.Device) tea.Cmd {
	ds, mac := m.ds, d.MAC
	return func() tea.Msg {
		peers, err := ds.Traffic(context.Background(), mac)
		return detailMsg{mac: mac, peers: peers, err: err}
	}
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
	case key.Matches(msg, m.keys.Cancel), key.Matches(msg, m.keys.Rules):
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
	case key.Matches(msg, m.keys.RuleEnable):
		m.stageRule("enable")
	case key.Matches(msg, m.keys.RuleDisable):
		m.stageRule("disable")
	case key.Matches(msg, m.keys.RuleDelete):
		m.stageRule("delete")
	}
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
	m.visible = m.visible[:0]
	for i, d := range m.devices {
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
