package picker

import (
	"testing"
	"unicode/utf8"
)

// FuzzRank checks that ranking never panics and that every reported match
// references a real item with in-bounds, unique, ascending match positions.
func FuzzRank(f *testing.F) {
	seeds := []struct{ items, query string }{
		{"phone\nlaptop\nhot tub", "pho"},
		{"AA:BB:CC:DD:EE:01\n192.0.2.10", "bb"},
		{"", ""},
		{"a\nb\nc", "z"},
		{"日本語\nemoji😀test", "本"},
	}
	for _, s := range seeds {
		f.Add(s.items, s.query)
	}
	f.Fuzz(func(t *testing.T, itemsBlob, query string) {
		items := splitNonEmpty(itemsBlob)
		matches := Rank(items, query)

		if len(matches) > len(items) {
			t.Fatalf("more matches (%d) than items (%d)", len(matches), len(items))
		}
		for _, m := range matches {
			if m.Index < 0 || m.Index >= len(items) {
				t.Fatalf("match index %d out of range [0,%d)", m.Index, len(items))
			}
			runeLen := utf8.RuneCountInString(items[m.Index])
			prev := -1
			for _, p := range m.Positions {
				if p < 0 || p >= runeLen {
					t.Fatalf("position %d out of range for %q (len %d)", p, items[m.Index], runeLen)
				}
				if p <= prev {
					t.Fatalf("positions not strictly ascending: %v", m.Positions)
				}
				prev = p
			}
		}

		// An empty query matches everything, in original order.
		if query == "" && len(matches) != len(items) {
			t.Fatalf("empty query matched %d of %d items", len(matches), len(items))
		}

		// Ranking is deterministic: a second run yields identical results.
		again := Rank(items, query)
		if len(again) != len(matches) {
			t.Fatalf("non-deterministic match count: %d vs %d", len(again), len(matches))
		}
		for i := range matches {
			if again[i].Index != matches[i].Index || again[i].Score != matches[i].Score {
				t.Fatalf("non-deterministic ranking at %d", i)
			}
		}
	})
}

// FuzzScoreMatch checks the scorer never panics and reports positions that are
// valid rune offsets into the text whenever it claims a match.
func FuzzScoreMatch(f *testing.F) {
	f.Add("phone", "pho")
	f.Add("192.0.2.10", "210")
	f.Add("", "")
	f.Add("日本語", "語")
	f.Fuzz(func(t *testing.T, text, query string) {
		score, pos, ok := scoreMatch(text, query)
		if !ok {
			return
		}
		runeLen := utf8.RuneCountInString(text)
		for _, p := range pos {
			if p < 0 || p >= runeLen {
				t.Fatalf("match position %d out of range for %q", p, text)
			}
		}
		if query != "" && len(pos) == 0 {
			t.Fatalf("matched non-empty query %q with no positions", query)
		}
		_ = score
	})
}

// splitNonEmpty splits a blob into lines, dropping empties — mirrors how the
// picker is fed a clean item list.
func splitNonEmpty(blob string) []string {
	var out []string
	start := 0
	for i := 0; i < len(blob); i++ {
		if blob[i] == '\n' {
			if line := blob[start:i]; line != "" {
				out = append(out, line)
			}
			start = i + 1
		}
	}
	if line := blob[start:]; line != "" {
		out = append(out, line)
	}
	return out
}
