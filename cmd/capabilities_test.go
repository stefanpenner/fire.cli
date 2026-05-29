package cmd

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworks_Table(t *testing.T) {
	client := &fakeClient{networks: []firewalla.Network{
		{Name: "Home", Type: "lan", Interface: "br0", Subnet: "192.0.2.0/24"},
		{Name: "IoT", Type: "lan", Interface: "eth2.2001", Parent: "eth2", VLANID: 2001, Subnet: "192.0.2.64/26"},
	}}
	out, _, err := exec(t, client, "vlans")
	require.NoError(t, err)
	assert.Contains(t, out, "IoT")
	assert.Contains(t, out, "2001")
	assert.Contains(t, out, "192.0.2.64/26")
}

func TestRules_FiltersDisabledByDefault(t *testing.T) {
	client := &fakeClient{rules: []firewalla.Rule{
		{ID: "1", Action: "block", Type: "dns", Target: "ads.example.net", Disabled: false},
		{ID: "2", Action: "block", Type: "category", Target: "games", Disabled: true},
	}}
	out, _, err := exec(t, client, "rules")
	require.NoError(t, err)
	assert.Contains(t, out, "ads.example.net")
	assert.NotContains(t, out, "games") // disabled, hidden without --all

	outAll, _, err := exec(t, client, "rules", "--all")
	require.NoError(t, err)
	assert.Contains(t, outAll, "games")
}

func TestWAN_Table(t *testing.T) {
	client := &fakeClient{wans: []firewalla.WAN{
		{Name: "ISP-A", Interface: "eth0", Role: "primary", Mode: "primary_standby", Active: true, Carrier: true, Ping: true, DNS: true},
		{Name: "ISP-B", Interface: "eth3", Role: "standby", Mode: "primary_standby", Active: false, Carrier: true},
	}}
	out, _, err := exec(t, client, "wan")
	require.NoError(t, err)
	assert.Contains(t, out, "ISP-A")
	assert.Contains(t, out, "primary")
	assert.Contains(t, out, "healthy")
}

func TestData_SummaryAndPlan(t *testing.T) {
	client := &fakeClient{
		networks: []firewalla.Network{{Name: "ISP-A", UUID: "u-1", Type: "wan"}},
		dataUsage: firewalla.DataUsageReport{
			PlanTotal: 1000000000000,
			ResetDay:  1,
			WANs:      []firewalla.WANUsage{{UUID: "u-1", Upload: 1024, Download: 1048576}},
		},
	}
	out, _, err := exec(t, client, "data")
	require.NoError(t, err)
	assert.Contains(t, out, "ISP-A")  // uuid resolved to name
	assert.Contains(t, out, "plan")   // summary line with plan
	assert.Contains(t, out, "1.0 MB") // download
}

func TestTraffic_ResolvesDeviceAndRenders(t *testing.T) {
	client := &fakeClient{
		devices: []firewalla.Device{
			{MAC: "AA:BB:CC:DD:EE:01", Name: "Phone", IP: "192.0.2.10"},
			{MAC: "AA:BB:CC:DD:EE:02", Name: "Laptop"},
		},
		peers: []firewalla.Peer{
			{Label: "video.example.com", Kind: "internet", Download: 409600, Upload: 2048},
			{PeerMAC: "AA:BB:CC:DD:EE:02", Kind: "device", Download: 1024, Upload: 512},
		},
	}
	// Resolve device by name.
	out, _, err := exec(t, client, "traffic", "Phone")
	require.NoError(t, err)
	assert.Equal(t, "AA:BB:CC:DD:EE:01", client.gotMAC)
	assert.Contains(t, out, "video.example.com")
	assert.Contains(t, out, "Laptop") // device peer rendered by name
}

func TestTraffic_FilterBetweenTwoDevices(t *testing.T) {
	client := &fakeClient{
		devices: []firewalla.Device{
			{MAC: "AA:BB:CC:DD:EE:01", Name: "Phone"},
			{MAC: "AA:BB:CC:DD:EE:02", Name: "Laptop"},
		},
		peers: []firewalla.Peer{
			{Label: "video.example.com", Kind: "internet", Download: 409600},
			{PeerMAC: "AA:BB:CC:DD:EE:02", Kind: "device", Download: 1024},
		},
	}
	out, _, err := exec(t, client, "traffic", "Phone", "Laptop")
	require.NoError(t, err)
	assert.Contains(t, out, "Laptop")
	assert.NotContains(t, out, "video.example.com") // filtered to the device peer
}

func TestTraffic_UnknownDevice(t *testing.T) {
	_, _, err := exec(t, &fakeClient{}, "traffic", "Nonexistent")
	require.Error(t, err)
}

func TestAlarms_Table(t *testing.T) {
	client := &fakeClient{alarms: []firewalla.Alarm{
		{ID: "2297", Type: "Port Scan", Device: "Laptop", Message: "Laptop was scanning ports", Time: time.Unix(1700000050, 0)},
	}}
	out, _, err := exec(t, client, "alarms", "--limit", "10")
	require.NoError(t, err)
	assert.Equal(t, 10, client.gotLimit)
	assert.Contains(t, out, "Port Scan")
	assert.Contains(t, out, "Laptop")
}

func TestFeatures_Table(t *testing.T) {
	client := &fakeClient{features: []firewalla.Feature{
		{Key: "adblock", Name: "Ad Block", Enabled: false},
		{Key: "vpn", Name: "VPN Server", Enabled: true},
	}}
	out, _, err := exec(t, client, "features")
	require.NoError(t, err)
	assert.Contains(t, out, "Ad Block")
	assert.Contains(t, out, "off")
	assert.Contains(t, out, "VPN Server")
	assert.Contains(t, out, "on")
}

func TestData_JSON(t *testing.T) {
	client := &fakeClient{dataUsage: firewalla.DataUsageReport{
		PlanTotal: 1000,
		WANs:      []firewalla.WANUsage{{UUID: "u-1", Upload: 10, Download: 20}},
	}}
	out, _, err := exec(t, client, "data", "--json")
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, float64(1000), got["planTotal"])
}
