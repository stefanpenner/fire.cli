package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClient implements the Client interface and records calls.
type fakeClient struct {
	devices   []firewalla.Device
	resolvers []firewalla.Resolver
	dns       []firewalla.DNSQuery
	rawOut    string
	err       error

	// recorded inputs
	gotDomain  string
	gotMAC     string
	gotLimit   int
	gotRawArgs string
}

func (f *fakeClient) Host() string { return "pi@test" }
func (f *fakeClient) ListDevices(context.Context) ([]firewalla.Device, error) {
	return f.devices, f.err
}
func (f *fakeClient) DNSByDevice(_ context.Context, mac string, limit int) ([]firewalla.DNSQuery, error) {
	f.gotMAC, f.gotLimit = mac, limit
	return f.dns, f.err
}
func (f *fakeClient) WhoResolved(_ context.Context, domain string) ([]firewalla.Resolver, error) {
	f.gotDomain = domain
	return f.resolvers, f.err
}
func (f *fakeClient) Raw(_ context.Context, args string) (string, error) {
	f.gotRawArgs = args
	return f.rawOut, f.err
}

// exec runs the CLI with the given args and a fixed clock, returning stdout.
func exec(t *testing.T, client Client, args ...string) (string, string, error) {
	t.Helper()
	var out, errBuf bytes.Buffer
	app := &App{
		Out:    &out,
		Err:    &errBuf,
		Client: client,
		Now:    func() time.Time { return time.Unix(1700000100, 0) },
	}
	root := NewRootCmd(app)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errBuf.String(), err
}

func TestVersion(t *testing.T) {
	out, _, err := exec(t, &fakeClient{}, "version")
	require.NoError(t, err)
	assert.Contains(t, out, Version)
}

func TestVersion_JSON(t *testing.T) {
	out, _, err := exec(t, &fakeClient{}, "version", "--json")
	require.NoError(t, err)
	var got map[string]string
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, Version, got["version"])
}

func TestDevices_Table(t *testing.T) {
	client := &fakeClient{devices: []firewalla.Device{
		{MAC: "AA:BB:CC:DD:EE:02", Name: "Example Hot Tub", IP: "192.0.2.20", Type: "spa", LastActive: time.Unix(1700000050, 0)},
		{MAC: "AA:BB:CC:DD:EE:09", Name: "Old Laptop", IP: "192.0.2.99", LastActive: time.Unix(1600000000, 0)},
	}}
	out, _, err := exec(t, client, "devices")
	require.NoError(t, err)
	assert.Contains(t, out, "Example Hot Tub")
	assert.Contains(t, out, "192.0.2.20")
	assert.Contains(t, out, "online")  // seen 50s ago vs 5m window
	assert.Contains(t, out, "offline") // old laptop
}

func TestDevices_JSON(t *testing.T) {
	client := &fakeClient{devices: []firewalla.Device{
		{MAC: "AA:BB:CC:DD:EE:02", Name: "Example Hot Tub", IP: "192.0.2.20"},
	}}
	out, _, err := exec(t, client, "devices", "--json")
	require.NoError(t, err)
	var got []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	require.Len(t, got, 1)
	assert.Equal(t, "Example Hot Tub", got[0]["name"])
}

func TestDevices_Error(t *testing.T) {
	_, _, err := exec(t, &fakeClient{err: errors.New("ssh failed")}, "devices")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssh failed")
}

func TestDNSWho(t *testing.T) {
	client := &fakeClient{resolvers: []firewalla.Resolver{
		{Client: "192.0.2.10", Count: 26},
		{Client: "192.0.2.11", Count: 1},
	}}
	out, _, err := exec(t, client, "dns", "who", "broker.example.com")
	require.NoError(t, err)
	assert.Equal(t, "broker.example.com", client.gotDomain)
	assert.Contains(t, out, "192.0.2.10")
	assert.Contains(t, out, "26")
}

func TestDNSWho_RequiresDomain(t *testing.T) {
	_, _, err := exec(t, &fakeClient{}, "dns", "who")
	require.Error(t, err)
}

func TestDNSDevice_PassesLimit(t *testing.T) {
	client := &fakeClient{dns: []firewalla.DNSQuery{
		{Domain: "broker.example.com", Client: "192.0.2.10", Count: 26},
	}}
	out, _, err := exec(t, client, "dns", "device", "aa:bb:cc:dd:ee:02", "--limit", "5")
	require.NoError(t, err)
	assert.Equal(t, "aa:bb:cc:dd:ee:02", client.gotMAC)
	assert.Equal(t, 5, client.gotLimit)
	assert.Contains(t, out, "broker.example.com")
}

func TestStatus(t *testing.T) {
	client := &fakeClient{rawOut: "PONG\n", devices: make([]firewalla.Device, 3)}
	out, _, err := exec(t, client, "status")
	require.NoError(t, err)
	assert.Contains(t, out, "pi@test")
	assert.Contains(t, out, "3") // device count
}

func TestRedisPassthrough(t *testing.T) {
	client := &fakeClient{rawOut: "PONG\n"}
	out, _, err := exec(t, client, "redis", "ping")
	require.NoError(t, err)
	assert.Equal(t, "ping", client.gotRawArgs)
	assert.Contains(t, out, "PONG")
}

func TestUnknownCommand(t *testing.T) {
	_, _, err := exec(t, &fakeClient{}, "frobnicate")
	require.Error(t, err)
}
