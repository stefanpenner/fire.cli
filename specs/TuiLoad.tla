------------------------------ MODULE TuiLoad ------------------------------
(***************************************************************************)
(* Purpose: prove the TUI never displays a stale load result.              *)
(*                                                                          *)
(* The dashboard fetches data off the UI goroutine (loadCmd, loadRulesCmd,  *)
(* …). Switching views or reloading fires a NEW async load while an older   *)
(* one may still be in flight. When a response arrives the code applies it. *)
(* If it applies UNCONDITIONALLY (the original design), a slow load from a  *)
(* view the user already left can clear the spinner and show data that does *)
(* not belong to the current view.                                          *)
(*                                                                          *)
(* Guarded models the fix: a list response is applied only if the model is  *)
(* still on that result's view (the msg type identifies the view). A load    *)
(* for a view the user already left is dropped. Unguarded (Guarded = FALSE)  *)
(* is the original design -- apply every response -- and is expected to      *)
(* violate Coherent (the mutation/bait check, TuiLoad_bug.cfg).             *)
(*                                                                          *)
(* Atomicity grain: issuing a load, and delivering one response, are each   *)
(* one atomic step. Two views suffice to expose the cross-view race.        *)
(***************************************************************************)
EXTENDS Naturals

CONSTANTS
    Views,    \* the set of dashboard views, e.g. {"dev", "rules"}
    MaxSeq,   \* bound on how many loads may be issued (bounds the run)
    Guarded   \* TRUE applies the seq guard (the fix); FALSE is the old design

None == "none"

VARIABLES
    view,     \* the view the user is currently on
    reqs,     \* in-flight loads: set of [seq, vw]
    nextSeq,  \* next seq to hand out
    curSeq,   \* seq of the most recent load kicked off (the model's loadSeq)
    shownVw,  \* view whose data is currently displayed (None before first load)
    loading   \* is the current view awaiting a load result

vars == <<view, reqs, nextSeq, curSeq, shownVw, loading>>

Req == [seq: 0..MaxSeq, vw: Views]

TypeOK ==
    /\ view \in Views
    /\ reqs \subseteq Req
    /\ nextSeq \in 0..(MaxSeq + 1)
    /\ curSeq \in 0..MaxSeq
    /\ shownVw \in (Views \union {None})
    /\ loading \in BOOLEAN

\* Init fires the first device load (seq 0), matching Model.Init/loadCmd.
Init ==
    /\ view \in Views
    /\ reqs = {[seq |-> 0, vw |-> view]}
    /\ nextSeq = 1
    /\ curSeq = 0
    /\ shownVw = None
    /\ loading = TRUE

\* Switch to view v (v = view models a plain reload). Kicks off a fresh load
\* and makes it the current one.
Switch(v) ==
    /\ nextSeq <= MaxSeq
    /\ view' = v
    /\ reqs' = reqs \union {[seq |-> nextSeq, vw |-> v]}
    /\ curSeq' = nextSeq
    /\ nextSeq' = nextSeq + 1
    /\ loading' = TRUE
    /\ UNCHANGED shownVw

\* A pending load completes. apply mirrors the code: with the guard, apply only
\* when the model is still on this result's view; without it, always apply.
Deliver(r) ==
    /\ r \in reqs
    /\ reqs' = reqs \ {r}
    /\ LET apply == IF Guarded THEN r.vw = view ELSE TRUE
       IN IF apply
            THEN /\ shownVw' = r.vw
                 /\ loading' = FALSE
            ELSE UNCHANGED <<shownVw, loading>>
    /\ UNCHANGED <<view, nextSeq, curSeq>>

Next ==
    \/ \E v \in Views : Switch(v)
    \/ \E r \in reqs : Deliver(r)

Spec == Init /\ [][Next]_vars

\* --- property ---

\* When the UI is not showing a spinner, the data on screen belongs to the
\* current view. Unguarded delivery of a stale cross-view load breaks this.
Coherent == (~loading) => (shownVw = view)

=============================================================================
