package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func viewOf(t *testing.T, m tea.Model) viewMode {
	t.Helper()
	mm, ok := m.(Model)
	require.True(t, ok)
	return mm.view
}

// Tab / →/ l cycle forward through the six views and wrap.
func TestNav_NextTabCyclesAndWraps(t *testing.T) {
	m := loaded(&fakeDS{devices: sampleDevices()})
	order := []viewMode{ruleView, alarmView, networkView, wanView, dataView, topView, deviceView}
	cur := tea.Model(m)
	for _, want := range order {
		nm, _ := cur.(Model).Update(tea.KeyMsg{Type: tea.KeyTab})
		assert.Equal(t, want, viewOf(t, nm))
		cur = nm
	}
}

// Shift-Tab / ← / h cycle backward and wrap.
func TestNav_PrevTabWraps(t *testing.T) {
	m := loaded(&fakeDS{devices: sampleDevices()})
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	assert.Equal(t, topView, viewOf(t, nm), "prev from devices wraps to the last tab")

	nm2, _ := nm.(Model).Update(runeKey("h"))
	assert.Equal(t, dataView, viewOf(t, nm2), "h goes prev")
}

// l / right also go forward.
func TestNav_LetterAndArrowForward(t *testing.T) {
	m := loaded(&fakeDS{devices: sampleDevices()})
	nm, _ := m.Update(runeKey("l"))
	assert.Equal(t, ruleView, viewOf(t, nm))

	nm2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, ruleView, viewOf(t, nm2))
}

// Tab navigation works from inside a sub-view, not just devices.
func TestNav_TabWorksFromSubView(t *testing.T) {
	m := loaded(&fakeDS{devices: sampleDevices()})
	nm, _ := m.Update(runeKey("2")) // rules view
	require.Equal(t, ruleView, viewOf(t, nm))

	nm2, _ := nm.(Model).Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, alarmView, viewOf(t, nm2), "tab advances from rules to alarms")
}

// A capital letter jumps directly to its view from anywhere (consistent, no
// toggle-back).
func TestNav_LetterJumpFromSubView(t *testing.T) {
	m := loaded(&fakeDS{devices: sampleDevices()})
	nm, _ := m.Update(runeKey("2")) // rules
	nm2, _ := nm.(Model).Update(runeKey("W"))
	assert.Equal(t, wanView, viewOf(t, nm2), "W jumps to wan from rules view")
}
