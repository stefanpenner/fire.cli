// Package transport is the only layer that touches the network. It runs a
// shell command on a Firewalla box and returns its raw output. Everything
// above it (the firewalla client, the cobra commands) depends on the
// Transport interface, never on ssh directly, which keeps the rest of the
// code unit-testable against a FakeTransport.
package transport

import "context"

// Result is the outcome of running a single command on the remote box.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Transport runs a shell command on a Firewalla box.
//
// Implementations:
//   - SSHTransport: shells out to the system ssh binary (real).
//   - FakeTransport: canned responses for tests.
type Transport interface {
	// Run executes command on the remote host and returns its output.
	// A non-nil error means the command could not be run or exited
	// non-zero; Result is still populated where possible.
	Run(ctx context.Context, command string) (Result, error)
	// Host returns the ssh destination (e.g. "pi@fire"), for messages.
	Host() string
}
