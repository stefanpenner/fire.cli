package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stefanpenner/fire.cli/internal/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the whole read stack end-to-end — cobra command →
// firewalla.Client → FakeTransport — by feeding the same anonymized fixtures
// the parser tests use as raw command stdout. Unlike the fakeClient tests
// (which stub the typed client), these prove the real transport command,
// parser, and renderer agree.

// fwFixture loads a fixture from the firewalla package's testdata.
func fwFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "internal", "firewalla", "testdata", name))
	require.NoError(t, err)
	return string(b)
}

// execReal runs the CLI with a real firewalla.Client over the given transport.
func execReal(t *testing.T, ft *transport.FakeTransport, args ...string) (string, string, error) {
	t.Helper()
	var out, errBuf bytes.Buffer
	app := &App{
		Out:        &out,
		Err:        &errBuf,
		Client:     firewalla.New(ft),
		Now:        func() time.Time { return time.Unix(1700000100, 0) },
		ConfigPath: filepath.Join(t.TempDir(), "no-config.json"),
	}
	root := NewRootCmd(app)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errBuf.String(), err
}

func TestE2E_Devices(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("host:mac:", transport.Result{Stdout: fwFixture(t, "devices.txt")})
	out, _, err := execReal(t, ft, "devices")
	require.NoError(t, err)
	assert.Contains(t, out, "Example Hot")
	assert.Contains(t, out, "192.0.2.")
}

func TestE2E_Rules(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("policy:[0-9]", transport.Result{Stdout: fwFixture(t, "rules.txt")})
	out, _, err := execReal(t, ft, "rules", "--all")
	require.NoError(t, err)
	assert.Contains(t, out, "ads.example.net")
}

func TestE2E_Features(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("hgetall policy:system", transport.Result{Stdout: fwFixture(t, "features.txt")})
	out, _, err := execReal(t, ft, "features")
	require.NoError(t, err)
	assert.Contains(t, out, "Ad Block") // adblock key → friendly name
}

func TestE2E_Networks(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("hgetall sys:network:info", transport.Result{Stdout: fwFixture(t, "network_info.txt")})
	out, _, err := execReal(t, ft, "networks")
	require.NoError(t, err)
	assert.Contains(t, out, "Home")
}

func TestE2E_Alarms(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("zrevrange alarm_active", transport.Result{Stdout: fwFixture(t, "alarms.txt")})
	out, _, err := execReal(t, ft, "alarms")
	require.NoError(t, err)
	assert.Contains(t, out, "Example") // anonymized device name in an alarm
}

func TestE2E_WAN(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("wan/connectivity", transport.Result{Stdout: fwFixture(t, "wan_stream.txt")})
	out, _, err := execReal(t, ft, "wan")
	require.NoError(t, err)
	assert.Contains(t, out, "ISP-A")
}

func TestE2E_Data(t *testing.T) {
	// The data command issues two commands: the usage script and a networks
	// lookup to resolve WAN uuid→name.
	ft := transport.NewFake("pi@test").
		OnMatch("sys:data:plan", transport.Result{Stdout: fwFixture(t, "data_usage.txt")}).
		OnMatch("hgetall sys:network:info", transport.Result{Stdout: fwFixture(t, "network_info.txt")})
	out, _, err := execReal(t, ft, "data")
	require.NoError(t, err)
	assert.Contains(t, out, "plan") // summary line "used … of … plan"
}

func TestE2E_Top(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("sumflow:*", transport.Result{Stdout: fwFixture(t, "top_talkers.txt")})
	out, _, err := execReal(t, ft, "top")
	require.NoError(t, err)
	assert.Contains(t, out, "AA:BB:CC:DD:EE:01") // ranked first
}

func TestE2E_DevicesJSON(t *testing.T) {
	ft := transport.NewFake("pi@test").
		OnMatch("host:mac:", transport.Result{Stdout: fwFixture(t, "devices.txt")})
	out, _, err := execReal(t, ft, "devices", "--json")
	require.NoError(t, err)
	assert.Contains(t, out, `"mac"`) // valid JSON array of devices
}
