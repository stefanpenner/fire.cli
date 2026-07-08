# TLA+ specs

Design-first models of fire's only genuinely stateful / concurrent parts. The
rest of the CLI is stateless request→parse→render and is covered by fuzzing +
unit tests, not model checking.

Run with the `tlc` CLI:

```bash
tlc --parse Spec.tla        # SANY parse
tlc Spec.tla                # check vs Spec.cfg
tlc -c Spec_bug.cfg Spec.tla   # mutation config (must FAIL)
```

## BlockLifecycle

**For:** confirm fire's device-access design is coherent under the timed-expiry
race. `block`/`pause`/`unblock`/`resume` create/remove block rules; the **box**
runs the auto-expiry timer for `pause` / `block --for`, not fire.

**Question:** can a stale expiry timer (from an already-resumed pause) sever a
*fresh* block created afterwards?

**Result (MaxId=3):**
- Correct config (`Buggy=FALSE`, per-policy expiry): `TypeOK`, `AllAllocated`,
  `NoSever` all hold — 108 states, depth 7.
- Mutation config (`Buggy=TRUE`, naive "expiry removes any timed rule"):
  `NoSever` **violated** — witness: timer for rule 2 fires and severs rule 3.

**Conclusion:** fire's remove-all `resume` combined with the box's per-pid
expiry is safe. The naive alternative would early-unblock a device. This
documents the box contract fire depends on.

## TuiLoad

**For:** prove the TUI never displays a stale async-load result when the user
switches views / reloads while a load is in flight.

**Result (Views={dev,rules}, MaxSeq=3):**
- Fixed config (`Guarded=TRUE`, apply only if still on the result's view):
  `Coherent` holds — 310 states, depth 8.
- Mutation config (`Guarded=FALSE`, original unconditional apply): `Coherent`
  **violated** — witness: after switching to `rules`, a stale `dev` load lands,
  clears the spinner, and shows dev data on the rules view.

**Conclusion:** justified the view-match guard in `internal/tui/model.go` — a
list response (devicesMsg, rulesMsg, …) is applied only if `m.view` still
matches that result's view; otherwise it is dropped as stale.

## Assumptions

Correctness properties (`NoSever`, `Coherent`) are stated here as the author's
model of intended behavior. `Coherent` maps directly to observable UI behavior.
`NoSever` encodes the box's per-policy expiry contract. Bounds are small by
design (most concurrency bugs appear at scope 2–3); green TLC is a strong bug
hunt at these bounds, not a proof at all scales.
