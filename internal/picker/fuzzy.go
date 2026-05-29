// Package picker provides a small interactive fuzzy finder for choosing among
// a list of strings (device names, etc.), plus the pure fuzzy-matching core it
// ranks with. The matcher is I/O-free and unit-tested; the Bubble Tea UI is a
// thin shell over it.
package picker

import (
	"sort"
	"strings"
)

// Match is one ranked item: its index in the original slice, its score, and
// the rune positions in the item that matched the query (for highlighting).
type Match struct {
	Index     int
	Score     int
	Positions []int
}

// Rank returns the items that fuzzy-match query, best score first (ties broken
// by original order). An empty query matches everything in original order.
func Rank(items []string, query string) []Match {
	if query == "" {
		out := make([]Match, len(items))
		for i := range items {
			out[i] = Match{Index: i, Score: 0}
		}
		return out
	}
	var out []Match
	for i, it := range items {
		if score, pos, ok := scoreMatch(it, query); ok {
			out = append(out, Match{Index: i, Score: score, Positions: pos})
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Score != out[b].Score {
			return out[a].Score > out[b].Score
		}
		return out[a].Index < out[b].Index
	})
	return out
}

// scoreMatch does case-insensitive subsequence matching of query against text,
// rewarding consecutive matches and matches at word boundaries. It returns the
// score, the matched rune positions, and whether every query rune matched.
func scoreMatch(text, query string) (int, []int, bool) {
	tr := []rune(text)
	qr := []rune(strings.ToLower(query))
	lower := []rune(strings.ToLower(text))

	score := 0
	positions := make([]int, 0, len(qr))
	qi := 0
	prevMatch := -2
	for ti := 0; ti < len(tr) && qi < len(qr); ti++ {
		if lower[ti] != qr[qi] {
			continue
		}
		positions = append(positions, ti)
		score += 1
		if ti == prevMatch+1 {
			score += 5 // consecutive run
		}
		if ti == 0 || isBoundary(tr[ti-1]) {
			score += 10 // start of a word
		}
		prevMatch = ti
		qi++
	}
	if qi != len(qr) {
		return 0, nil, false
	}
	// Prefer shorter matches (less slack) and earlier first hit.
	score -= len(tr) / 10
	if len(positions) > 0 {
		score -= positions[0] / 4
	}
	return score, positions, true
}

func isBoundary(r rune) bool {
	switch r {
	case ' ', '-', '_', '.', '/', ':', '\t':
		return true
	}
	return false
}
