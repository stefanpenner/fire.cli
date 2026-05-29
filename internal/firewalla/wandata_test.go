package firewalla

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWANs(t *testing.T) {
	wans, err := parseWANs(fixture(t, "wan_stream.txt"))
	require.NoError(t, err)
	require.Len(t, wans, 2)

	// primary first
	assert.Equal(t, "ISP-A", wans[0].Name)
	assert.Equal(t, "eth0", wans[0].Interface)
	assert.Equal(t, "primary", wans[0].Role)
	assert.Equal(t, "primary_standby", wans[0].Mode)
	assert.True(t, wans[0].Active)
	assert.True(t, wans[0].Ping)

	assert.Equal(t, "ISP-B", wans[1].Name)
	assert.Equal(t, "standby", wans[1].Role)
	assert.False(t, wans[1].Active) // wanConnState.active false and connectivity-active handled
	assert.False(t, wans[1].DNS)
}

func TestParseDataUsage(t *testing.T) {
	r := parseDataUsage(fixture(t, "data_usage.txt"))
	assert.Equal(t, int64(1000000000000), r.PlanTotal)
	assert.Equal(t, 1, r.ResetDay)
	require.Len(t, r.WANs, 2)

	byUUID := map[string]WANUsage{}
	for _, w := range r.WANs {
		byUUID[w.UUID] = w
	}
	a := byUUID["338ef2cb-35d6-4ebe-b643-04174b23f27b"]
	assert.Equal(t, int64(300), a.Download)
	assert.Equal(t, int64(30), a.Upload)

	b := byUUID["60bbef6a-80be-499b-b131-639f6cf249ac"]
	assert.Equal(t, int64(6), b.Bytes())

	assert.Equal(t, int64(336), r.Total())
}
