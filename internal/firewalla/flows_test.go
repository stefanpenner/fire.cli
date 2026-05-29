package firewalla

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTraffic(t *testing.T) {
	peers := parseTraffic(fixture(t, "traffic_sumflow.txt"))
	require.Len(t, peers, 3)

	byLabel := map[string]Peer{}
	for _, p := range peers {
		byLabel[p.Label] = p
	}

	// Internet peer: merges download + upload across sections.
	video := byLabel["video.example.com"]
	assert.Equal(t, "internet", video.Kind)
	assert.Equal(t, int64(409600), video.Download)
	assert.Equal(t, int64(2048), video.Upload)
	assert.Equal(t, int64(411648), video.Bytes())

	// Internet peer with download only.
	assert.Equal(t, int64(8192), byLabel["api.example.net"].Download)

	// Device↔device peer keyed by dstMac.
	dev := byLabel["AA:BB:CC:DD:EE:02"]
	assert.Equal(t, "device", dev.Kind)
	assert.Equal(t, "AA:BB:CC:DD:EE:02", dev.PeerMAC)
	assert.Equal(t, int64(1024), dev.Download)
	assert.Equal(t, int64(512), dev.Upload)

	// Sorted by total bytes desc.
	assert.Equal(t, "video.example.com", peers[0].Label)
}

func TestParseTraffic_Empty(t *testing.T) {
	assert.Empty(t, parseTraffic(""))
}
