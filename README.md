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
fire --host pi@fire.walla status      # is the box reachable? how many devices?
fire devices                          # list devices (online/offline, last seen)
fire devices --json | jq '.[].name'   # machine-readable
fire dns who broker.example.com       # which clients resolved this domain?
fire dns device AA:BB:CC:DD:EE:FF     # recent DNS lookups by a device
fire networks                         # networks & VLANs (alias: vlans)
fire wan                              # internet uplinks: dual-WAN role + live health
fire data                             # data-plan usage this period, per WAN (alias: usage)
fire rules                            # firewall block/allow rules (--all for disabled)
fire alarms --limit 20                # recent security alarms (scans, new devices, …)
fire features                         # which features are on (ad block, VPN, DoH, QoS, …)
fire traffic "Living Room TV"         # who a device talks to (internet + LAN peers)
fire traffic phone laptop             # traffic between two devices
fire traffic phone --with spotify.com # …filtered to a destination
fire block "Kids iPad" --confirm      # block a device (--for 1h to auto-expire)
fire unblock "Kids iPad" --confirm    # remove the block
fire rules add block dns ads.example.com --confirm   # create a rule
fire rules rm 215 --confirm           # delete a rule (also: enable/disable <id>)
fire redis keys 'policy:*'            # escape hatch: raw redis-cli on the box
```

Run `traffic`, `block`, or `unblock` with **no device argument** in a terminal
and an interactive fuzzy finder opens over your devices (type to filter on name,
IP, or MAC; ↑/↓ to move; enter to select; esc to cancel). In a pipe/script it
falls back to requiring an explicit argument.

Mutating commands (`block`, `unblock`, `rules add|rm|enable|disable`) print what
they will do and require `--confirm` to apply. They go through Firewalla's own
PolicyManager so changes are enforced exactly like the app, not just written to
redis.

`traffic` accepts a MAC, IP, or device name for both the device and the peer.
Internet peers show as domains/IPs (from Firewalla's `sumflow` rollups); LAN
peers resolve to device names (from `flow:local`/`sumflow:…:local`).

Global flags: `--host` (ssh destination, default `pi@fire.walla`), `--json`,
`--no-color`. Color is auto-disabled for pipes and when `NO_COLOR` is set.

> The `redis` subcommand passes everything through to `redis-cli`, so put global
> flags **before** it: `fire --host pi@fire.walla redis ping`.

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
  box (anonymized — see `CLAUDE.md`).

## Development

```sh
go test ./...                          # all tests
go test ./internal/render -update      # regenerate golden files
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
