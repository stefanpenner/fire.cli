package transport

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSSHArgs(t *testing.T) {
	args := buildSSHArgs("pi@fire", nil, "redis-cli ping")

	// Non-interactive, fail-fast defaults must always be present.
	assert.Contains(t, args, "BatchMode=yes")
	assert.Contains(t, args, "ConnectTimeout=8")

	// Host and the remote command must be the final two positional args,
	// in that order, so the command is never interpreted as an ssh option.
	require.GreaterOrEqual(t, len(args), 2)
	assert.Equal(t, "pi@fire", args[len(args)-2])
	assert.Equal(t, "redis-cli ping", args[len(args)-1])
}

func TestBuildSSHArgs_ExtraOptionsPrecedeHost(t *testing.T) {
	args := buildSSHArgs("pi@fire", []string{"-p", "2222"}, "uptime")
	assert.Equal(t, []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=8",
		"-p", "2222",
		"pi@fire", "uptime",
	}, args)
}

func TestSSHTransport_Run_MapsExecOutput(t *testing.T) {
	var gotName string
	var gotArgs []string
	tr := NewSSH("pi@fire")
	tr.exec = func(_ context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		gotName, gotArgs = name, args
		return []byte("PONG\n"), []byte(""), 0, nil
	}

	res, err := tr.Run(context.Background(), "redis-cli ping")

	require.NoError(t, err)
	assert.Equal(t, "ssh", gotName)
	assert.Equal(t, "pi@fire", gotArgs[len(gotArgs)-2])
	assert.Equal(t, "PONG\n", res.Stdout)
	assert.Equal(t, 0, res.ExitCode)
}

func TestSSHTransport_Run_PropagatesExitCodeAndError(t *testing.T) {
	tr := NewSSH("pi@fire")
	tr.exec = func(context.Context, string, ...string) ([]byte, []byte, int, error) {
		return nil, []byte("boom"), 1, errors.New("exit status 1")
	}

	res, err := tr.Run(context.Background(), "false")

	require.Error(t, err)
	assert.Equal(t, 1, res.ExitCode)
	assert.Equal(t, "boom", res.Stderr)
}

func TestSSHTransport_Host(t *testing.T) {
	assert.Equal(t, "pi@fire", NewSSH("pi@fire").Host())
}

func TestFakeTransport_RecordsAndMatchesExactCommand(t *testing.T) {
	fake := NewFake("pi@test").
		On("redis-cli ping", Result{Stdout: "PONG\n"})

	res, err := fake.Run(context.Background(), "redis-cli ping")

	require.NoError(t, err)
	assert.Equal(t, "PONG\n", res.Stdout)
	assert.Equal(t, []string{"redis-cli ping"}, fake.Commands)
	assert.Equal(t, "pi@test", fake.Host())
}

func TestFakeTransport_SubstringMatcher(t *testing.T) {
	fake := NewFake("pi@test").
		OnMatch("host:mac:", Result{Stdout: "hash"})

	res, err := fake.Run(context.Background(), "redis-cli type host:mac:AA")

	require.NoError(t, err)
	assert.Equal(t, "hash", res.Stdout)
}

func TestFakeTransport_UnmatchedCommandErrors(t *testing.T) {
	fake := NewFake("pi@test")

	_, err := fake.Run(context.Background(), "redis-cli unknown")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis-cli unknown")
}

func TestFakeTransport_ExactBeatsSubstring(t *testing.T) {
	fake := NewFake("pi@test").
		OnMatch("redis-cli", Result{Stdout: "substring"}).
		On("redis-cli ping", Result{Stdout: "exact"})

	res, _ := fake.Run(context.Background(), "redis-cli ping")
	assert.Equal(t, "exact", res.Stdout)
}
