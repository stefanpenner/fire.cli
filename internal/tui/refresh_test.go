package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 'f' toggles live auto-refresh on and off.
func TestAutoRefresh_Toggle(t *testing.T) {
	m := loaded(&fakeDS{devices: sampleDevices()})
	require.False(t, m.autoRefresh)

	nm, cmd := m.Update(runeKey("f"))
	m = nm.(Model)
	assert.True(t, m.autoRefresh, "f turns auto-refresh on")
	assert.NotNil(t, cmd, "and starts the refresh timer")
	assert.Contains(t, m.View(), "live")

	nm, _ = m.Update(runeKey("f"))
	m = nm.(Model)
	assert.False(t, m.autoRefresh, "f again turns it off")
}

// A refresh tick reloads the current view and reschedules while enabled.
func TestAutoRefresh_TickReloadsAndReschedules(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds).WithRefresh(2 * time.Second) // WithRefresh enables live mode

	nm, cmd := m.Update(refreshTickMsg{})
	m = nm.(Model)
	require.NotNil(t, cmd, "tick reschedules + reloads")
	// the reschedule + reload batch includes a devices reload
	_, ok := firstMsg(cmd).(devicesMsg)
	assert.True(t, ok, "tick triggers a reload of the current view")
}

// A tick after auto-refresh is off is a no-op.
func TestAutoRefresh_TickIgnoredWhenOff(t *testing.T) {
	m := loaded(&fakeDS{devices: sampleDevices()})
	_, cmd := m.Update(refreshTickMsg{})
	assert.Nil(t, cmd, "no reschedule when auto-refresh is off")
}
