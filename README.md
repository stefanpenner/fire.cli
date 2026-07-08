# fire

A fast, well-tested command-line interface for a [Firewalla](https://firewalla.com) box.

`fire` talks to your Firewalla over SSH and turns its internals (the Redis
state store, Zeek DNS logs, scripts) into clean, scriptable output — devices,
DNS activity, and a `redis-cli` escape hatch — with both human tables and
`--json`.

> Born out of a real debugging session: tracking down why a device couldn't
> reach its cloud backend, which hinged on *which* host had resolved a given
> domain. `fire dns who <domain>` is that question, as a command.

## Install

```sh
go install github.com/stefanpenner/fire.cli@latest   # installs the `fire` binary
# or from a clone:
make build && ./fire --help
```

Requires Go 1.26+ and SSH access to the box (key-based, non-interactive).

## Usage

```sh
fire                                  # launch the interactive dashboard (alias: fire tui)
fire --host pi@fire.walla status      # is the box reachable? how many devices?
fire devices                          # list devices (online/offline, last seen)
fire devices --json | jq '.[].name'   # machine-readable
fire dns who broker.example.com       # which clients resolved this domain?
fire dns device "Living Room TV"      # recent DNS lookups by a device (name, IP, or MAC)
fire networks                         # networks & VLANs (alias: vlans)
fire wan                              # internet uplinks: dual-WAN role + live health
fire data                             # data-plan usage this period, per WAN (alias: usage)
fire rules                            # firewall block/allow rules (--all for disabled)
fire alarms --limit 20                # recent security alarms (scans, new devices, …)
fire features                         # which features are on (ad block, VPN, DoH, QoS, …)
fire traffic "Living Room TV"         # who a device talks to (internet + LAN peers)
fire traffic phone laptop             # traffic between two devices
fire traffic phone --with video.example.com # …filtered to a destination
fire block "Kids iPad" --confirm      # block a device (--for 1h to auto-expire)
fire unblock "Kids iPad" --confirm    # remove the block
fire pause "Kids iPad" --for 1h --confirm  # pause internet (auto-resumes); resume to lift
fire rules add block dns ads.example.com --confirm   # create a rule
fire rules rm --confirm               # pick a rule to delete (also: enable/disable [id])
fire features enable adblock --confirm # toggle a box feature (also: disable; by key or name)
fire alarms ack --confirm             # pick an alarm to acknowledge (also: rm to delete)
fire redis keys 'policy:*'            # escape hatch: raw redis-cli on the box
```

### Interactive dashboard

`fire tui` (or just `fire` in a terminal) opens a Bubble Tea dashboard with a
tab bar across six views — **devices │ rules │ alarms │ networks │ wan │ data** —
switched with `R`/`A`/`N`/`W`/`D`, the number keys `1`–`6`, or `esc` back to
devices. Navigate any list with ↑/↓ (or `j`/`k`, `g`/`G`); `r` reloads, `?`
shows help, `q` quits. Every mutation is confirmed with `y` (`n`/`esc` cancels),
mirroring the CLI's `--confirm` gate. A load that finishes after you switch
views is dropped as stale, so the screen always reflects the view you're on
(see `specs/TuiLoad.tla`). Piped or redirected, `fire` prints help instead.

Every list view supports `/` to search/filter and `s` to cycle the sort
(default ↔ by name). `enter` opens a detail pane for the selected row.

- **devices** — `/` fuzzy-searches by name/IP/MAC; `o` toggles online-only;
  **enter** opens a device's detail pane (top traffic peers); `b`/`u`
  block/unblock the selection.
- **rules** — `e`/`d`/`x` enable/disable/delete the selected rule.
- **alarms** — `a` archives, `x` deletes the selected alarm.
- **networks** — read-only list of networks/VLANs.
- **wan** — read-only internet uplinks with live health (healthy/degraded/down).
- **data** — read-only data-plan usage this period, per WAN.

### Pickers everywhere

Run a command that selects something — `traffic`/`block`/`unblock` (a device),
`rules rm|enable|disable` (a rule), `features enable|disable` (a feature),
`alarms archive|rm` (an alarm) — with **no argument** in a terminal and an
interactive fuzzy finder opens over the live list (type to filter; ↑/↓ to move;
enter to select; esc to cancel). Tab-completion offers the same values. In a
pipe/script the commands require an explicit argument instead.

Mutating commands (`block`, `unblock`, `rules add|rm|enable|disable`,
`features enable|disable`, `alarms archive|rm`) print what they will do and
require `--confirm` to apply. They go through Firewalla's own managers
(PolicyManager2, HostManager, AlarmManager2) so changes are enforced exactly
like the app, not just written to redis.

`traffic` accepts a MAC, IP, or device name for both the device and the peer.
Internet peers show as domains/IPs (from Firewalla's `sumflow` rollups); LAN
peers resolve to device names (from `flow:local`/`sumflow:…:local`).

Global flags: `--host` (ssh destination, default `pi@fire.walla`), `--json`,
`--no-color`, `--timeout` (per-command wall-clock bound, default `30s`; `0`
disables). Color is auto-disabled for pipes and when `NO_COLOR` is set.

> The `redis` subcommand passes everything through to `redis-cli`, so put global
> flags **before** it: `fire --host pi@fire.walla redis ping`.

### Scripting & agents

Built to be driven by scripts and AI agents, not just humans:

- **`--json` on every command — reads *and* mutations.** Read commands emit a
  JSON array/object; mutating commands emit one result object:

  ```jsonc
  // fire --json block "Kids iPad" --confirm
  {"action":"block","target":"Kids iPad","mac":"AA:BB:CC:DD:EE:01","rule":"321","applied":true}
  // fire --json block "Kids iPad"          (no --confirm)
  {"action":"block","target":"Kids iPad","mac":"AA:BB:CC:DD:EE:01","applied":false,"dryRun":true}
  ```

  Key on `applied` (did it change anything?) and `dryRun`. `unblock` adds
  `count` (rules removed); `feature.*` add `state`; `rule.add`/`rule.*` set
  `rule` (policy id).
- **Dry-run by default.** Mutations need `--confirm`; without it they report the
  would-be change and exit 0 without touching the box — safe to probe.
- **Exit codes:** `0` success, `1` a handled error (message on stderr, prefixed
  `error:`), `2` an internal error contained at the boundary. Human text never
  pollutes stdout in `--json` mode.
- **Bounded:** every remote command is time-limited (`--timeout`) and its output
  is capped, so a stalled or hostile box degrades to an error, never a hang.
- **Discovery:** `--host`, MAC/IP/name resolution, and tab-completion
  (`fire completion <shell>`) all work headless; name/IP resolve to a device for
  `block`/`unblock`/`pause`/`resume`/`traffic`/`dns device`.

### Config file & named boxes

Optional, at `~/.config/fire/config.json` (or `$XDG_CONFIG_HOME/fire/config.json`,
or `--config <path>`). Set a default host, per-command defaults, and named
boxes so you can target several Firewallas without retyping `--host`:

```jsonc
{
  "default_box": "home",
  "timeout": "45s",
  "boxes": {
    "home":  { "host": "pi@fire.walla" },
    "cabin": { "host": "pi@cabin.lan" }
  }
}
```

```sh
fire devices                 # uses default_box → home
fire --box cabin devices     # target the cabin box
fire --host pi@other status  # an explicit --host always wins
```

A missing file is fine (built-in defaults apply); a malformed one warns and
falls back rather than failing.

## Architecture

Three layers, each depending only on the one below — which is what makes it
testable end-to-end without a network:

```
cmd (cobra)  ──►  internal/firewalla (typed Client + pure parsers)  ──►  internal/transport (SSH | Fake)
                                   └──►  internal/render (table / json, color-gated)
```

- **transport** is the only code that touches the network. Commands and the
  client are tested against a `FakeTransport`.
- **parsers are pure functions** tested against fixtures captured from a real
  box (anonymized — see `CLAUDE.md`). Every parser also has a `Fuzz` test.
- **`internal/tui`** is the Bubble Tea dashboard: a value-type `Model` driven
  through a `DataSource` interface, so the whole thing is unit-tested by
  feeding key messages and asserting on `View()` — no terminal required.
- **`specs/`** holds TLA+ models of the two stateful/concurrent parts: the
  device-access lifecycle (`BlockLifecycle.tla`, the timed-expiry race) and the
  TUI async-load supersede (`TuiLoad.tla`). Each ships a mutation config that
  must fail, pinning down why the design is correct. See `specs/README.md`.

## Development

```sh
go test ./...                          # all tests
go test ./internal/render -update      # regenerate render golden files
go test ./internal/tui -update         # regenerate TUI view snapshots
make build | test | lint | fmt
```

TDD throughout: tests are written first; `RunE` returns errors; output goes
through injected writers so it can be asserted.

## Data hygiene

This repo never contains real network data. Fixtures use RFC-5737
documentation IPs, fake MACs, and `example.com` domains. See
[`CLAUDE.md`](./CLAUDE.md) before adding fixtures.

## License

MIT
