package cmd

import (
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newDevicesCmd(app *App) *cobra.Command {
	var onlineWithin time.Duration
	cmd := &cobra.Command{
		Use:     "devices",
		Aliases: []string{"device", "dev", "ls"},
		Short:   "List devices known to the Firewalla",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			devices, err := app.Client.ListDevices(c.Context())
			if err != nil {
				return err
			}
			// Online first, then most-recently-active.
			now := app.now()
			sort.SliceStable(devices, func(i, j int) bool {
				return devices[i].LastActive.After(devices[j].LastActive)
			})

			rows := make([][]string, 0, len(devices))
			for _, d := range devices {
				status := "offline"
				if d.SeenWithin(onlineWithin, now) {
					status = "online"
				}
				rows = append(rows, []string{
					d.Name, d.IP, d.MAC, d.Type, lastSeen(d.LastActive, now), status,
				})
			}
			return app.output(
				[]string{"name", "ip", "mac", "type", "last seen", "status"},
				rows, devices,
			)
		},
	}
	cmd.Flags().DurationVar(&onlineWithin, "online-within", 5*time.Minute,
		"treat a device as online if seen within this window")
	return cmd
}

// lastSeen renders a device's last-active time relative to now.
func lastSeen(t, now time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return humanizeDuration(now.Sub(t)) + " ago"
}
