package tui

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update golden files")

// ansiRE matches SGR escape sequences (color/bold/reverse), stripped so the
// golden snapshots are stable regardless of the terminal color profile.
var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// assertGolden compares got against testdata/<name>.golden, regenerating it
// when -update is passed (go test ./internal/tui -update).
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden file %s; run: go test ./internal/tui -update", path)
	require.Equal(t, string(want), got)
}

// goldenDS is the deterministic data set behind every snapshot.
func goldenDS() *fakeDS {
	return &fakeDS{
		devices: sampleDevices(),
		rules:   sampleRules(),
		alarms:  sampleAlarms(),
		peers: []firewalla.Peer{
			{Label: "video.example.com", Kind: "internet", Download: 409600, Upload: 2048},
			{PeerMAC: "AA:BB:CC:DD:EE:02", Kind: "device", Download: 1024},
		},
		networks: []firewalla.Network{
			{Name: "Home", Type: "lan", Subnet: "192.0.2.0/24", Interface: "br0"},
			{Name: "IoT", Type: "lan", VLANID: 2001, Subnet: "192.0.2.64/26", Interface: "eth2.2001"},
		},
		wans: []firewalla.WAN{
			{Name: "ISP-A", Interface: "eth0", Role: "primary", Active: true, Carrier: true, Ping: true, DNS: true},
			{Name: "ISP-B", Interface: "eth3", Role: "standby", Carrier: false},
		},
		data: firewalla.DataUsageReport{
			PlanTotal: 1000000000000,
			ResetDay:  1,
			WANs:      []firewalla.WANUsage{{UUID: "u-1", Upload: 1024, Download: 1048576}},
		},
	}
}

// goldenModel returns a plain-styled, fixed-size model with devices loaded.
func goldenModel() Model {
	m := NewModel(goldenDS(), fixedNow).WithColor(false)
	m.width, m.height = 80, 24
	nm, _ := m.Update(devicesMsg{devices: m.ds.(*fakeDS).devices})
	return nm.(Model)
}

func TestGolden_Views(t *testing.T) {
	ds := goldenDS()

	// Each case switches to a view, delivers its (deterministic) load result,
	// then snapshots the rendered, ANSI-stripped frame.
	cases := []struct {
		name string
		key  string
		msg  tea.Msg
	}{
		{"devices", "1", devicesMsg{devices: ds.devices}},
		{"rules", "2", rulesMsg{rules: ds.rules}},
		{"alarms", "3", alarmsMsg{alarms: ds.alarms}},
		{"networks", "4", networksMsg{networks: ds.networks}},
		{"wan", "5", wansMsg{wans: ds.wans}},
		{"data", "6", dataMsg{report: ds.data, names: map[string]string{"u-1": "ISP-A"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := goldenModel()
			nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(c.key)})
			nm, _ = nm.(Model).Update(c.msg)
			assertGolden(t, c.name, stripANSI(nm.(Model).View()))
		})
	}
}

func TestGolden_DetailPane(t *testing.T) {
	ds := goldenDS()
	m := goldenModel()
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open detail for selected device
	nm, _ = nm.(Model).Update(detailMsg{
		mac:   "AA:BB:CC:DD:EE:03",
		peers: ds.peers,
		rules: rulesForMAC(ds.rules, "AA:BB:CC:DD:EE:03"),
	})
	assertGolden(t, "detail", stripANSI(nm.(Model).View()))
}

func TestGolden_HelpModal(t *testing.T) {
	m := goldenModel()
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	assertGolden(t, "help", stripANSI(nm.(Model).View()))
}

func TestGolden_ConfirmBar(t *testing.T) {
	m := goldenModel()
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")}) // stage block
	assertGolden(t, "confirm", stripANSI(nm.(Model).View()))
}
