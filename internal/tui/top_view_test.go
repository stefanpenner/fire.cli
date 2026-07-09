package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The top-talkers view ranks devices with a bar chart, resolving MAC → name.
func TestTopView_RanksWithBars(t *testing.T) {
	ds := goldenDS()
	m := loaded(ds)
	nm, _ := m.Update(runeKey("7"))
	m = nm.(Model)
	require.Equal(t, topView, m.view)

	nm, _ = m.Update(topMsg{talkers: ds.topTalkers})
	m = nm.(Model)
	v := m.View()
	assert.Contains(t, v, "top 2")         // count in the active tab
	assert.Contains(t, v, "Example Phone") // AA:...:01 resolved to its device name
	assert.Contains(t, v, "█")             // a bar was drawn
	assert.Contains(t, v, "4.0 GB")        // humanized total (down+up) for the top talker
}

// enter opens a detail pane for the selected ranked device.
func TestTopView_EnterDetail(t *testing.T) {
	ds := goldenDS()
	m := loaded(ds)
	nm, _ := m.Update(runeKey("7"))
	nm, _ = nm.(Model).Update(topMsg{talkers: ds.topTalkers})
	nm, _ = nm.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	require.NotNil(t, m.detail)
	assert.Contains(t, m.View(), "download")
}

// / filters the ranking by name/MAC.
func TestTopView_Search(t *testing.T) {
	ds := goldenDS()
	m := loaded(ds)
	nm, _ := m.Update(runeKey("7"))
	nm, _ = nm.(Model).Update(topMsg{talkers: ds.topTalkers})
	m = typeSearch(nm.(Model), "phone")
	require.Len(t, m.visible, 1)
	assert.Equal(t, "AA:BB:CC:DD:EE:01", m.topTalkers[m.visible[0]].MAC)
}
