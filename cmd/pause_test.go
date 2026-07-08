package cmd

import (
	"testing"

	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPause_RequiresConfirm(t *testing.T) {
	client := &fakeClient{devices: []firewalla.Device{{MAC: "AA:BB:CC:DD:EE:01", Name: "Phone"}}}
	_, errOut, err := exec(t, client, "pause", "Phone")
	require.NoError(t, err)
	assert.Contains(t, errOut, "would pause")
	assert.Empty(t, client.gotRuleSpec.Target, "must not mutate without --confirm")
}

func TestPause_CreatesBlockRule(t *testing.T) {
	client := &fakeClient{
		devices:   []firewalla.Device{{MAC: "AA:BB:CC:DD:EE:01", Name: "Phone"}},
		createPID: "321",
	}
	out, _, err := exec(t, client, "pause", "Phone", "--confirm")
	require.NoError(t, err)
	assert.Equal(t, "block", client.gotRuleSpec.Action)
	assert.Equal(t, "mac", client.gotRuleSpec.Type)
	assert.Equal(t, "AA:BB:CC:DD:EE:01", client.gotRuleSpec.Target)
	assert.Contains(t, out, "paused Phone")
}

func TestPause_ForSetsExpire(t *testing.T) {
	client := &fakeClient{devices: []firewalla.Device{{MAC: "AA:BB:CC:DD:EE:01", Name: "Phone"}}}
	_, _, err := exec(t, client, "pause", "Phone", "--for", "30m", "--confirm")
	require.NoError(t, err)
	assert.Equal(t, 1800, client.gotRuleSpec.ExpireSec)
}

func TestResume_RemovesBlock(t *testing.T) {
	client := &fakeClient{devices: []firewalla.Device{{MAC: "AA:BB:CC:DD:EE:01", Name: "Phone"}}}
	out, _, err := exec(t, client, "resume", "Phone", "--confirm")
	require.NoError(t, err)
	assert.Equal(t, "AA:BB:CC:DD:EE:01", client.gotRuleSpec.Target)
	assert.Contains(t, out, "resumed Phone")
}

func TestPause_Completion(t *testing.T) {
	client := &fakeClient{devices: []firewalla.Device{{Name: "Phone", IP: "192.0.2.10"}}}
	out, _, err := exec(t, client, "__complete", "pause", "")
	require.NoError(t, err)
	assert.Contains(t, out, "Phone")
}
