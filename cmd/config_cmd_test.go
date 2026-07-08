package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runWithConfig runs the CLI with a given config file and args, returning the
// (possibly config-resolved) App.
func runWithConfig(t *testing.T, cfgBody string, args ...string) (*App, error) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgBody), 0o600))
	var out, errBuf bytes.Buffer
	app := &App{
		Out:        &out,
		Err:        &errBuf,
		Client:     &fakeClient{},
		Now:        func() time.Time { return time.Unix(1700000100, 0) },
		ConfigPath: cfgPath,
	}
	root := NewRootCmd(app)
	root.SetArgs(args)
	return app, root.Execute()
}

// --box resolves the host from the config (version needs no network).
func TestConfig_BoxResolvesHost(t *testing.T) {
	app, err := runWithConfig(t, `{"boxes":{"cabin":{"host":"pi@cabin.lan"}}}`, "--box", "cabin", "devices")
	require.NoError(t, err)
	assert.Equal(t, "pi@cabin.lan", app.Host)
}

// default_box is used when no --box is given.
func TestConfig_DefaultBox(t *testing.T) {
	app, err := runWithConfig(t, `{"default_box":"home","boxes":{"home":{"host":"pi@home.lan"}}}`, "devices")
	require.NoError(t, err)
	assert.Equal(t, "pi@home.lan", app.Host)
}

// An explicit --host always overrides a box.
func TestConfig_HostFlagOverridesBox(t *testing.T) {
	app, err := runWithConfig(t, `{"boxes":{"cabin":{"host":"pi@cabin.lan"}}}`,
		"--box", "cabin", "--host", "pi@explicit", "devices")
	require.NoError(t, err)
	assert.Equal(t, "pi@explicit", app.Host)
}

// An unknown box is a clear error.
func TestConfig_UnknownBoxErrors(t *testing.T) {
	_, err := runWithConfig(t, `{"boxes":{"home":{"host":"pi@home.lan"}}}`, "--box", "ghost", "devices")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown box")
}

// timeout from config applies when --timeout isn't passed.
func TestConfig_TimeoutFromConfig(t *testing.T) {
	app, err := runWithConfig(t, `{"host":"pi@x","timeout":"90s"}`, "devices")
	require.NoError(t, err)
	assert.Equal(t, 90*time.Second, app.Timeout)
}
