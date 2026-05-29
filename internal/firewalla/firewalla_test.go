package firewalla

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stefanpenner/fire.cli/internal/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(b)
}

func TestParseRedisHash(t *testing.T) {
	m := parseRedisHash("mac\nAA:BB\nipv4Addr\n10.0.0.1\n")
	assert.Equal(t, "AA:BB", m["mac"])
	assert.Equal(t, "10.0.0.1", m["ipv4Addr"])
	assert.Len(t, m, 2)
}

func TestParseRedisHash_IgnoresDanglingKey(t *testing.T) {
	// An odd number of lines (trailing key with no value) must not panic.
	m := parseRedisHash("mac\nAA:BB\ndangling\n")
	assert.Equal(t, "AA:BB", m["mac"])
	_, ok := m["dangling"]
	assert.False(t, ok)
}

func TestParseDevices(t *testing.T) {
	devices := parseDevices(fixture(t, "devices.txt"))
	require.Len(t, devices, 2)

	phone := devices[0]
	assert.Equal(t, "AA:BB:CC:DD:EE:01", phone.MAC)
	assert.Equal(t, "Example Phone", phone.Name) // falls back to bname when no name
	assert.Equal(t, "192.0.2.10", phone.IP)
	assert.Equal(t, "Acme Devices, Inc.", phone.Vendor)
	assert.Equal(t, "phone", phone.Type) // pulled out of detect JSON
	assert.Equal(t, int64(1700000575), phone.LastActive.Unix())
	assert.Equal(t, int64(1690000842), phone.FirstSeen.Unix())

	spa := devices[1]
	assert.Equal(t, "AA:BB:CC:DD:EE:02", spa.MAC)
	assert.Equal(t, "Example Hot Tub", spa.Name) // explicit name field wins
	assert.Equal(t, "192.0.2.20", spa.IP)
	assert.Equal(t, "spa", spa.Type)
}

func TestDevice_SeenWithin(t *testing.T) {
	spa := Device{LastActive: time.Unix(1700000007, 0)}
	now := time.Unix(1700000100, 0) // 93s later
	assert.True(t, spa.SeenWithin(5*time.Minute, now))
	assert.False(t, spa.SeenWithin(30*time.Second, now))
	// Zero LastActive is never "seen".
	assert.False(t, Device{}.SeenWithin(time.Hour, now))
}

func TestParseDNSFlows(t *testing.T) {
	q, err := parseDNSFlows(fixture(t, "dns_flows.txt"))
	require.NoError(t, err)
	require.Len(t, q, 3)

	assert.Equal(t, "shop.example.com", q[0].Domain)
	assert.Equal(t, "192.0.2.10", q[0].Client)
	assert.Equal(t, []string{"cdn.example.net", "198.51.100.41"}, q[0].Answers)
	assert.Equal(t, int64(1700000445), q[0].Time.Unix())

	assert.Equal(t, "broker.example.com", q[2].Domain)
	assert.Equal(t, 26, q[2].Count)
	assert.Equal(t, []string{"198.51.100.7"}, q[2].Answers)
}

func TestParseDNSFlows_SkipsBlankLines(t *testing.T) {
	q, err := parseDNSFlows("\n\n")
	require.NoError(t, err)
	assert.Empty(t, q)
}

func TestParseResolvers(t *testing.T) {
	// Who resolved broker.example.com? (the "who queried this domain" feature)
	r := parseResolvers(fixture(t, "zeek_dns.log"), "broker.example.com")
	require.Len(t, r, 2)
	// Sorted by count desc: 192.0.2.10 (2) before 192.0.2.11 (1).
	assert.Equal(t, "192.0.2.10", r[0].Client)
	assert.Equal(t, 2, r[0].Count)
	assert.Equal(t, "192.0.2.11", r[1].Client)
	assert.Equal(t, 1, r[1].Count)
}

// ---- Client methods over a FakeTransport ----

func TestClient_ListDevices(t *testing.T) {
	fake := transport.NewFake("pi@fire").
		OnMatch("host:mac:", transport.Result{Stdout: fixture(t, "devices.txt")})
	c := New(fake)

	devices, err := c.ListDevices(context.Background())
	require.NoError(t, err)
	assert.Len(t, devices, 2)
	assert.Equal(t, "Example Hot Tub", devices[1].Name)
}

func TestClient_DNSByDevice(t *testing.T) {
	fake := transport.NewFake("pi@fire").
		OnMatch("flow:dns:AA:BB:CC:DD:EE:02", transport.Result{Stdout: fixture(t, "dns_flows.txt")})
	c := New(fake)

	q, err := c.DNSByDevice(context.Background(), "AA:BB:CC:DD:EE:02", 100)
	require.NoError(t, err)
	require.Len(t, q, 3)
	// The command must target the right per-device zset.
	assert.Contains(t, fake.Commands[0], "flow:dns:AA:BB:CC:DD:EE:02")
}

func TestClient_WhoResolved(t *testing.T) {
	fake := transport.NewFake("pi@fire").
		OnMatch("broker.example.com", transport.Result{Stdout: fixture(t, "zeek_dns.log")})
	c := New(fake)

	resolvers, err := c.WhoResolved(context.Background(), "broker.example.com")
	require.NoError(t, err)
	require.Len(t, resolvers, 2)
	assert.Equal(t, "192.0.2.10", resolvers[0].Client)
}

func TestClient_PropagatesTransportError(t *testing.T) {
	fake := transport.NewFake("pi@fire") // nothing programmed
	c := New(fake)
	_, err := c.ListDevices(context.Background())
	require.Error(t, err)
}
