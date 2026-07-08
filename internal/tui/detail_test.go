package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enterDetail switches to a view, delivers its data, and presses enter.
func enterDetail(t *testing.T, key string, deliver tea.Msg) Model {
	t.Helper()
	m := loaded(goldenDS())
	nm, _ := m.Update(runeKey(key))
	nm, _ = nm.(Model).Update(deliver)
	nm, _ = nm.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := nm.(Model)
	require.NotNil(t, mm.detail, "enter must open a detail pane")
	return mm
}

// enter in the rules view opens a detail pane with the rule's full fields.
func TestDetail_Rules(t *testing.T) {
	ds := goldenDS()
	m := enterDetail(t, "2", rulesMsg{rules: ds.rules})
	v := m.View()
	assert.Contains(t, v, ds.rules[0].Target, "full target shown")
	assert.Contains(t, v, "direction")
}

// enter in the alarms view shows the full (untruncated) message.
func TestDetail_Alarms(t *testing.T) {
	ds := goldenDS()
	m := enterDetail(t, "3", alarmsMsg{alarms: ds.alarms})
	assert.Contains(t, m.View(), ds.alarms[0].Message)
}

// enter in the networks view shows subnet + gateway.
func TestDetail_Networks(t *testing.T) {
	ds := goldenDS()
	m := enterDetail(t, "4", networksMsg{networks: ds.networks})
	assert.Contains(t, m.View(), "subnet")
	assert.Contains(t, m.View(), ds.networks[0].Subnet)
}

// enter in the WAN view shows the health + role.
func TestDetail_WAN(t *testing.T) {
	ds := goldenDS()
	m := enterDetail(t, "5", wansMsg{wans: ds.wans})
	assert.Contains(t, m.View(), "health")
	assert.Contains(t, m.View(), "role")
}

// esc closes a non-device detail pane and returns to the list.
func TestDetail_NonDeviceEscCloses(t *testing.T) {
	ds := goldenDS()
	m := enterDetail(t, "2", rulesMsg{rules: ds.rules})
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, nm.(Model).detail, "esc closes the detail pane")
}

// block/unblock are inert in a non-device detail (they only apply to devices).
func TestDetail_NonDeviceIgnoresBlock(t *testing.T) {
	ds := goldenDS()
	m := enterDetail(t, "2", rulesMsg{rules: ds.rules})
	nm, _ := m.Update(runeKey("b"))
	assert.Nil(t, nm.(Model).pending, "b must not stage a block from a rule detail")
}
