-------------------------- MODULE BlockLifecycle --------------------------
(***************************************************************************)
(* Purpose: confirm fire's device-access design is coherent under the      *)
(* timed-expiry race.                                                       *)
(*                                                                          *)
(* fire controls one device's internet access via block rules on the box:  *)
(*   block           -> create a permanent block rule                       *)
(*   pause / block --for D -> create a *timed* block rule; the BOX          *)
(*                     auto-removes it after D (fire does not run the timer)*)
(*   unblock / resume -> DeleteMatching: remove ALL block rules for the mac *)
(*                                                                          *)
(* fire delegates auto-expiry to the box. This spec models that split and  *)
(* asks: can a stale expiry timer (from an already-resumed pause) sever a   *)
(* FRESH block/pause created afterwards? The box scopes expiry to a policy  *)
(* pid, so firing a dead pid is a no-op -- modeled as ExpireCorrect.        *)
(* The naive alternative (expiry removes "any timed rule") is ExpireBuggy,  *)
(* selected by the Buggy constant, and is expected to violate NoSever.      *)
(*                                                                          *)
(* Atomicity grain: each fire operation and each box timer firing is one    *)
(* atomic step. Rules are abstract unique ids; a single device suffices     *)
(* (devices are independent, so this is a symmetry reduction).              *)
(***************************************************************************)
EXTENDS Naturals

CONSTANTS
    MaxId,   \* bound on allocated rule ids (bounds the run)
    Buggy    \* TRUE selects the naive expiry semantics (mutation check)

VARIABLES
    active,       \* set of rule ids currently enforcing a block
    timedIds,     \* ids created as timed (pause / block --for)
    timers,       \* pending box auto-expiries (subset of timedIds)
    nextId,       \* next fresh id to allocate
    everSevered   \* history: an expiry removed a rule that was not its own

vars == <<active, timedIds, timers, nextId, everSevered>>

TypeOK ==
    /\ active \subseteq 1..MaxId
    /\ timedIds \subseteq 1..MaxId
    /\ timers \subseteq timedIds
    /\ nextId \in 1..(MaxId + 1)
    /\ everSevered \in BOOLEAN

Init ==
    /\ active = {}
    /\ timedIds = {}
    /\ timers = {}
    /\ nextId = 1
    /\ everSevered = FALSE

\* block: permanent block rule.
Block ==
    /\ nextId <= MaxId
    /\ active' = active \union {nextId}
    /\ nextId' = nextId + 1
    /\ UNCHANGED <<timedIds, timers, everSevered>>

\* pause / block --for D: timed rule; box will auto-expire it.
Pause ==
    /\ nextId <= MaxId
    /\ active' = active \union {nextId}
    /\ timedIds' = timedIds \union {nextId}
    /\ timers' = timers \union {nextId}
    /\ nextId' = nextId + 1
    /\ UNCHANGED everSevered

\* unblock / resume: DeleteMatching removes ALL block rules.
\* Stale timers survive on the box and may still fire later (as no-ops).
Resume ==
    /\ active' = {}
    /\ UNCHANGED <<timedIds, timers, nextId, everSevered>>

\* Correct (real) box behavior: expiry is scoped to its own policy id.
\* Firing a timer for an id no longer active is a no-op.
ExpireCorrect(id) ==
    /\ timers' = timers \ {id}
    /\ active' = active \ {id}
    /\ UNCHANGED <<timedIds, nextId, everSevered>>

\* Naive design: expiry removes an arbitrary timed rule that happens to be
\* active. This can sever a rule created AFTER the timer was scheduled.
ExpireBuggy(id) ==
    /\ timers' = timers \ {id}
    /\ IF (active \intersect timedIds) = {}
         THEN /\ active' = active
              /\ everSevered' = everSevered
         ELSE \E victim \in (active \intersect timedIds) :
                /\ active' = active \ {victim}
                /\ everSevered' = (everSevered \/ (victim # id))
    /\ UNCHANGED <<timedIds, nextId>>

Expire(id) == IF Buggy THEN ExpireBuggy(id) ELSE ExpireCorrect(id)

Next ==
    \/ Block
    \/ Pause
    \/ Resume
    \/ \E id \in timers : Expire(id)

Spec == Init /\ [][Next]_vars

\* --- properties ---

\* Every enforcing rule was actually allocated.
AllAllocated == \A x \in active : x < nextId

\* An expiry never removes a rule other than the one it was scheduled for.
\* Holds under ExpireCorrect; ExpireBuggy is expected to violate it, which
\* is the mutation/bait check (run with BlockLifecycle_bug.cfg).
NoSever == everSevered = FALSE

=============================================================================
