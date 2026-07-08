package firewalla

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNetworks(t *testing.T) {
	nets := parseNetworks(fixture(t, "network_info.txt"))
	require.Len(t, nets, 4) // publicIp and ddns plain fields are skipped

	// WANs first, then LANs by VLAN id.
	assert.Equal(t, "wan", nets[0].Type)
	assert.Equal(t, "wan", nets[1].Type)

	byName := map[string]Network{}
	for _, n := range nets {
		byName[n.Name] = n
	}

	iot := byName["IoT"]
	assert.Equal(t, "eth2.2001", iot.Interface)
	assert.Equal(t, "eth2", iot.Parent)
	assert.Equal(t, 2001, iot.VLANID)
	assert.Equal(t, "lan", iot.Type)
	assert.Equal(t, "192.0.2.64/26", iot.Subnet)

	home := byName["Home"]
	assert.Equal(t, "br0", home.Interface)
	assert.Zero(t, home.VLANID)
	assert.Equal(t, "192.0.2.1", home.Gateway)

	ispA := byName["ISP-A"]
	assert.Equal(t, "wan", ispA.Type)
	assert.Equal(t, []string{"203.0.113.1", "203.0.113.10"}, ispA.DNS)
}

func TestParseNetworks_Empty(t *testing.T) {
	assert.Empty(t, parseNetworks(""))
}
