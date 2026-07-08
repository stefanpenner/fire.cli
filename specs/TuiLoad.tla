------------------------------ MODULE TuiLoad ------------------------------
(***************************************************************************)
(* Purpose: decide the guard the TUI needs so it never displays a stale     *)
(* async-load result, INCLUDING under live auto-refresh.                    *)
(*                                                                          *)
(* The dashboard fetches data off the UI goroutine. The user can switch     *)
(* views, press r, or enable live auto-refresh (a periodic reload of the    *)
(* current view). Each of those issues a load; responses may arrive         *)
(* out of order (that is the concurrency). When a response arrives the code  *)
(* must decide whether to apply it.                                         *)
(*                                                                          *)
(* Guard selects the policy under test:                                     *)
(*   "none"      apply every response            (original naive design)    *)
(*   "viewmatch" apply if it is for the current view (the first fix)        *)
(*   "seq"       apply if it is for the current view AND is the latest       *)
(*               generation issued for that view                            *)
(*                                                                          *)
(* Result: "viewmatch" is enough for CROSS-view staleness but NOT for two    *)
(* reloads of the SAME view arriving out of order -- exactly what live       *)
(* auto-refresh produces. "seq" holds. This is why the code carries a        *)
(* per-view load generation, mirrored by internal/tui/loadsm.               *)
(*                                                                          *)
(* Atomicity grain: issuing a load and delivering one response are each one  *)
(* atomic step. gen[v] is the latest generation handed out for view v.       *)
(***************************************************************************)
EXTENDS Naturals

CONSTANTS
    Views,   \* dashboard views, e.g. {"dev", "rules"}
    MaxGen,  \* bound on reloads per view (bounds the run)
    Guard    \* "none" | "viewmatch" | "seq"

None == "none-view"

VARIABLES
    view,     \* the view the user is on
    gen,      \* [Views -> Nat]: latest load generation issued per view
    inflight, \* set of in-flight loads: [vw, g]
    shownVw,  \* view whose data is displayed (None before the first load)
    shownGen, \* generation of the displayed data
    loading   \* is the current view awaiting a (newer) load

vars == <<view, gen, inflight, shownVw, shownGen, loading>>

Load == [vw: Views, g: 0..MaxGen]

TypeOK ==
    /\ view \in Views
    /\ gen \in [Views -> 0..MaxGen]
    /\ inflight \subseteq Load
    /\ shownVw \in (Views \union {None})
    /\ shownGen \in 0..MaxGen
    /\ loading \in BOOLEAN

Init ==
    /\ view \in Views
    /\ gen = [v \in Views |-> 0]
    /\ inflight = {[vw |-> view, g |-> 0]}
    /\ shownVw = None
    /\ shownGen = 0
    /\ loading = TRUE

\* Switch to view v: issue a load at v's CURRENT generation (a plain view change
\* does not supersede an in-flight reload; only Reload bumps the generation).
Switch(v) ==
    /\ view' = v
    /\ inflight' = inflight \union {[vw |-> v, g |-> gen[v]]}
    /\ loading' = TRUE
    /\ UNCHANGED <<gen, shownVw, shownGen>>

\* Reload the current view: the r key, a post-mutation refresh, or an
\* auto-refresh tick. Repeated reloads are what create same-view races.
Reload ==
    /\ gen[view] < MaxGen
    /\ gen' = [gen EXCEPT ![view] = @ + 1]
    /\ inflight' = inflight \union {[vw |-> view, g |-> gen[view] + 1]}
    /\ loading' = TRUE
    /\ UNCHANGED <<view, shownVw, shownGen>>

\* A pending load completes. Any in-flight load may be chosen, modeling
\* out-of-order arrival. apply mirrors the code's guard.
Deliver(r) ==
    /\ r \in inflight
    /\ inflight' = inflight \ {r}
    /\ LET apply == CASE Guard = "none"      -> TRUE
                      [] Guard = "viewmatch" -> r.vw = view
                      [] Guard = "seq"       -> r.vw = view /\ r.g = gen[r.vw]
       IN IF apply
            THEN /\ shownVw' = r.vw
                 /\ shownGen' = r.g
                 /\ loading' = FALSE
            ELSE UNCHANGED <<shownVw, shownGen, loading>>
    /\ UNCHANGED <<view, gen>>

Next ==
    \/ \E v \in Views : Switch(v)
    \/ Reload
    \/ \E r \in inflight : Deliver(r)

Spec == Init /\ [][Next]_vars

\* When the UI is not showing a spinner, the data on screen is the LATEST
\* load issued for the CURRENT view. Both cross-view and same-view (reorder)
\* staleness violate this.
Coherent == (~loading) => (shownVw = view /\ shownGen = gen[view])

=============================================================================
