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

**For:** decide the guard the TUI needs so it never displays a stale async-load
result — including under **live auto-refresh**, where the current view is
reloaded on a timer and responses can arrive out of order.

Models view switches, reloads (r / auto-refresh / post-mutation), and
out-of-order delivery over per-view load **generations**. A `Guard` constant
selects the policy: `none`, `viewmatch`, or `seq`.

**Result (Views={dev,rules}, MaxGen=2):**
- `seq` (apply only the current view's latest generation): `Coherent` holds —
  1344 states, depth ~14.
- `none` (apply everything): **violated**.
- `viewmatch` (apply if for the current view — the *first* fix): **violated**.
  Witness: two reloads of the same view; the older generation arrives last and
  overwrites the newer. This is the bug live auto-refresh can hit — view-match
  alone is not enough.

**Conclusion:** the code carries a per-view load generation and applies a
response only if it is the current view's latest (`internal/tui/loadsm.Apply`,
wired into `internal/tui/model.go`).

## Codegen bridge (spec ↔ code, no JVM)

`internal/tui/loadsm` is the executable mirror of `TuiLoad.tla`:
- `loadsm.Apply` is the `seq` guard the dashboard actually calls.
- `loadsm`'s test is a **BFS mini model-checker** that re-derives the spec's
  result (seq holds; viewmatch and none fail) in plain Go. It runs in CI with
  no JVM, so the guarantee can't silently drift from the spec.

The full TLA+ specs are also checked in CI by `specs/check.sh` (the `specs`
job installs a JVM); locally, run `specs/check.sh` or the `tlc` CLI directly.

## Assumptions

Correctness properties (`NoSever`, `Coherent`) are stated here as the author's
model of intended behavior. `Coherent` maps directly to observable UI behavior.
`NoSever` encodes the box's per-policy expiry contract. Bounds are small by
design (most concurrency bugs appear at scope 2–3); green TLC is a strong bug
hunt at these bounds, not a proof at all scales.
