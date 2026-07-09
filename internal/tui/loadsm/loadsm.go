// Package loadsm is the executable mirror of specs/TuiLoad.tla: the single
// decision of whether an async load response should be applied to the TUI.
//
// The real dashboard (internal/tui) calls Apply so its behavior is exactly the
// "seq" guard the spec proves correct, and loadsm's test re-checks that guard
// with a BFS mini model-checker (no JVM) so the guarantee is enforced in CI and
// cannot drift from the spec.
package loadsm

// Apply reports whether a load response for view msgView at generation msgGen
// should be applied, given the current view and the latest generation issued
// for that view. It is the "seq" guard from specs/TuiLoad.tla: apply only the
// current view's most-recent load. This alone defeats both cross-view
// staleness and same-view out-of-order arrival (which live auto-refresh
// produces); view-match alone does not (see the spec's viewmatch mutation).
func Apply(msgView, curView, msgGen, curGen int) bool {
	return msgView == curView && msgGen == curGen
}

// Fresh reports whether a response is its view's latest generation (not
// superseded), regardless of which view is showing. The TUI uses it to decide
// whether to *cache* a load — including one prefetched for a tab the user is
// not on yet, so switching to it is instant. Caching an off-view load does not
// change what is displayed, so it is neutral to the Coherent property that
// Apply (the display guard) upholds; Apply == (msgView == curView) && Fresh.
func Fresh(msgGen, curGen int) bool {
	return msgGen == curGen
}
