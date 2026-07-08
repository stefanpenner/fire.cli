package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

func TestLoad_Missing(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	require.NoError(t, err, "a missing file is not an error")
	assert.Empty(t, c.Boxes)
}

func TestLoad_Malformed(t *testing.T) {
	c, err := Load(write(t, "{not json"))
	require.Error(t, err, "malformed config surfaces an error to warn on")
	assert.Empty(t, c.DefaultBox, "and yields the zero config to fall back on")
}

func TestResolveHost_Precedence(t *testing.T) {
	c := Config{
		DefaultBox: "home",
		Host:       "pi@bare",
		Boxes: map[string]Box{
			"home":  {Host: "pi@fire.walla"},
			"cabin": {Host: "pi@cabin.lan"},
		},
	}
	// Explicit box wins.
	h, err := c.ResolveHost("cabin", "fallback")
	require.NoError(t, err)
	assert.Equal(t, "pi@cabin.lan", h)
	// No box → default_box.
	h, err = c.ResolveHost("", "fallback")
	require.NoError(t, err)
	assert.Equal(t, "pi@fire.walla", h)
	// Unknown box → error (don't silently use the wrong box).
	_, err = c.ResolveHost("ghost", "fallback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown box")
}

func TestResolveHost_BareAndFallback(t *testing.T) {
	// No boxes, no default: the bare host.
	h, err := Config{Host: "pi@bare"}.ResolveHost("", "fallback")
	require.NoError(t, err)
	assert.Equal(t, "pi@bare", h)
	// Empty config: the fallback.
	h, err = Config{}.ResolveHost("", "pi@fire.walla")
	require.NoError(t, err)
	assert.Equal(t, "pi@fire.walla", h)
}

func TestTimeoutOr(t *testing.T) {
	assert.Equal(t, 5*time.Second, Config{}.TimeoutOr(5*time.Second))
	assert.Equal(t, 90*time.Second, Config{Timeout: "90s"}.TimeoutOr(5*time.Second))
	assert.Equal(t, 5*time.Second, Config{Timeout: "garbage"}.TimeoutOr(5*time.Second))
	assert.Equal(t, 5*time.Second, Config{Timeout: "-3s"}.TimeoutOr(5*time.Second))
}

// FuzzLoad: parsing an arbitrary config file must never panic.
func FuzzLoad(f *testing.F) {
	f.Add(`{"default_box":"home","boxes":{"home":{"host":"pi@x"}}}`)
	f.Add(`{`)
	f.Add(``)
	f.Add(`{"timeout":"30s"}`)
	f.Add("\x00\xff")
	f.Fuzz(func(t *testing.T, body string) {
		p := filepath.Join(t.TempDir(), "c.json")
		if os.WriteFile(p, []byte(body), 0o600) != nil {
			return
		}
		c, _ := Load(p)
		_, _ = c.ResolveHost("", "fallback")
		_ = c.TimeoutOr(time.Second)
	})
}
