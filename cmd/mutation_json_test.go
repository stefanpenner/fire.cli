package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeResult parses the single JSON object a mutating command emits with
// --json.
func decodeResult(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(s), &m), "output must be one JSON object: %q", s)
	return m
}

// A confirmed block emits a structured, applied result an agent can parse.
func TestBlock_JSON_Applied(t *testing.T) {
	client := &fakeClient{
		devices:   []firewalla.Device{{MAC: "AA:BB:CC:DD:EE:01", Name: "Phone"}},
		createPID: "321",
	}
	out, _, err := exec(t, client, "--json", "block", "Phone", "--confirm")
	require.NoError(t, err)
	got := decodeResult(t, out)
	assert.Equal(t, "block", got["action"])
	assert.Equal(t, "Phone", got["target"])
	assert.Equal(t, "AA:BB:CC:DD:EE:01", got["mac"])
	assert.Equal(t, "321", got["rule"])
	assert.Equal(t, true, got["applied"])
}

// A dry-run block (no --confirm) emits applied=false + dryRun=true and does not
// mutate.
func TestBlock_JSON_DryRun(t *testing.T) {
	client := &fakeClient{devices: []firewalla.Device{{MAC: "AA:BB:CC:DD:EE:01", Name: "Phone"}}}
	out, _, err := exec(t, client, "--json", "block", "Phone")
	require.NoError(t, err)
	got := decodeResult(t, out)
	assert.Equal(t, false, got["applied"])
	assert.Equal(t, true, got["dryRun"])
	assert.Empty(t, client.gotRuleSpec.Target, "dry-run must not mutate")
}

// Unblock reports the number of rules removed.
func TestUnblock_JSON_Count(t *testing.T) {
	client := &fakeClient{devices: []firewalla.Device{{MAC: "AA:BB:CC:DD:EE:01", Name: "Phone"}}}
	out, _, err := exec(t, client, "--json", "unblock", "Phone", "--confirm")
	require.NoError(t, err)
	got := decodeResult(t, out)
	assert.Equal(t, "unblock", got["action"])
	assert.EqualValues(t, 1, got["count"])
	assert.Equal(t, true, got["applied"])
}

// Feature toggle reports the target feature and state.
func TestFeatureEnable_JSON(t *testing.T) {
	client := &fakeClient{features: []firewalla.Feature{
		{Key: "adblock", Name: "Ad Block", Enabled: false},
	}}
	out, _, err := exec(t, client, "--json", "features", "enable", "Ad Block", "--confirm")
	require.NoError(t, err)
	got := decodeResult(t, out)
	assert.Equal(t, "feature.enable", got["action"])
	assert.Equal(t, "Ad Block", got["target"])
	assert.Equal(t, "on", got["state"])
	assert.Equal(t, true, got["applied"])
	assert.True(t, client.gotFeatSet)
}

// A feature already in the requested state reports applied=false, no mutation.
func TestFeatureEnable_JSON_AlreadyOn(t *testing.T) {
	client := &fakeClient{features: []firewalla.Feature{
		{Key: "adblock", Name: "Ad Block", Enabled: true},
	}}
	out, _, err := exec(t, client, "--json", "features", "enable", "Ad Block", "--confirm")
	require.NoError(t, err)
	got := decodeResult(t, out)
	assert.Equal(t, false, got["applied"])
	assert.False(t, client.gotFeatSet, "no-op must not call SetFeature")
}

// rules add emits the created policy id.
func TestRuleAdd_JSON(t *testing.T) {
	client := &fakeClient{createPID: "77"}
	out, _, err := exec(t, client, "--json", "rules", "add", "block", "dns", "ads.example.com", "--confirm")
	require.NoError(t, err)
	got := decodeResult(t, out)
	assert.Equal(t, "rule.add", got["action"])
	assert.Equal(t, "77", got["rule"])
	assert.Equal(t, true, got["applied"])
}
