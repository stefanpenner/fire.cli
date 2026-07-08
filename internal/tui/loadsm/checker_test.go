package loadsm

import (
	"fmt"
	"sort"
	"testing"
)

// This is the executable bridge to specs/TuiLoad.tla: a bounded BFS
// model-checker that mirrors the spec's state machine (Switch / Reload /
// Deliver over per-view load generations, with out-of-order delivery) and
// checks the Coherent invariant under a pluggable guard. It re-derives the
// spec's result in CI with no JVM, so the guard the code ships (Apply) stays
// faithful to the model.
//
// Mirrors: Switch issues a load at the view's current generation; Reload bumps
// it; Deliver applies a chosen in-flight load iff guard() allows. Coherent:
// when not loading, the shown data is the current view's latest generation.

const (
	nViews = 2
	maxGen = 2
	none   = -1
)

type load struct{ view, gen int }

type state struct {
	view     int
	gen      [nViews]int
	inflight []load // set: sorted + deduped
	shownVw  int    // none when nothing applied yet
	shownGen int
	loading  bool
}

func (s state) key() string {
	return fmt.Sprintf("%d|%v|%v|%d|%d|%t", s.view, s.gen, s.inflight, s.shownVw, s.shownGen, s.loading)
}

// addLoad returns inflight with l added (set semantics: no duplicates).
func addLoad(in []load, l load) []load {
	for _, x := range in {
		if x == l {
			return in
		}
	}
	out := append(append([]load{}, in...), l)
	sort.Slice(out, func(i, j int) bool {
		if out[i].view != out[j].view {
			return out[i].view < out[j].view
		}
		return out[i].gen < out[j].gen
	})
	return out
}

func removeLoad(in []load, i int) []load {
	out := append([]load{}, in[:i]...)
	return append(out, in[i+1:]...)
}

// successors enumerates the next states, mirroring the spec's Next.
func successors(s state, guard func(msgView, curView, msgGen, curGen int) bool) []state {
	var out []state

	// Switch(v): issue a load at v's current generation, make v current.
	for v := 0; v < nViews; v++ {
		ns := s
		ns.view = v
		ns.inflight = addLoad(s.inflight, load{v, s.gen[v]})
		ns.loading = true
		out = append(out, ns)
	}

	// Reload: bump the current view's generation and issue a load.
	if s.gen[s.view] < maxGen {
		ns := s
		ns.gen[s.view]++
		ns.inflight = addLoad(s.inflight, load{s.view, ns.gen[s.view]})
		ns.loading = true
		out = append(out, ns)
	}

	// Deliver(r): any in-flight load may complete (out-of-order).
	for i, r := range s.inflight {
		ns := s
		ns.inflight = removeLoad(s.inflight, i)
		if guard(r.view, s.view, r.gen, s.gen[r.view]) {
			ns.shownVw, ns.shownGen, ns.loading = r.view, r.gen, false
		}
		out = append(out, ns)
	}
	return out
}

func coherent(s state) bool {
	if s.loading {
		return true
	}
	return s.shownVw == s.view && s.shownGen == s.gen[s.view]
}

// check runs BFS from every initial state and returns a counter-example path
// (nil if the guard keeps Coherent invariant over the whole reachable space).
func check(guard func(msgView, curView, msgGen, curGen int) bool) []state {
	seen := map[string]bool{}
	type node struct {
		s    state
		path []state
	}
	var queue []node
	for v := 0; v < nViews; v++ {
		init := state{view: v, inflight: []load{{v, 0}}, shownVw: none, loading: true}
		queue = append(queue, node{init, []state{init}})
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if seen[n.s.key()] {
			continue
		}
		seen[n.s.key()] = true
		if !coherent(n.s) {
			return n.path
		}
		for _, ns := range successors(n.s, guard) {
			if !seen[ns.key()] {
				queue = append(queue, node{ns, append(append([]state{}, n.path...), ns)})
			}
		}
	}
	return nil
}

// The "seq" guard the code ships (loadsm.Apply) keeps Coherent invariant —
// matching specs/TuiLoad.tla's seq config (no error).
func TestSeqGuard_Coherent(t *testing.T) {
	if cx := check(Apply); cx != nil {
		t.Fatalf("seq guard violated Coherent; counter-example:\n%v", cx)
	}
}

// view-match alone must NOT be coherent (mirrors the viewmatch mutation
// config): a same-view reload arriving out of order overwrites fresh data.
func TestViewMatchGuard_Fails(t *testing.T) {
	viewMatch := func(msgView, curView, _, _ int) bool { return msgView == curView }
	if check(viewMatch) == nil {
		t.Fatal("view-match guard unexpectedly held Coherent; the spec says it must fail")
	}
}

// Applying every response (no guard) must fail too (the none mutation config).
func TestNoGuard_Fails(t *testing.T) {
	always := func(_, _, _, _ int) bool { return true }
	if check(always) == nil {
		t.Fatal("no-guard unexpectedly held Coherent")
	}
}
