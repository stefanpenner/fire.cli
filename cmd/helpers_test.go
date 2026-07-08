package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHumanizeDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{3 * time.Hour, "3h"},
		{50 * time.Hour, "2d"},
		{-45 * time.Second, "45s"}, // negative magnitudes are absolute
	}
	for _, c := range cases {
		assert.Equal(t, c.want, humanizeDuration(c.d), "%s", c.d)
	}
}

func TestHumanizeBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{4096, "4.0 KB"},
		{1572864, "1.5 MB"},
		{13207024434, "12.3 GB"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, humanizeBytes(c.n), "%d", c.n)
	}
}

func TestCapitalize(t *testing.T) {
	assert.Equal(t, "", capitalize(""))
	assert.Equal(t, "Block", capitalize("block"))
	assert.Equal(t, "Pause", capitalize("pause"))
	// Must not byte-underflow on a non-lowercase-ASCII first rune.
	assert.Equal(t, "1x", capitalize("1x"))
	assert.Equal(t, "Éa", capitalize("éa"))
	assert.NotPanics(t, func() { capitalize("\x00x") })
}

func TestDeviceIndex_ResolveAndName(t *testing.T) {
	client := &fakeClient{devices: []firewalla.Device{
		{MAC: "AA:BB:CC:DD:EE:01", Name: "Living Room TV", IP: "192.0.2.10"},
		{MAC: "AA:BB:CC:DD:EE:02", Name: "Example Phone", IP: "192.0.2.20"},
	}}
	app := &App{Client: client}
	idx := loadDevices(context.Background(), app)

	// MAC passes through (upper-cased); IP and exact/substring name all resolve.
	assert.Equal(t, "AA:BB:CC:DD:EE:01", idx.resolveMAC("aa:bb:cc:dd:ee:01"))
	assert.Equal(t, "AA:BB:CC:DD:EE:01", idx.resolveMAC("192.0.2.10"))
	assert.Equal(t, "AA:BB:CC:DD:EE:02", idx.resolveMAC("Example Phone"))
	assert.Equal(t, "AA:BB:CC:DD:EE:01", idx.resolveMAC("living room"), "substring match")
	assert.Equal(t, "", idx.resolveMAC("nonexistent"))

	// name renders the friendly label, falling back to the MAC.
	assert.Equal(t, "Example Phone", idx.name("AA:BB:CC:DD:EE:02"))
	assert.Equal(t, "AA:BB:CC:DD:EE:99", idx.name("AA:BB:CC:DD:EE:99"))
}

func TestLoadRulePicks(t *testing.T) {
	client := &fakeClient{rules: []firewalla.Rule{
		{ID: "10", Action: "block", Type: "dns", Target: "ads.example.net", Disabled: false},
		{ID: "11", Action: "allow", Type: "mac", Target: "AA:BB:CC:DD:EE:01", Disabled: true},
	}}
	picks, err := loadRulePicks(context.Background(), &App{Client: client})
	require.NoError(t, err)
	require.Len(t, picks, 2)
	assert.Equal(t, "10", picks[0].id)
	assert.Contains(t, picks[0].display, "ads.example.net")
	assert.Contains(t, picks[0].display, "enabled")
	assert.Contains(t, picks[1].display, "disabled") // rule 11 is disabled
}

// selectIndex's empty-list guard returns a clear error without touching the
// interactive picker (which would need a TTY).
func TestSelectIndex_EmptyList(t *testing.T) {
	app := &App{Err: nil}
	i, err := app.selectIndex("rule", "pick", nil)
	require.Error(t, err)
	assert.Equal(t, -1, i)
	assert.Contains(t, err.Error(), "no rules to choose from")
}
