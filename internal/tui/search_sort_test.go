package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// typeSearch enters search mode and types q into the active view.
func typeSearch(m Model, q string) Model {
	nm, _ := m.Update(runeKey("/"))
	m = nm.(Model)
	for _, r := range q {
		nm, _ = m.Update(runeKey(string(r)))
		m = nm.(Model)
	}
	return m
}

// / filters the rules view to matching rows.
func TestSearch_FiltersRules(t *testing.T) {
	ds := goldenDS()
	m := loaded(ds)
	nm, _ := m.Update(runeKey("2"))
	nm, _ = nm.(Model).Update(rulesMsg{rules: ds.rules})
	m = nm.(Model)
	require.Greater(t, len(m.visible), 1)

	m = typeSearch(m, ds.rules[0].Target[:3])
	// At least one match, and every visible rule contains the query.
	require.NotEmpty(t, m.visible)
	for _, idx := range m.visible {
		assert.Contains(t, m.rules[idx].Target, ds.rules[0].Target[:3])
	}
}

// / filters devices too (unchanged behavior, via the shared path).
func TestSearch_FiltersDevices(t *testing.T) {
	ds := goldenDS()
	m := loaded(ds)
	m = typeSearch(m, "hot tub")
	require.Len(t, m.visible, 1)
	d := m.devices[m.visible[0]]
	assert.Contains(t, d.Name, "Hot Tub")
}

// s cycles the sort mode and reorders visible by label.
func TestSort_ByLabel(t *testing.T) {
	ds := goldenDS()
	m := loaded(ds)
	nm, _ := m.Update(runeKey("4")) // networks
	nm, _ = nm.(Model).Update(networksMsg{networks: ds.networks})
	m = nm.(Model)

	// Default order: as loaded (Home, IoT).
	require.Equal(t, "Home", m.networks[m.visible[0]].Name)

	// Press s → sort by name; "Home" < "IoT" so order is stable here, but the
	// mode must flip and remain coherent.
	nm, _ = m.Update(runeKey("s"))
	m = nm.(Model)
	assert.Equal(t, sortByLabel, m.sortMode[networkView])
	names := []string{m.networks[m.visible[0]].Name, m.networks[m.visible[1]].Name}
	assert.Equal(t, []string{"Home", "IoT"}, names)

	// Cycles back to natural after numSortModes presses.
	nm, _ = m.Update(runeKey("s"))
	m = nm.(Model)
	assert.Equal(t, sortNatural, m.sortMode[networkView])
}

// Switching views clears the search query so each view starts unfiltered.
func TestSearch_ClearedOnViewSwitch(t *testing.T) {
	ds := goldenDS()
	m := loaded(ds)
	m = typeSearch(m, "zzz-no-match")
	assert.Empty(t, m.visible)

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // leave search
	nm, _ = nm.(Model).Update(runeKey("2"))         // switch to rules
	nm, _ = nm.(Model).Update(rulesMsg{rules: ds.rules})
	m = nm.(Model)
	assert.False(t, m.searching)
	assert.Equal(t, "", m.search.Value())
	assert.Len(t, m.visible, len(ds.rules), "rules view is unfiltered after switch")
}
