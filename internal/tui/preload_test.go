package tui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// After the first device load, the other tabs are prefetched; their responses
// are cached off-view so switching to them is instant (no spinner).
func TestPreload_CachesOtherTabs(t *testing.T) {
	ds := goldenDS()
	m := NewModel(ds, fixedNow)
	m.width, m.height = 80, 24

	nm, cmd := m.Update(devicesMsg{devices: ds.devices, gen: 0})
	m = nm.(Model)
	require.True(t, m.prefetched, "device load triggers a one-time prefetch")
	require.NotNil(t, cmd, "prefetch issues background loads")

	// A prefetched rules load arrives while still on the devices view.
	nm, _ = m.Update(rulesMsg{rules: ds.rules, gen: 0})
	m = nm.(Model)
	assert.True(t, m.loaded[ruleView], "off-view rules load is cached")
	assert.Equal(t, deviceView, m.view, "still on devices")
	assert.NoError(t, m.err, "off-view load must not touch the current view's error")

	// Switching to rules now shows the cached data with no spinner.
	nm, _ = m.Update(runeKey("2"))
	m = nm.(Model)
	assert.False(t, m.rulesLoading, "cached → instant switch, no spinner")
	assert.Len(t, m.rules, len(ds.rules))
}

// An off-view error must not clobber the current view.
func TestPreload_OffViewErrorIgnored(t *testing.T) {
	ds := goldenDS()
	m := NewModel(ds, fixedNow)
	m.width, m.height = 80, 24
	nm, _ := m.Update(devicesMsg{devices: ds.devices, gen: 0})
	m = nm.(Model)

	nm, _ = m.Update(topMsg{err: errors.New("top boom"), gen: 0})
	m = nm.(Model)
	assert.NoError(t, m.err, "an off-view (prefetch) error does not surface on devices")
	assert.False(t, m.loaded[topView], "a failed load is not marked loaded")
}
