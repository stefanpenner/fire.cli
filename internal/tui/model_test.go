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
	devices   []firewalla.Device
	listErr   error
	createErr error
	deleteErr error
	gotSpec   firewalla.RuleSpec
	createCnt int
	deleteCnt int
}

func (f *fakeDS) Host() string { return "pi@test" }
func (f *fakeDS) ListDevices(context.Context) ([]firewalla.Device, error) {
	return f.devices, f.listErr
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

func TestBlock_IssuesMacRule(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds) // cursor on Old Laptop (online, most-recently-active → first)

	nm, cmd := m.Update(runeKey("b"))
	m = nm.(Model)
	require.NotNil(t, cmd)
	msg := cmd() // run the action
	am, ok := msg.(actionMsg)
	require.True(t, ok)
	require.NoError(t, am.err)

	assert.Equal(t, 1, ds.createCnt)
	assert.Equal(t, "block", ds.gotSpec.Action)
	assert.Equal(t, "mac", ds.gotSpec.Type)
	assert.Equal(t, "AA:BB:CC:DD:EE:03", ds.gotSpec.Target)
	assert.Contains(t, am.text, "blocked Old Laptop")
}

func TestUnblock_DeletesMatching(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	_, cmd := m.Update(runeKey("u"))
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

func TestBlock_EmptyListNoCrash(t *testing.T) {
	ds := &fakeDS{}
	m := loaded(ds)
	_, cmd := m.Update(runeKey("b"))
	assert.Nil(t, cmd, "block with no selectable device is a no-op")
}
