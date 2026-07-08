package picker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func indices(ms []Match) []int {
	out := make([]int, len(ms))
	for i, m := range ms {
		out[i] = m.Index
	}
	return out
}

func TestRank_EmptyQueryKeepsOrder(t *testing.T) {
	items := []string{"a", "b", "c"}
	assert.Equal(t, []int{0, 1, 2}, indices(Rank(items, "")))
}

func TestRank_SubsequenceMatch(t *testing.T) {
	items := []string{"Sonos Roam", "iPhone", "Govee", "Brenda-Surface"}
	// "phone" matches iPhone only.
	got := Rank(items, "phone")
	require.Len(t, got, 1)
	assert.Equal(t, 1, got[0].Index)
}

func TestRank_AbbreviationAcrossWords(t *testing.T) {
	items := []string{"Living Room TV", "Laptop", "Bedroom Light"}
	// "lrt" should hit "Living Room TV" via word-initial letters.
	got := Rank(items, "lrt")
	require.NotEmpty(t, got)
	assert.Equal(t, 0, got[0].Index)
}

func TestRank_WordStartOutranksMidword(t *testing.T) {
	items := []string{"xnest", "Nest"} // both contain "nest"
	got := Rank(items, "nest")
	require.Len(t, got, 2)
	assert.Equal(t, 1, got[0].Index, "word-start match should rank first")
}

func TestRank_NoMatchExcluded(t *testing.T) {
	assert.Empty(t, Rank([]string{"abc", "def"}, "xyz"))
}

func TestScoreMatch_Positions(t *testing.T) {
	_, pos, ok := scoreMatch("iPhone", "phone")
	require.True(t, ok)
	assert.Equal(t, []int{1, 2, 3, 4, 5}, pos)
}
