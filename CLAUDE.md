# fire.cli — agent & contributor guide

`fire` is a Go CLI for a Firewalla box. It runs commands on the box over SSH
(`redis-cli`, Zeek logs, scripts) and renders typed results.

## ⚠️ Data hygiene — DO NOT LEAK INTERNAL DATA

This is a real repository that may be public. **Never commit real network data
from any actual Firewalla box or LAN.** That includes, in code, tests, fixtures,
docs, commit messages, and example output:

- real **MAC addresses**, **IP addresses**, **hostnames**, **device names**,
  **SSIDs**, **domains**, **user/bonjour names**, or **geolocation**.

When you capture sample data from a live box to build a fixture, **anonymize it
first**:

- IPs → RFC-5737 documentation ranges: `192.0.2.0/24`, `198.51.100.0/24`,
  `203.0.113.0/24` (and `2001:db8::/32` for IPv6).
- MACs → locally-administered fakes like `AA:BB:CC:DD:EE:01`.
- domains → `example.com` / `example.net` / `example.invalid`.
- device names → generic labels ("Example Phone", "Example Hot Tub").
- timestamps → round/synthetic epoch values.

The default `--host` is the generic `pi@fire`; never hardcode a real box
hostname, IP, or key path.

When running the CLI live against a real box for verification, prefer commands
that do not echo sensitive data into logs/chat (`status`, `redis ping`); avoid
pasting raw `devices` / `dns` output anywhere it will be committed or shared.

## Architecture (three layers, each depends only on the one below)

1. `internal/transport` — the only layer that touches the network. `Transport`
   interface; `SSHTransport` shells out to `ssh`; `FakeTransport` for tests.
2. `internal/firewalla` — typed `Client` over a `Transport`. **All parsing lives
   in pure functions** (`parseDevices`, `parseDNSFlows`, `parseResolvers`) tested
   against fixtures in `internal/firewalla/testdata/`.
3. `cmd` — cobra commands depending only on the consumer-side `Client`
   interface and an injected `App` (writers + clock), so every command is
   unit-tested against a fake client with no SSH.
4. `internal/render` — table (lipgloss for TTY, plain/tab for pipes) + JSON;
   color gated behind isatty/`NO_COLOR`.

## Conventions

- TDD: write the test first. `RunE` returns errors; `main` prints once + exits.
- No `init()`, no package globals for state — inject via `App`.
- Human output → stdout; errors → stderr; `--json` for machine output.

## Commands

- `go test ./...` — run everything.
- `go test ./internal/render -update` — regenerate golden files.
- `make build` / `make test` / `make lint`.
