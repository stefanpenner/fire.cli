package cmd

import (
	"github.com/spf13/cobra"
)

func newWANCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wan",
		Short: "Show internet uplinks (dual-WAN role and live health)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			wans, err := app.Client.ListWANs(c.Context())
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(wans))
			for _, w := range wans {
				rows = append(rows, []string{
					w.Name, w.Interface, w.Role,
					yesNo(w.Active), w.Mode,
					health(w.Carrier, w.Ping, w.DNS),
				})
			}
			return app.output(
				[]string{"name", "interface", "role", "in use", "mode", "health"},
				rows, wans,
			)
		},
	}
	return cmd
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// health summarizes the carrier/ping/dns checks as a compact label.
func health(carrier, ping, dns bool) string {
	switch {
	case !carrier:
		return "down (no carrier)"
	case ping && dns:
		return "healthy"
	case ping || dns:
		return "degraded"
	default:
		return "no connectivity"
	}
}
