package firewalla

import (
	"context"
	"testing"

	"github.com/stefanpenner/fire.cli/internal/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the node scripts the mutation methods generate and the
// parsing of their results, using a FakeTransport that records the command and
// returns a canned stdout. They are the safety net for the box-side wiring
// (PolicyManager2 / HostManager / AlarmManager2) without needing a real box.

func TestCreateRule_BuildsPolicyAndParsesPID(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("checkAndSaveAsync", transport.Result{Stdout: `{"pid":"123","exists":"no"}`})
	pid, err := New(ft).CreateRule(context.Background(), RuleSpec{
		Action: "block", Type: "mac", Target: "AA:BB:CC:DD:EE:01",
	})
	require.NoError(t, err)
	assert.Equal(t, "123", pid)

	cmd := ft.Commands[0]
	assert.Contains(t, cmd, "PolicyManager2.js")
	assert.Contains(t, cmd, "checkAndSaveAsync")
	assert.Contains(t, cmd, `"action":"block"`)
	assert.Contains(t, cmd, `"target":"AA:BB:CC:DD:EE:01"`)
	assert.Contains(t, cmd, `"direction":"bidirection"`, "direction defaults to bidirection")
}

func TestCreateRule_TransportErrorPropagates(t *testing.T) {
	// No programmed response → the fake transport errors → CreateRule errors.
	_, err := New(transport.NewFake("pi@test")).
		CreateRule(context.Background(), RuleSpec{Action: "block", Type: "mac", Target: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pi@test")
}

func TestDeleteMatching_ParsesRemovedCount(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("getSamePolicies", transport.Result{Stdout: `{"removed":2}`})
	n, err := New(ft).DeleteMatching(context.Background(), RuleSpec{
		Action: "block", Type: "mac", Target: "AA:BB:CC:DD:EE:01",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Contains(t, ft.Commands[0], "getSamePolicies")
}

func TestDeleteRule_PassesPID(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("disableAndDeletePolicy", transport.Result{Stdout: "ok"})
	require.NoError(t, New(ft).DeleteRule(context.Background(), "42"))
	assert.Contains(t, ft.Commands[0], "FIRE_PID='42'")
	assert.Contains(t, ft.Commands[0], "getPolicy")
}

func TestSetRuleDisabled_TogglesFlag(t *testing.T) {
	for _, tc := range []struct {
		disabled bool
		want     string
	}{{true, "FIRE_DISABLE='1'"}, {false, "FIRE_DISABLE='0'"}} {
		ft := transport.NewFake("pi@test").OnMatch("getPolicy", transport.Result{Stdout: "ok"})
		require.NoError(t, New(ft).SetRuleDisabled(context.Background(), "7", tc.disabled))
		assert.Contains(t, ft.Commands[0], tc.want)
		assert.Contains(t, ft.Commands[0], "FIRE_PID='7'")
	}
}

func TestSetFeature_UsesHostManagerAndEnv(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("setPolicyAsync", transport.Result{Stdout: `{"key":"adblock","state":true}`})
	require.NoError(t, New(ft).SetFeature(context.Background(), "adblock", true))

	cmd := ft.Commands[0]
	assert.Contains(t, cmd, "HostManager.js")
	assert.Contains(t, cmd, "setPolicyAsync")
	assert.Contains(t, cmd, "FIRE_KEY='adblock'")
	assert.Contains(t, cmd, "FIRE_ON='1'")
}

func TestSetFeature_DisableSetsZero(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("setPolicyAsync", transport.Result{Stdout: "{}"})
	require.NoError(t, New(ft).SetFeature(context.Background(), "vpn", false))
	assert.Contains(t, ft.Commands[0], "FIRE_ON='0'")
}

func TestArchiveAlarm_UsesAlarmManager(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("AlarmManager2", transport.Result{Stdout: "ok"})
	require.NoError(t, New(ft).ArchiveAlarm(context.Background(), "2297"))

	cmd := ft.Commands[0]
	assert.Contains(t, cmd, "AlarmManager2.js")
	assert.Contains(t, cmd, "archiveAlarm")
	assert.Contains(t, cmd, "FIRE_AID='2297'")
	assert.Contains(t, cmd, "FIRE_OP='archive'")
}

func TestDeleteAlarm_UsesRemoveAndDeleteOp(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("AlarmManager2", transport.Result{Stdout: "ok"})
	require.NoError(t, New(ft).DeleteAlarm(context.Background(), "55"))

	cmd := ft.Commands[0]
	assert.Contains(t, cmd, "removeAlarmAsync")
	assert.Contains(t, cmd, "FIRE_AID='55'")
	assert.Contains(t, cmd, "FIRE_OP='delete'")
}
