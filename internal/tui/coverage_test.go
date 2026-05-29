package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_ReturnsLoad(t *testing.T) {
	m := NewModel(&fakeDS{devices: sampleDevices()}, fixedNow)
	cmd := m.Init()
	require.NotNil(t, cmd)
	_, ok := cmd().(devicesMsg)
	assert.True(t, ok)
}

func TestDeviceLabel_Fallbacks(t *testing.T) {
	assert.Equal(t, "Phone", deviceLabel(firewalla.Device{Name: "Phone", IP: "192.0.2.1", MAC: "AA:BB:CC:DD:EE:01"}))
	assert.Equal(t, "192.0.2.1", deviceLabel(firewalla.Device{IP: "192.0.2.1", MAC: "AA:BB:CC:DD:EE:01"}))
	assert.Equal(t, "AA:BB:CC:DD:EE:01", deviceLabel(firewalla.Device{MAC: "AA:BB:CC:DD:EE:01"}))
}

func TestLastSeen_Ranges(t *testing.T) {
	now := time.Unix(1700000000, 0)
	assert.Equal(t, "never", lastSeen(time.Time{}, now))
	assert.Equal(t, "30s ago", lastSeen(now.Add(-30*time.Second), now))
	assert.Equal(t, "5m ago", lastSeen(now.Add(-5*time.Minute), now))
	assert.Equal(t, "3h ago", lastSeen(now.Add(-3*time.Hour), now))
	assert.Equal(t, "2d ago", lastSeen(now.Add(-50*time.Hour), now))
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "short", truncate("short", 10))
	assert.Equal(t, "exactlyten", truncate("exactlyten", 10))
	assert.Equal(t, "truncat…", truncate("truncated!!", 8))
}

func TestWANHealth_AllLabels(t *testing.T) {
	ds := &fakeDS{
		devices: sampleDevices(),
		wans: []firewalla.WAN{
			{Name: "A", Carrier: true, Ping: true, DNS: true},  // healthy
			{Name: "B", Carrier: true, Ping: true, DNS: false}, // degraded
			{Name: "C", Carrier: true},                         // no connectivity
			{Name: "D", Carrier: false},                        // down
		},
	}
	m := loaded(ds)
	nm, _ := m.Update(runeKey("W"))
	m = nm.(Model)
	nm, _ = m.Update(wansMsg{wans: ds.wans})
	v := nm.(Model).View()
	for _, want := range []string{"healthy", "degraded", "no connectivity", "down"} {
		assert.Contains(t, v, want)
	}
}

func TestSearchFooter_Rendered(t *testing.T) {
	m := loaded(&fakeDS{devices: sampleDevices()})
	nm, _ := m.Update(runeKey("/"))
	assert.Contains(t, nm.(Model).View(), "type to filter")
}

func TestFilter_NoMatch(t *testing.T) {
	m := loaded(&fakeDS{devices: sampleDevices()})
	nm, _ := m.Update(runeKey("/"))
	m = nm.(Model)
	for _, r := range "zzzznope" {
		nm, _ = m.Update(runeKey(string(r)))
		m = nm.(Model)
	}
	assert.Contains(t, m.View(), "no devices match")
}

// navExercise drives a list view through down/bottom/top/up/reload and asserts
// the reload returns a command, covering each handler's nav branches.
func navExercise(t *testing.T, m Model, cursorOf func(Model) int) {
	t.Helper()
	nm, _ := m.Update(runeKey("j"))
	m = nm.(Model)
	assert.Equal(t, 1, cursorOf(m), "down")
	nm, _ = m.Update(runeKey("G"))
	m = nm.(Model)
	assert.Equal(t, 1, cursorOf(m), "bottom (2 items)")
	nm, _ = m.Update(runeKey("g"))
	m = nm.(Model)
	assert.Equal(t, 0, cursorOf(m), "top")
	nm, _ = m.Update(runeKey("k"))
	m = nm.(Model)
	assert.Equal(t, 0, cursorOf(m), "up clamps at 0")
	_, cmd := m.Update(runeKey("r"))
	assert.NotNil(t, cmd, "reload returns a command")
}

func TestRulesView_Nav(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), rules: sampleRules()}
	navExercise(t, loadedRules(ds), func(m Model) int { return m.ruleCursor })
}

func TestAlarmsView_Nav(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), alarms: sampleAlarms()}
	navExercise(t, loadedAlarms(ds), func(m Model) int { return m.alarmCursor })
}

func TestNetworksView_Nav(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), networks: []firewalla.Network{
		{Name: "Home", Type: "lan"}, {Name: "IoT", Type: "lan", VLANID: 2001},
	}}
	m := loaded(ds)
	nm, _ := m.Update(runeKey("N"))
	m = nm.(Model)
	nm, _ = m.Update(networksMsg{networks: ds.networks})
	navExercise(t, nm.(Model), func(m Model) int { return m.networkCursor })
}

func TestWANView_Nav(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices(), wans: []firewalla.WAN{
		{Name: "ISP-A"}, {Name: "ISP-B"},
	}}
	m := loaded(ds)
	nm, _ := m.Update(runeKey("W"))
	m = nm.(Model)
	nm, _ = m.Update(wansMsg{wans: ds.wans})
	navExercise(t, nm.(Model), func(m Model) int { return m.wanCursor })
}

func TestDataView_Reload(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)
	nm, _ := m.Update(runeKey("D"))
	m = nm.(Model)
	nm, _ = m.Update(dataMsg{})
	m = nm.(Model)
	_, cmd := m.Update(runeKey("r"))
	assert.NotNil(t, cmd, "data view reload returns a command")
}

// Acting on an empty list view stages nothing (selectedRule/Alarm return false).
func TestEmptyListViews_NoStage(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}

	m := loaded(ds)
	nm, _ := m.Update(runeKey("2")) // rules view, no rules
	m = nm.(Model)
	nm, _ = m.Update(rulesMsg{})
	m = nm.(Model)
	nm, _ = m.Update(runeKey("d")) // disable — nothing selected
	assert.Nil(t, nm.(Model).pending)

	nm, _ = m.Update(runeKey("3")) // alarms view, no alarms
	m = nm.(Model)
	nm, _ = m.Update(alarmsMsg{})
	m = nm.(Model)
	nm, _ = m.Update(runeKey("a")) // archive — nothing selected
	assert.Nil(t, nm.(Model).pending)
}

// Error states render an error line rather than a list.
func TestViews_ErrorRendering(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds)

	nm, _ := m.Update(runeKey("2"))
	m = nm.(Model)
	nm, _ = m.Update(rulesMsg{err: assertErr("rules boom")})
	assert.Contains(t, nm.(Model).View(), "rules boom")

	nm2, _ := loaded(ds).Update(runeKey("5"))
	nm2, _ = nm2.(Model).Update(wansMsg{err: assertErr("wan boom")})
	assert.Contains(t, nm2.(Model).View(), "wan boom")
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

// A tea.WindowSizeMsg resizes the model.
func TestOnlineOnlyFilter(t *testing.T) {
	// sampleDevices: phone + laptop online (within 5m of fixedNow), hot tub offline.
	m := loaded(&fakeDS{devices: sampleDevices()})
	require.Len(t, m.visible, 3)

	nm, _ := m.Update(runeKey("o"))
	m = nm.(Model)
	assert.True(t, m.onlineOnly)
	assert.Len(t, m.visible, 2, "offline hot tub hidden")
	assert.Contains(t, m.View(), "online only")

	// Composes with search: filter to "example" keeps only the online "Example Phone".
	nm, _ = m.Update(runeKey("/"))
	m = nm.(Model)
	for _, r := range "example" {
		nm, _ = m.Update(runeKey(string(r)))
		m = nm.(Model)
	}
	require.Len(t, m.visible, 1)
	d, _ := m.SelectedDevice()
	assert.Equal(t, "Example Phone", d.Name)

	// Toggling off (need to leave search first) restores all devices.
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // exit search, clears query
	m = nm.(Model)
	nm, _ = m.Update(runeKey("o")) // online-only off
	m = nm.(Model)
	assert.False(t, m.onlineOnly)
	assert.Len(t, m.visible, 3)
}

func TestWindowSize(t *testing.T) {
	m := loaded(&fakeDS{devices: sampleDevices()})
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got := nm.(Model)
	assert.Equal(t, 120, got.width)
	assert.Equal(t, 40, got.height)
}
