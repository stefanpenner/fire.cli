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
	devices    []firewalla.Device
	peers      []firewalla.Peer
	rules      []firewalla.Rule
	listErr    error
	trafficErr error
	createErr  error
	deleteErr  error
	rulesErr   error
	gotSpec    firewalla.RuleSpec
	gotMAC     string
	gotRuleID  string
	gotDisable bool
	gotDelete  string
	createCnt  int
	deleteCnt  int
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
	}
	m := loaded(ds)

	// Enter opens the detail pane for the selected device and kicks off a load.
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	require.NotNil(t, m.detail)
	assert.True(t, m.detail.loading)
	require.NotNil(t, cmd)

	// The detail load targets the selected device's MAC.
	msg := cmd()
	dm, ok := msg.(detailMsg)
	require.True(t, ok)
	assert.Equal(t, "AA:BB:CC:DD:EE:03", ds.gotMAC)

	nm, _ = m.Update(dm)
	m = nm.(Model)
	assert.False(t, m.detail.loading)

	v := m.View()
	assert.Contains(t, v, "Old Laptop")        // header
	assert.Contains(t, v, "video.example.com") // top peer
	assert.Contains(t, v, "Top traffic")
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
	rm, ok := cmd().(rulesMsg)
	require.True(t, ok)

	nm, _ = m.Update(rm)
	m = nm.(Model)
	v := m.View()
	assert.Contains(t, v, "rules (2)")
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

func TestBlock_EmptyListNoCrash(t *testing.T) {
	ds := &fakeDS{}
	m := loaded(ds)
	_, cmd := m.Update(runeKey("b"))
	assert.Nil(t, cmd, "block with no selectable device is a no-op")
}
