package cmd

import (
	"testing"

	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlarmArchive_Confirm(t *testing.T) {
	client := &fakeClient{}
	out, _, err := exec(t, client, "alarms", "archive", "2297", "--confirm")
	require.NoError(t, err)
	assert.Equal(t, "2297", client.gotAlarmID)
	assert.Equal(t, "archive", client.gotAlarmOp)
	assert.Contains(t, out, "archived alarm 2297")
}

func TestAlarmArchive_RequiresConfirm(t *testing.T) {
	client := &fakeClient{}
	_, errOut, err := exec(t, client, "alarms", "ack", "2297")
	require.NoError(t, err)
	assert.Contains(t, errOut, "would archive")
	assert.Empty(t, client.gotAlarmID, "must not mutate without --confirm")
}

func TestAlarmRm_Confirm(t *testing.T) {
	client := &fakeClient{}
	out, _, err := exec(t, client, "alarms", "rm", "42", "--confirm")
	require.NoError(t, err)
	assert.Equal(t, "42", client.gotAlarmID)
	assert.Equal(t, "delete", client.gotAlarmOp)
	assert.Contains(t, out, "deleted alarm 42")
}

func TestAlarm_NoArgNonInteractive(t *testing.T) {
	_, _, err := exec(t, &fakeClient{}, "alarms", "archive")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alarm required")
}

func TestAlarm_Completion(t *testing.T) {
	client := &fakeClient{alarms: []firewalla.Alarm{
		{ID: "2297", Type: "Port Scan", Device: "Laptop"},
		{ID: "55", Type: "New Device", Device: "Phone"},
	}}
	out, _, err := exec(t, client, "__complete", "alarms", "rm", "")
	require.NoError(t, err)
	assert.Contains(t, out, "2297")
	assert.Contains(t, out, "Port Scan")
	assert.Contains(t, out, "55")
}
