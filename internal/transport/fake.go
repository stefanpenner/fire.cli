package transport

import (
	"context"
	"fmt"
	"strings"
)

// FakeTransport is an in-memory Transport for tests. It records every command
// it is asked to run and returns programmed responses, matched either exactly
// (On) or by substring (OnMatch). Exact matches win over substring matches.
type FakeTransport struct {
	host     string
	exact    map[string]Result
	matchers []matcher
	// Commands records, in order, every command passed to Run.
	Commands []string
}

type matcher struct {
	substr string
	result Result
}

// NewFake returns a FakeTransport for the given host.
func NewFake(host string) *FakeTransport {
	return &FakeTransport{host: host, exact: map[string]Result{}}
}

// On programs an exact-match response. Returns the transport for chaining.
func (f *FakeTransport) On(command string, result Result) *FakeTransport {
	f.exact[command] = result
	return f
}

// OnMatch programs a substring-match response. Returns the transport for
// chaining. Matchers are tried in registration order.
func (f *FakeTransport) OnMatch(substr string, result Result) *FakeTransport {
	f.matchers = append(f.matchers, matcher{substr: substr, result: result})
	return f
}

// Host returns the configured host.
func (f *FakeTransport) Host() string { return f.host }

// Run records the command and returns the programmed response, or an error if
// no response was programmed (so tests fail loudly on unexpected commands).
func (f *FakeTransport) Run(_ context.Context, command string) (Result, error) {
	f.Commands = append(f.Commands, command)
	if r, ok := f.exact[command]; ok {
		return r, nil
	}
	for _, m := range f.matchers {
		if strings.Contains(command, m.substr) {
			return m.result, nil
		}
	}
	return Result{}, fmt.Errorf("fake transport: no programmed response for command: %q", command)
}
