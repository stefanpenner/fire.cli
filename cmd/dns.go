package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newDNSCmd(app *App) *cobra.Command {
	dns := &cobra.Command{
		Use:   "dns",
		Short: "Inspect DNS activity recorded by the Firewalla",
	}
	dns.AddCommand(newDNSWhoCmd(app), newDNSDeviceCmd(app))
	return dns
}

// newDNSWhoCmd answers "which clients resolved this hostname?" — the question
// from the spa debug, where it revealed which device actually queried a domain.
func newDNSWhoCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "who <domain>",
		Short: "Show which clients resolved a domain (from Zeek dns logs)",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			resolvers, err := app.Client.WhoResolved(c.Context(), args[0])
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(resolvers))
			for _, r := range resolvers {
				rows = append(rows, []string{r.Client, strconv.Itoa(r.Count)})
			}
			return app.output([]string{"client", "queries"}, rows, resolvers)
		},
	}
}

func newDNSDeviceCmd(app *App) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:               "device <device>",
		Aliases:           []string{"dev"},
		Short:             "Show recent DNS lookups made by a device (by name, IP, or MAC)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: app.completeDevice,
		RunE: func(c *cobra.Command, args []string) error {
			idx := loadDevices(c.Context(), app)
			mac := idx.resolveMAC(args[0])
			if mac == "" {
				return fmt.Errorf("no device matches %q; run `fire devices` to list devices (name, IP, or MAC all work)", args[0])
			}
			queries, err := app.Client.DNSByDevice(c.Context(), mac, limit)
			if err != nil {
				return err
			}
			now := app.now()
			rows := make([][]string, 0, len(queries))
			for _, q := range queries {
				rows = append(rows, []string{
					lastSeen(q.Time, now), q.Domain, q.Client, strconv.Itoa(q.Count),
				})
			}
			return app.output([]string{"when", "domain", "client", "count"}, rows, queries)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum number of lookups to return")
	return cmd
}
