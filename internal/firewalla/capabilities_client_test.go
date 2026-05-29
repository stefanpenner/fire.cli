package firewalla

import (
	"context"
	"testing"

	"github.com/stefanpenner/fire.cli/internal/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListNetworks(t *testing.T) {
	fake := transport.NewFake("pi@fire").
		OnMatch("sys:network:info", transport.Result{Stdout: fixture(t, "network_info.txt")})
	c := New(fake)

	nets, err := c.ListNetworks(context.Background())
	require.NoError(t, err)
	require.Len(t, nets, 4)
	assert.Contains(t, fake.Commands[0], "sys:network:info")
}

func TestClient_ListRules(t *testing.T) {
	fake := transport.NewFake("pi@fire").
		OnMatch("policy:", transport.Result{Stdout: fixture(t, "rules.txt")})
	c := New(fake)

	rules, err := c.ListRules(context.Background())
	require.NoError(t, err)
	require.Len(t, rules, 3)
	assert.Contains(t, fake.Commands[0], "policy:[0-9]*")
}

func TestClient_Traffic(t *testing.T) {
	fake := transport.NewFake("pi@fire").
		OnMatch("sumflow:AA:BB:CC:DD:EE:01", transport.Result{Stdout: fixture(t, "traffic_sumflow.txt")})
	c := New(fake)

	peers, err := c.Traffic(context.Background(), "aa:bb:cc:dd:ee:01")
	require.NoError(t, err)
	require.Len(t, peers, 3)
	// MAC is upper-cased into the sumflow key patterns.
	assert.Contains(t, fake.Commands[0], "sumflow:AA:BB:CC:DD:EE:01")
}

func TestClient_ListWANs(t *testing.T) {
	fake := transport.NewFake("pi@fire").
		OnMatch("wan/connectivity", transport.Result{Stdout: fixture(t, "wan_stream.txt")})
	c := New(fake)

	wans, err := c.ListWANs(context.Background())
	require.NoError(t, err)
	require.Len(t, wans, 2)
	assert.Equal(t, "primary", wans[0].Role)
}

func TestClient_DataUsage(t *testing.T) {
	fake := transport.NewFake("pi@fire").
		OnMatch("sys:data:plan", transport.Result{Stdout: fixture(t, "data_usage.txt")})
	c := New(fake)

	r, err := c.DataUsage(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1000000000000), r.PlanTotal)
	require.Len(t, r.WANs, 2)
}
