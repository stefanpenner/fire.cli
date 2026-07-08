package firewalla

import (
	"context"
	"testing"

	"github.com/stefanpenner/fire.cli/internal/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTopTalkers(t *testing.T) {
	got := parseTopTalkers(fixture(t, "top_talkers.txt"))
	require.Len(t, got, 3)

	// Ranked by total bytes desc: device 01 (4.32G) > 02 (1.13G) > 03 (210M,
	// download excluded because it's a local flow).
	assert.Equal(t, "AA:BB:CC:DD:EE:01", got[0].MAC)
	assert.EqualValues(t, 4200000000, got[0].Download)
	assert.EqualValues(t, 120000000, got[0].Upload)

	assert.Equal(t, "AA:BB:CC:DD:EE:02", got[1].MAC)

	// Device 03's local download is excluded; only its internet upload counts.
	assert.Equal(t, "AA:BB:CC:DD:EE:03", got[2].MAC)
	assert.EqualValues(t, 0, got[2].Download)
	assert.EqualValues(t, 210000000, got[2].Upload)

	// The stale older window for device 01 must not override the recent one.
	assert.NotEqualValues(t, 999, got[0].Download)
}

func TestClient_TopTalkers(t *testing.T) {
	fake := transport.NewFake("pi@fire").
		OnMatch("sumflow:*", transport.Result{Stdout: fixture(t, "top_talkers.txt")})
	c := New(fake)
	got, err := c.TopTalkers(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)
	assert.Equal(t, "AA:BB:CC:DD:EE:01", got[0].MAC)
}
