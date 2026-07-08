package cmd

import (
	"testing"

	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Without an id and without a TTY (tests use buffers), rm/enable/disable must
// error clearly rather than trying to launch the picker.
func TestRulesRm_NoArgNonInteractive(t *testing.T) {
	_, _, err := exec(t, &fakeClient{}, "rules", "rm")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rule required")
}

func TestRulesDisable_NoArgNonInteractive(t *testing.T) {
	_, _, err := exec(t, &fakeClient{}, "rules", "disable")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rule required")
}

// Tab-completion offers rule ids annotated with their action/type/target so it
// doubles as discovery of valid values.
func TestRulesRm_Completion(t *testing.T) {
	client := &fakeClient{rules: []firewalla.Rule{
		{ID: "42", Action: "block", Type: "dns", Target: "ads.example.net"},
		{ID: "7", Action: "allow", Type: "mac", Target: "AA:BB:CC:DD:EE:01"},
	}}
	out, _, err := exec(t, client, "__complete", "rules", "rm", "")
	require.NoError(t, err)
	assert.Contains(t, out, "42")
	assert.Contains(t, out, "ads.example.net")
	assert.Contains(t, out, "7")
}

// An explicit id is still passed straight through, even for an id the local
// list doesn't know about (the box validates).
func TestRulesRm_ExplicitIDPassThrough(t *testing.T) {
	client := &fakeClient{}
	_, _, err := exec(t, client, "rules", "rm", "999", "--confirm")
	require.NoError(t, err)
	assert.Equal(t, "999", client.gotRuleID)
}
