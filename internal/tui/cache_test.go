package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The first visit to a view shows the spinner; a later visit shows cached data
// instantly (no spinner) while refreshing in the background.
func TestCache_SecondVisitIsInstant(t *testing.T) {
	ds := goldenDS()
	m := loaded(ds)

	// First visit to rules: spinner on (nothing cached yet).
	nm, _ := m.Update(runeKey("2"))
	rm := nm.(Model)
	require.True(t, rm.rulesLoading, "first visit shows the spinner")

	nm, _ = rm.Update(rulesMsg{rules: ds.rules})
	rm = nm.(Model)
	require.False(t, rm.rulesLoading)
	require.Len(t, rm.rules, len(ds.rules))

	// Leave to devices and come back to rules.
	nm, _ = rm.Update(runeKey("1"))
	nm, _ = nm.(Model).Update(devicesMsg{devices: ds.devices})
	nm, cmd := nm.(Model).Update(runeKey("2"))
	rm = nm.(Model)

	assert.False(t, rm.rulesLoading, "cached revisit shows no spinner")
	assert.Len(t, rm.rules, len(ds.rules), "cached data is shown immediately")
	assert.NotNil(t, cmd, "a background refresh still runs")
}

// The spinner stops ticking once nothing is loading, so an idle dashboard does
// not redraw on every frame.
func TestCache_SpinnerStopsWhenIdle(t *testing.T) {
	ds := goldenDS()
	m := loaded(ds) // devices delivered → idle

	assert.False(t, m.anyLoading())
	_, cmd := m.Update(m.spinner.Tick())
	assert.Nil(t, cmd, "no re-tick while idle")
}

// While a load is in flight the spinner keeps ticking.
func TestCache_SpinnerTicksWhileLoading(t *testing.T) {
	m := NewModel(&fakeDS{}, fixedNow) // loading=true, nothing delivered
	require.True(t, m.anyLoading())
	_, cmd := m.Update(m.spinner.Tick())
	assert.NotNil(t, cmd, "spinner keeps ticking while loading")
}
