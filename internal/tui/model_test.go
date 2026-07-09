package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDS is a test DataSource recording block/unblock calls.
type fakeDS struct {
	devices     []firewalla.Device
	peers       []firewalla.Peer
	rules       []firewalla.Rule
	alarms      []firewalla.Alarm
	networks    []firewalla.Network
	wans        []firewalla.WAN
	data        firewalla.DataUsageReport
	topTalkers  []firewalla.TopTalker
	topErr      error
	listErr     error
	trafficErr  error
	createErr   error
	deleteErr   error
	rulesErr    error
	alarmsErr   error
	networksErr error
	wansErr     error
	dataErr     error
	gotSpec     firewalla.RuleSpec
	gotMAC      string
	gotRuleID   string
	gotDisable  bool
	gotDelete   string
	gotAlarmID  string
	gotAlarmOp  string
	gotLimit    int
	createCnt   int
	deleteCnt   int
}

func (f *fakeDS) Host() string { return "pi@test" }
func (f *fakeDS) ListDevices(context.Context) ([]firewalla.Device, error) {
	return f.devices, f.listErr
}
func (f *fakeDS) Traffic(_ context.Context, mac string) ([]firewalla.Peer, error) {
	f.gotMAC = mac
	return f.peers, f.trafficErr
}
func (f *fakeDS) ListRules(context.Context) ([]firewalla.Rule, error) {
	return f.rules, f.rulesErr
}
func (f *fakeDS) SetRuleDisabled(_ context.Context, id string, disabled bool) error {
	f.gotRuleID, f.gotDisable = id, disabled
	return nil
}
func (f *fakeDS) DeleteRule(_ context.Context, id string) error {
	f.gotDelete = id
	return nil
}
func (f *fakeDS) ListAlarms(_ context.Context, limit int) ([]firewalla.Alarm, error) {
	f.gotLimit = limit
	return f.alarms, f.alarmsErr
}
func (f *fakeDS) ArchiveAlarm(_ context.Context, id string) error {
	f.gotAlarmID, f.gotAlarmOp = id, "archive"
	return nil
}
func (f *fakeDS) DeleteAlarm(_ context.Context, id string) error {
	f.gotAlarmID, f.gotAlarmOp = id, "delete"
	return nil
}
func (f *fakeDS) ListNetworks(context.Context) ([]firewalla.Network, error) {
	return f.networks, f.networksErr
}
func (f *fakeDS) ListWANs(context.Context) ([]firewalla.WAN, error) {
	return f.wans, f.wansErr
}
func (f *fakeDS) DataUsage(context.Context) (firewalla.DataUsageReport, error) {
	return f.data, f.dataErr
}
func (f *fakeDS) TopTalkers(context.Context) ([]firewalla.TopTalker, error) {
	return f.topTalkers, f.topErr
}
func (f *fakeDS) CreateRule(_ context.Context, spec firewalla.RuleSpec) (string, error) {
	f.gotSpec, f.createCnt = spec, f.createCnt+1
	return "999", f.createErr
}
func (f *fakeDS) DeleteMatching(_ context.Context, spec firewalla.RuleSpec) (int, error) {
	f.gotSpec, f.deleteCnt = spec, f.deleteCnt+1
	return 1, f.deleteErr
}

var fixedNow = func() time.Time { return time.Unix(1700000100, 0) }

func sampleDevices() []firewalla.Device {
	return []firewalla.Device{
		{MAC: "AA:BB:CC:DD:EE:01", Name: "Example Phone", IP: "192.0.2.10", LastActive: time.Unix(1700000090, 0)},
		{MAC: "AA:BB:CC:DD:EE:02", Name: "Example Hot Tub", IP: "192.0.2.20", LastActive: time.Unix(1600000000, 0)},
		{MAC: "AA:BB:CC:DD:EE:03", Name: "Old Laptop", IP: "192.0.2.30", LastActive: time.Unix(1700000095, 0)},
	}
}

// loaded builds a model with devices already delivered (the common test setup).
func loaded(ds *fakeDS) Model {
	m := NewModel(ds, fixedNow)
	m.width, m.height = 100, 30
	nm, _ := m.Update(devicesMsg{devices: ds.devices})
	return nm.(Model)
}

func runeKey(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// firstMsg runs a command and returns its message. switchTo batches the load
// with the spinner tick; the load is always first, so this unwraps a batch to
// the load's message.
func firstMsg(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		if len(batch) == 0 || batch[0] == nil {
			return nil
		}
		return firstMsg(batch[0]) // the load is always the batch's first cmd
	}
	return msg
}

func TestNewModel_Defaults(t *testing.T) {
	m := NewModel(&fakeDS{}, nil)
	assert.Equal(t, 80, m.width)
	assert.Equal(t, 24, m.height)
	assert.True(t, m.loading)
	assert.NotNil(t, m.now)
}

func TestView_RendersDevices(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	v := loaded(ds).View()
	assert.Contains(t, v, "fire")
	assert.Contains(t, v, "pi@test")
	assert.Contains(t, v, "Example Phone")
	assert.Contains(t, v, "192.0.2.20")
	assert.Contains(t, v, "3 devices")
	assert.Contains(t, v, "2 online") // phone + laptop within 5m of fixedNow
}

func TestView_TinyDimensionsNoPanic(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	for _, dim := range [][2]int{{1, 1}, {0, 0}, {5, 3}, {200, 1}} {
		m.width, m.height = dim[0], dim[1]
		assert.NotPanics(t, func() { _ = m.View() }, "dims %v", dim)
	}
}

func TestView_LoadingAndError(t *testing.T) {
	m := NewModel(&fakeDS{}, fixedNow)
	assert.Contains(t, m.View(), "loading")

	nm, _ := m.Update(devicesMsg{err: errors.New("ssh blew up")})
	assert.Contains(t, nm.(Model).View(), "ssh blew up")
}

func TestNavigation(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	require.Equal(t, 0, m.cursor)

	nm, _ := m.Update(runeKey("j"))
	m = nm.(Model)
	assert.Equal(t, 1, m.cursor)

	nm, _ = m.Update(runeKey("k"))
	m = nm.(Model)
	assert.Equal(t, 0, m.cursor)

	// k at the top stays at 0.
	nm, _ = m.Update(runeKey("k"))
	assert.Equal(t, 0, nm.(Model).cursor)

	// G jumps to the last visible row; g back to top.
	nm, _ = m.Update(runeKey("G"))
	m = nm.(Model)
	assert.Equal(t, 2, m.cursor)
	nm, _ = m.Update(runeKey("g"))
	assert.Equal(t, 0, nm.(Model).cursor)
}

func TestSearch_FiltersAndSelects(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)

	// Enter search mode and type "tub".
	nm, _ := m.Update(runeKey("/"))
	m = nm.(Model)
	assert.True(t, m.searching)
	for _, r := range "tub" {
		nm, _ = m.Update(runeKey(string(r)))
		m = nm.(Model)
	}
	require.Len(t, m.visible, 1)
	d, ok := m.SelectedDevice()
	require.True(t, ok)
	assert.Equal(t, "Example Hot Tub", d.Name)

	// Enter accepts and exits search mode, keeping the filter.
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	assert.False(t, m.searching)
	assert.Len(t, m.visible, 1)

	// Re-open and Esc clears the filter back to all devices.
	nm, _ = m.Update(runeKey("/"))
	m = nm.(Model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(Model)
	assert.False(t, m.searching)
	assert.Len(t, m.visible, 3)
}

func TestBlock_ConfirmIssuesMacRule(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds) // cursor on Old Laptop (online, most-recently-active → first)

	// b stages the action but must NOT mutate yet.
	nm, cmd := m.Update(runeKey("b"))
	m = nm.(Model)
	assert.Nil(t, cmd, "block should not fire before confirmation")
	require.NotNil(t, m.pending)
	assert.Contains(t, m.View(), "Block Old Laptop?")
	assert.Equal(t, 0, ds.createCnt)

	// y confirms and fires the rule.
	nm, cmd = m.Update(runeKey("y"))
	m = nm.(Model)
	require.NotNil(t, cmd)
	assert.Nil(t, m.pending)
	am, ok := cmd().(actionMsg)
	require.True(t, ok)
	require.NoError(t, am.err)
	assert.Equal(t, 1, ds.createCnt)
	assert.Equal(t, "block", ds.gotSpec.Action)
	assert.Equal(t, "mac", ds.gotSpec.Type)
	assert.Equal(t, "AA:BB:CC:DD:EE:03", ds.gotSpec.Target)
	assert.Contains(t, am.text, "blocked Old Laptop")
}

func TestBlock_CancelDoesNotMutate(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	nm, _ := m.Update(runeKey("b"))
	m = nm.(Model)
	require.NotNil(t, m.pending)

	nm, cmd := m.Update(runeKey("n"))
	m = nm.(Model)
	assert.Nil(t, cmd)
	assert.Nil(t, m.pending, "n cancels the staged action")
	assert.Equal(t, 0, ds.createCnt, "cancel must not mutate")
}

func TestUnblock_ConfirmDeletesMatching(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	nm, _ := m.Update(runeKey("u"))
	m = nm.(Model)
	assert.Contains(t, m.View(), "Unblock Old Laptop?")

	_, cmd := m.Update(runeKey("y"))
	require.NotNil(t, cmd)
	am := cmd().(actionMsg)
	require.NoError(t, am.err)
	assert.Equal(t, 1, ds.deleteCnt)
	assert.Equal(t, "AA:BB:CC:DD:EE:03", ds.gotSpec.Target)
	assert.Contains(t, am.text, "unblocked")
}

func TestActionMsg_TriggersReload(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	nm, cmd := m.Update(actionMsg{text: "blocked X"})
	m = nm.(Model)
	assert.Equal(t, "blocked X", m.status)
	require.NotNil(t, cmd, "action result should trigger a reload")
	_, ok := cmd().(devicesMsg)
	assert.True(t, ok)
}

func TestHelpModalToggle(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	nm, _ := m.Update(runeKey("?"))
	m = nm.(Model)
	assert.True(t, m.showHelp)
	assert.Contains(t, m.View(), "Keyboard Shortcuts")
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, nm.(Model).showHelp)
}

func TestQuit(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	_, cmd := m.Update(runeKey("q"))
	require.NotNil(t, cmd)
	assert.Equal(t, tea.Quit(), cmd())
}

func TestDetail_OpensAndLoadsTraffic(t *testing.T) {
	ds := &fakeDS{
		devices: sampleDevices(),
		peers: []firewalla.Peer{
			{Label: "video.example.com", Kind: "internet", Download: 409600, Upload: 2048},
			{PeerMAC: "AA:BB:CC:DD:EE:02", Kind: "device", Download: 1024},
		},
		rules: []firewalla.Rule{
			{ID: "77", Action: "block", Type: "mac", Target: "aa:bb:cc:dd:ee:03"}, // targets Old Laptop (case-insensitive)
			{ID: "88", Action: "block", Type: "dns", Target: "ads.example.net"},   // unrelated
		},
	}
	m := loaded(ds)

	// Enter opens the detail pane for the selected device and kicks off a load.
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	require.NotNil(t, m.detail)
	assert.True(t, m.detail.loading)
	require.NotNil(t, cmd)

	// The detail load targets the selected device's MAC.
	msg := firstMsg(cmd)
	dm, ok := msg.(detailMsg)
	require.True(t, ok)
	assert.Equal(t, "AA:BB:CC:DD:EE:03", ds.gotMAC)

	nm, _ = m.Update(dm)
	m = nm.(Model)
	assert.False(t, m.detail.loading)

	// Only the rule targeting this device's MAC is kept.
	require.Len(t, m.detail.rules, 1)
	assert.Equal(t, "77", m.detail.rules[0].ID)

	v := m.View()
	assert.Contains(t, v, "Old Laptop")        // header
	assert.Contains(t, v, "video.example.com") // top peer
	assert.Contains(t, v, "Top traffic")
	assert.Contains(t, v, "Rules") // rules section
	assert.Contains(t, v, "77")    // the targeting rule
}

func TestDetail_EscCloses(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	require.NotNil(t, m.detail)

	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, nm.(Model).detail)
}

func TestDetail_BlockFromPaneStagesConfirm(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	nm, cmd := m.Update(runeKey("b"))
	m = nm.(Model)
	assert.Nil(t, cmd, "block from the detail pane still confirms first")
	require.NotNil(t, m.pending)
	assert.Equal(t, "Block Old Laptop?", m.pending.prompt)
	assert.Contains(t, m.View(), "Block Old Laptop?", "confirm bar shows in the detail pane")
}

func sampleRules() []firewalla.Rule {
	return []firewalla.Rule{
		{ID: "10", Action: "block", Type: "dns", Target: "ads.example.net", Disabled: false},
		{ID: "11", Action: "allow", Type: "mac", Target: "AA:BB:CC:DD:EE:01", Disabled: true},
	}
}

// loadedRules opens the rules view with rules already delivered.
func loadedRules(ds *fakeDS) Model {
	m := loaded(ds)
	nm, _ := m.Update(runeKey("R")) // enter rules view (kicks off a load)
	m = nm.(Model)
	nm, _ = m.Update(rulesMsg{rules: ds.rules})
	return nm.(Model)
}

func TestRulesView_OpensAndLists(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), rules: sampleRules()}
	m := loaded(ds)

	nm, cmd := m.Update(runeKey("R"))
	m = nm.(Model)
	assert.Equal(t, ruleView, m.view)
	require.NotNil(t, cmd) // load kicked off
	rm, ok := firstMsg(cmd).(rulesMsg)
	require.True(t, ok)

	nm, _ = m.Update(rm)
	m = nm.(Model)
	v := m.View()
	assert.Contains(t, v, "rules 2") // count is shown in the active tab
	assert.Contains(t, v, "ads.example.net")
	assert.Contains(t, v, "on")  // rule 10 enabled
	assert.Contains(t, v, "off") // rule 11 disabled
}

func TestRulesView_EscReturnsToDevices(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), rules: sampleRules()}
	m := loadedRules(ds)
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(Model)
	assert.Equal(t, deviceView, m.view)
	assert.Contains(t, m.View(), "Example Phone") // back to devices
}

func TestRulesView_DisableConfirms(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), rules: sampleRules()}
	m := loadedRules(ds) // cursor on rule 10

	nm, cmd := m.Update(runeKey("d"))
	m = nm.(Model)
	assert.Nil(t, cmd, "must confirm before mutating")
	require.NotNil(t, m.pending)
	assert.Equal(t, "Disable rule 10?", m.pending.prompt)

	nm, cmd = m.Update(runeKey("y"))
	m = nm.(Model)
	require.NotNil(t, cmd)
	am, ok := cmd().(actionMsg)
	require.True(t, ok)
	require.NoError(t, am.err)
	assert.Equal(t, "10", ds.gotRuleID)
	assert.True(t, ds.gotDisable)
	assert.Contains(t, am.text, "disabled rule 10")
}

func TestRulesView_EnableAndDelete(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), rules: sampleRules()}
	m := loadedRules(ds)

	// enable rule 10
	nm, _ := m.Update(runeKey("e"))
	m = nm.(Model)
	require.NotNil(t, m.pending)
	assert.Equal(t, "Enable rule 10?", m.pending.prompt)
	nm, cmd := m.Update(runeKey("y"))
	m = nm.(Model)
	cmd()
	assert.Equal(t, "10", ds.gotRuleID)
	assert.False(t, ds.gotDisable)

	// move to rule 11, delete it
	nm, _ = m.Update(runeKey("j"))
	m = nm.(Model)
	nm, _ = m.Update(runeKey("x"))
	m = nm.(Model)
	require.NotNil(t, m.pending)
	assert.Equal(t, "Delete rule 11?", m.pending.prompt)
	nm, cmd = m.Update(runeKey("y"))
	m = nm.(Model)
	cmd()
	assert.Equal(t, "11", ds.gotDelete)
}

func TestRulesView_ActionReloadsRules(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), rules: sampleRules()}
	m := loadedRules(ds)
	nm, cmd := m.Update(actionMsg{text: "disabled rule 10"})
	m = nm.(Model)
	require.NotNil(t, cmd)
	_, ok := cmd().(rulesMsg)
	assert.True(t, ok, "an action in the rules view reloads rules, not devices")
}

func sampleAlarms() []firewalla.Alarm {
	return []firewalla.Alarm{
		{ID: "2297", Type: "Port Scan", Device: "Laptop", Message: "Laptop scanned ports", Time: time.Unix(1700000050, 0)},
		{ID: "55", Type: "New Device", Device: "Phone", Time: time.Unix(1700000000, 0)},
	}
}

func loadedAlarms(ds *fakeDS) Model {
	m := loaded(ds)
	nm, _ := m.Update(runeKey("A"))
	m = nm.(Model)
	nm, _ = m.Update(alarmsMsg{alarms: ds.alarms})
	return nm.(Model)
}

func TestAlarmsView_OpensAndLists(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), alarms: sampleAlarms()}
	m := loaded(ds)

	nm, cmd := m.Update(runeKey("A"))
	m = nm.(Model)
	assert.Equal(t, alarmView, m.view)
	require.NotNil(t, cmd)
	am, ok := firstMsg(cmd).(alarmsMsg)
	require.True(t, ok)
	assert.Equal(t, alarmViewLimit, ds.gotLimit)

	nm, _ = m.Update(am)
	m = nm.(Model)
	v := m.View()
	assert.Contains(t, v, "alarms 2")
	assert.Contains(t, v, "Port Scan")
	assert.Contains(t, v, "2297")
}

func TestAlarmsView_EscReturnsToDevices(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), alarms: sampleAlarms()}
	m := loadedAlarms(ds)
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(Model)
	assert.Equal(t, deviceView, m.view)
	assert.Contains(t, m.View(), "Example Phone")
}

func TestAlarmsView_ArchiveConfirms(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), alarms: sampleAlarms()}
	m := loadedAlarms(ds) // cursor on alarm 2297

	nm, cmd := m.Update(runeKey("a"))
	m = nm.(Model)
	assert.Nil(t, cmd, "archive must confirm first")
	require.NotNil(t, m.pending)
	assert.Equal(t, "Archive alarm 2297?", m.pending.prompt)

	nm, cmd = m.Update(runeKey("y"))
	m = nm.(Model)
	require.NotNil(t, cmd)
	am := cmd().(actionMsg)
	require.NoError(t, am.err)
	assert.Equal(t, "2297", ds.gotAlarmID)
	assert.Equal(t, "archive", ds.gotAlarmOp)
	assert.Contains(t, am.text, "archived alarm 2297")
}

func TestAlarmsView_DeleteConfirms(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), alarms: sampleAlarms()}
	m := loadedAlarms(ds)
	nm, _ := m.Update(runeKey("x"))
	m = nm.(Model)
	require.NotNil(t, m.pending)
	assert.Equal(t, "Delete alarm 2297?", m.pending.prompt)
	nm, cmd := m.Update(runeKey("y"))
	m = nm.(Model)
	cmd()
	assert.Equal(t, "2297", ds.gotAlarmID)
	assert.Equal(t, "delete", ds.gotAlarmOp)
}

func TestAlarmsView_ActionReloadsAlarms(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), alarms: sampleAlarms()}
	m := loadedAlarms(ds)
	_, cmd := m.Update(actionMsg{text: "archived alarm 2297"})
	require.NotNil(t, cmd)
	_, ok := cmd().(alarmsMsg)
	assert.True(t, ok, "an action in the alarms view reloads alarms")
}

func TestNetworksView_OpensAndLists(t *testing.T) {
	ds := &fakeDS{
		devices: sampleDevices(),
		networks: []firewalla.Network{
			{Name: "Home", Type: "lan", Subnet: "192.0.2.0/24", Interface: "br0"},
			{Name: "IoT", Type: "lan", VLANID: 2001, Subnet: "192.0.2.64/26", Interface: "eth2.2001"},
		},
	}
	m := loaded(ds)

	nm, cmd := m.Update(runeKey("N"))
	m = nm.(Model)
	assert.Equal(t, networkView, m.view)
	require.NotNil(t, cmd)
	nmsg, ok := firstMsg(cmd).(networksMsg)
	require.True(t, ok)

	nm, _ = m.Update(nmsg)
	m = nm.(Model)
	v := m.View()
	assert.Contains(t, v, "networks 2")
	assert.Contains(t, v, "IoT")
	assert.Contains(t, v, "vlan 2001")
	assert.Contains(t, v, "192.0.2.64/26")
}

func TestNetworksView_EscReturnsToDevices(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), networks: []firewalla.Network{{Name: "Home", Type: "lan"}}}
	m := loaded(ds)
	nm, _ := m.Update(runeKey("N"))
	m = nm.(Model)
	nm, _ = m.Update(networksMsg{networks: ds.networks})
	m = nm.(Model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(Model)
	assert.Equal(t, deviceView, m.view)
}

// The tab bar lists every view and is present across views.
func TestTabBar_ShownAcrossViews(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	for _, label := range []string{"devices", "rules", "alarms", "networks"} {
		assert.Contains(t, m.View(), label, "device view tab bar")
	}
}

func TestWANView_OpensAndShowsHealth(t *testing.T) {
	ds := &fakeDS{
		devices: sampleDevices(),
		wans: []firewalla.WAN{
			{Name: "ISP-A", Interface: "eth0", Role: "primary", Active: true, Carrier: true, Ping: true, DNS: true},
			{Name: "ISP-B", Interface: "eth3", Role: "standby", Carrier: false},
		},
	}
	m := loaded(ds)

	nm, cmd := m.Update(runeKey("W"))
	m = nm.(Model)
	assert.Equal(t, wanView, m.view)
	require.NotNil(t, cmd)
	wm, ok := firstMsg(cmd).(wansMsg)
	require.True(t, ok)

	nm, _ = m.Update(wm)
	m = nm.(Model)
	v := m.View()
	assert.Contains(t, v, "wan 2")
	assert.Contains(t, v, "ISP-A")
	assert.Contains(t, v, "healthy")
	assert.Contains(t, v, "down") // ISP-B has no carrier
}

func TestWANView_EscReturnsToDevices(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), wans: []firewalla.WAN{{Name: "ISP-A"}}}
	m := loaded(ds)
	nm, _ := m.Update(runeKey("W"))
	m = nm.(Model)
	nm, _ = m.Update(wansMsg{wans: ds.wans})
	m = nm.(Model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(Model)
	assert.Equal(t, deviceView, m.view)
}

func TestDataView_ShowsPlanAndPerWAN(t *testing.T) {
	ds := &fakeDS{
		devices:  sampleDevices(),
		networks: []firewalla.Network{{UUID: "u-1", Name: "ISP-A", Type: "wan"}},
		data: firewalla.DataUsageReport{
			PlanTotal: 1000000000000, // 1 TB
			ResetDay:  1,
			WANs:      []firewalla.WANUsage{{UUID: "u-1", Upload: 1024, Download: 1048576}},
		},
	}
	m := loaded(ds)

	nm, cmd := m.Update(runeKey("D"))
	m = nm.(Model)
	assert.Equal(t, dataView, m.view)
	require.NotNil(t, cmd)
	dm, ok := firstMsg(cmd).(dataMsg)
	require.True(t, ok)

	nm, _ = m.Update(dm)
	m = nm.(Model)
	v := m.View()
	assert.Contains(t, v, "plan")   // data view content (the name line is gone)
	assert.Contains(t, v, "plan")   // summary line
	assert.Contains(t, v, "ISP-A")  // uuid resolved to name
	assert.Contains(t, v, "1.0 MB") // download
	assert.Contains(t, v, "resets day 1")
}

func TestDataView_EscReturnsToDevices(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	nm, _ := m.Update(runeKey("D"))
	m = nm.(Model)
	nm, _ = m.Update(dataMsg{})
	m = nm.(Model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(Model)
	assert.Equal(t, deviceView, m.view)
}

func TestPlainStyles_UsesReverseSelection(t *testing.T) {
	// Without color, the selected row relies on reverse video; the colored set
	// uses a background instead.
	assert.True(t, PlainStyles().Selected.GetReverse())
	assert.False(t, DefaultStyles().Selected.GetReverse())
	// Hierarchy survives via bold in the plain set.
	assert.True(t, PlainStyles().Title.GetBold())
}

func TestWithColor_AppliesStyleSet(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}

	plain := NewModel(ds, fixedNow).WithColor(false)
	assert.True(t, plain.styles.Selected.GetReverse(), "no-color model uses reverse selection")

	colored := NewModel(ds, fixedNow).WithColor(true)
	assert.False(t, colored.styles.Selected.GetReverse(), "colored model keeps its background selection")

	// Plain styling still renders content and the selection marker.
	nm, _ := plain.Update(devicesMsg{devices: ds.devices})
	v := nm.(Model).View()
	assert.Contains(t, v, "Old Laptop")
	assert.Contains(t, v, "❯")
}

func TestNumberKeys_JumpToViews(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)

	cases := []struct {
		key  string
		want viewMode
	}{
		{"2", ruleView},
		{"3", alarmView},
		{"4", networkView},
		{"5", wanView},
		{"6", dataView},
		{"1", deviceView},
	}
	for _, c := range cases {
		nm, cmd := m.Update(runeKey(c.key))
		m = nm.(Model)
		assert.Equal(t, c.want, m.view, "key %s", c.key)
		require.NotNil(t, cmd, "key %s should trigger a load", c.key)
	}

	// Number keys jump across views, not just from devices: from the rules
	// view, 3 goes straight to alarms.
	nm, _ := m.Update(runeKey("2"))
	m = nm.(Model)
	nm, _ = m.Update(runeKey("3"))
	m = nm.(Model)
	assert.Equal(t, alarmView, m.view)
}

func TestNumberKeys_IgnoredWhileSearching(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	nm, _ := m.Update(runeKey("/"))
	m = nm.(Model)
	nm, _ = m.Update(runeKey("2")) // typed into the filter, not a view switch
	m = nm.(Model)
	assert.Equal(t, deviceView, m.view)
	assert.Equal(t, "2", m.search.Value())
}

func TestBlock_EmptyListNoCrash(t *testing.T) {
	ds := &fakeDS{}
	m := loaded(ds)
	_, cmd := m.Update(runeKey("b"))
	assert.Nil(t, cmd, "block with no selectable device is a no-op")
}

// TestModel_DropsStaleCrossViewLoad proves the view-match guard (see
// specs/TuiLoad.tla): a device load that completes after the user switched to
// the rules view is dropped, so it cannot clobber the rules view's state.
func TestModel_DropsStaleCrossViewLoad(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds) // deviceView, devices delivered

	nm, _ := m.Update(runeKey("2")) // switch to rules view (a rules load is now in flight)
	rm := nm.(Model)
	require.Equal(t, ruleView, rm.view)

	// A stale device load (kicked off before the switch) completes with an error.
	nm2, _ := rm.Update(devicesMsg{err: errors.New("stale dev load boom")})
	sm := nm2.(Model)

	assert.Equal(t, ruleView, sm.view, "must stay on the rules view")
	assert.NoError(t, sm.err, "stale cross-view load must not set the current view's error")
}

// TestModel_AppliesCurrentViewLoad is the paired positive case: a load for the
// current view is applied normally.
func TestModel_AppliesCurrentViewLoad(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := NewModel(ds, fixedNow)
	m.width, m.height = 100, 30

	nm, _ := m.Update(devicesMsg{devices: ds.devices})
	sm := nm.(Model)
	assert.False(t, sm.loading)
	assert.Len(t, sm.devices, 3)
}
