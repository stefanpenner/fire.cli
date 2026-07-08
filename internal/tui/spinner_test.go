package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// While the device list is loading, the animated spinner frame is shown next to
// "loading…".
func TestSpinner_ShownWhileLoading(t *testing.T) {
	m := NewModel(&fakeDS{}, fixedNow)
	m.width, m.height = 80, 24
	v := m.View()
	assert.Contains(t, v, "loading")
	assert.Contains(t, v, m.spinner.View(), "the spinner frame appears while loading")
}

// A sub-view (rules) shows the spinner while its own load is in flight.
func TestSpinner_ShownInSubViewLoading(t *testing.T) {
	m := loaded(&fakeDS{devices: sampleDevices()})
	nm, _ := m.Update(runeKey("2")) // rules view -> rulesLoading, no rules yet
	rm := nm.(Model)
	require.Equal(t, ruleView, rm.view)
	assert.Contains(t, rm.View(), rm.spinner.View())
}

// Init starts the spinner ticking (batched with the first load).
func TestSpinner_InitReturnsCommand(t *testing.T) {
	m := NewModel(&fakeDS{}, fixedNow)
	assert.NotNil(t, m.Init())
}

// A matching TickMsg advances the spinner and schedules the next tick, so the
// animation keeps running.
func TestSpinner_TickKeepsAnimating(t *testing.T) {
	m := NewModel(&fakeDS{}, fixedNow)
	m.width, m.height = 80, 24
	// Drive one real tick produced by the spinner itself so the ID matches.
	tick := m.spinner.Tick()
	nm, cmd := m.Update(tick)
	assert.NotNil(t, cmd, "spinner reschedules its next tick")
	_ = nm.(Model)
}
