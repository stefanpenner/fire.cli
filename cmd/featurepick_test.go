package cmd

import (
	"testing"

	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func featClient() *fakeClient {
	return &fakeClient{features: []firewalla.Feature{
		{Key: "adblock", Name: "Ad Block", Enabled: false},
		{Key: "vpn", Name: "VPN Server", Enabled: true},
	}}
}

func TestFeaturesEnable_ByKey(t *testing.T) {
	client := featClient()
	out, _, err := exec(t, client, "features", "enable", "adblock", "--confirm")
	require.NoError(t, err)
	assert.True(t, client.gotFeatSet)
	assert.Equal(t, "adblock", client.gotFeatKey)
	assert.True(t, client.gotFeatOn)
	assert.Contains(t, out, "Ad Block")
}

// A feature is selectable by its friendly name too, case-insensitively.
func TestFeaturesDisable_ByName(t *testing.T) {
	client := featClient()
	_, _, err := exec(t, client, "features", "disable", "vpn server", "--confirm")
	require.NoError(t, err)
	assert.Equal(t, "vpn", client.gotFeatKey)
	assert.False(t, client.gotFeatOn)
}

func TestFeaturesEnable_RequiresConfirm(t *testing.T) {
	client := featClient()
	_, errOut, err := exec(t, client, "features", "enable", "adblock")
	require.NoError(t, err)
	assert.Contains(t, errOut, "would enable")
	assert.False(t, client.gotFeatSet, "must not mutate without --confirm")
}

func TestFeaturesEnable_UnknownFeature(t *testing.T) {
	_, _, err := exec(t, featClient(), "features", "enable", "nope", "--confirm")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no feature")
}

func TestFeaturesEnable_NoArgNonInteractive(t *testing.T) {
	_, _, err := exec(t, featClient(), "features", "enable")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "feature required")
}

func TestFeatures_Completion(t *testing.T) {
	out, _, err := exec(t, featClient(), "__complete", "features", "enable", "")
	require.NoError(t, err)
	assert.Contains(t, out, "adblock")
	assert.Contains(t, out, "Ad Block")
}
