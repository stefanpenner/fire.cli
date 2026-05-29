package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stefanpenner/fire.cli/internal/firewalla"
)

// newTrafficCmd answers "what is this device talking to?" and, with a second
// argument or --with, "is A talking to B?" — traffic between anything, built on
// Firewalla's own sumflow rollups.
func newTrafficCmd(app *App) *cobra.Command {
	var with string
	cmd := &cobra.Command{
		Use:     "traffic <device> [peer]",
		Aliases: []string{"flows", "flow", "conns"},
		Short:   "Show who a device is talking to (internet domains and LAN devices)",
		Long: "Show the peers a device exchanged traffic with — internet destinations\n" +
			"by domain/IP and other LAN devices by name — newest rollup, by bytes.\n\n" +
			"<device> and [peer] accept a MAC, IP, or device name (run `fire devices`\n" +
			"to see them, or tab-complete). Give a peer (or --with) to filter to traffic\n" +
			"with that endpoint, e.g. between two devices.",
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: app.completeDevice,
		RunE: func(c *cobra.Command, args []string) error {
			idx := loadDevices(c.Context(), app)
			mac := idx.resolveMAC(args[0])
			if mac == "" {
				return fmt.Errorf("no device matches %q; run `fire devices` to list devices (name, IP, or MAC all work)", args[0])
			}
			if len(args) == 2 {
				with = args[1]
			}

			peers, err := app.Client.Traffic(c.Context(), mac)
			if err != nil {
				return err
			}
			peers = filterPeers(peers, with, idx)

			rows := make([][]string, 0, len(peers))
			for _, p := range peers {
				label := p.Label
				if p.Kind == "device" {
					label = idx.name(p.PeerMAC)
				}
				rows = append(rows, []string{
					label, p.Kind,
					humanizeBytes(p.Upload), humanizeBytes(p.Download), humanizeBytes(p.Bytes()),
				})
			}
			return app.output(
				[]string{"peer", "kind", "up", "down", "total"},
				rows, peers,
			)
		},
	}
	cmd.Flags().StringVar(&with, "with", "", "only traffic with this peer (device name/MAC/IP, or domain substring)")
	return cmd
}

// filterPeers narrows peers to those matching `with`: a device (by resolved
// MAC) or an internet destination (by label substring). Empty `with` is a no-op.
func filterPeers(peers []firewalla.Peer, with string, idx *deviceIndex) []firewalla.Peer {
	if with == "" {
		return peers
	}
	wantMAC := idx.resolveMAC(with)
	low := strings.ToLower(with)
	var out []firewalla.Peer
	for _, p := range peers {
		switch {
		case p.Kind == "device" && wantMAC != "" && strings.EqualFold(p.PeerMAC, wantMAC):
			out = append(out, p)
		case strings.Contains(strings.ToLower(p.Label), low):
			out = append(out, p)
		}
	}
	return out
}
