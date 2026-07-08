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

// TestSeqGuard_DropsStaleSameViewReload proves the fix for the bug
// specs/TuiLoad.tla found: two reloads of the same view (as live auto-refresh
// produces) arriving out of order must not let the older one win.
func TestSeqGuard_DropsStaleSameViewReload(t *testing.T) {
	ds := &fakeDS{devices: sampleDevices()}
	m := loaded(ds) // devices delivered at gen 0

	nm, _ := m.Update(runeKey("r")) // reload → gen 1
	m = nm.(Model)
	nm, _ = m.Update(runeKey("r")) // reload → gen 2
	m = nm.(Model)
	require.Equal(t, 2, m.loadGen[deviceView])

	// A stale gen-1 response arrives late: it must be ignored.
	nm, _ = m.Update(devicesMsg{devices: sampleDevices()[:1], gen: 1})
	m = nm.(Model)
	assert.Len(t, m.devices, 3, "stale gen-1 reload dropped")

	// The current gen-2 response applies.
	nm, _ = m.Update(devicesMsg{devices: sampleDevices()[:1], gen: 2})
	m = nm.(Model)
	assert.Len(t, m.devices, 1, "current gen-2 reload applied")
}
