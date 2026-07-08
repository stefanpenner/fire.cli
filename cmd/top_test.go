package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTop_Table(t *testing.T) {
	client := &fakeClient{
		devices: []firewalla.Device{{MAC: "AA:BB:CC:DD:EE:01", Name: "Living Room TV"}},
		topTalkers: []firewalla.TopTalker{
			{MAC: "AA:BB:CC:DD:EE:01", Download: 4200000000, Upload: 120000000},
			{MAC: "AA:BB:CC:DD:EE:02", Download: 1100000000, Upload: 30000000},
		},
	}
	out, _, err := exec(t, client, "top")
	require.NoError(t, err)
	assert.Contains(t, out, "Living Room TV") // MAC resolved to name
	assert.Contains(t, out, "3.9 GB")         // 4.2e9 bytes humanized
}

func TestTop_JSON(t *testing.T) {
	client := &fakeClient{topTalkers: []firewalla.TopTalker{
		{MAC: "AA:BB:CC:DD:EE:01", Download: 100, Upload: 20},
	}}
	out, _, err := exec(t, client, "top", "--json")
	require.NoError(t, err)
	var got []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	require.Len(t, got, 1)
	assert.EqualValues(t, 100, got[0]["download"])
}

func TestTop_Limit(t *testing.T) {
	client := &fakeClient{topTalkers: []firewalla.TopTalker{
		{MAC: "AA:BB:CC:DD:EE:01", Download: 3},
		{MAC: "AA:BB:CC:DD:EE:02", Download: 2},
		{MAC: "AA:BB:CC:DD:EE:03", Download: 1},
	}}
	out, _, err := exec(t, client, "top", "--limit", "1")
	require.NoError(t, err)
	assert.Contains(t, out, "AA:BB:CC:DD:EE:01")
	assert.NotContains(t, out, "AA:BB:CC:DD:EE:02")
}
