package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDataCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "data",
		Aliases: []string{"usage", "plan"},
		Short:   "Show data-plan usage this period (per WAN, vs plan limit)",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			report, err := app.Client.DataUsage(c.Context())
			if err != nil {
				return err
			}
			// Resolve WAN uuid → friendly name via the network list (best effort).
			names := map[string]string{}
			if nets, nerr := app.Client.ListNetworks(c.Context()); nerr == nil {
				for _, n := range nets {
					if n.UUID != "" {
						names[n.UUID] = n.Name
					}
				}
			}

			rows := make([][]string, 0, len(report.WANs)+1)
			for _, w := range report.WANs {
				name := names[w.UUID]
				if name == "" {
					name = w.UUID
				}
				rows = append(rows, []string{
					name, humanizeBytes(w.Upload), humanizeBytes(w.Download), humanizeBytes(w.Bytes()),
				})
			}

			if !app.JSON {
				total := report.Total()
				summary := fmt.Sprintf("used %s", humanizeBytes(total))
				if report.PlanTotal > 0 {
					pct := float64(total) / float64(report.PlanTotal) * 100
					summary = fmt.Sprintf("used %s of %s plan (%.1f%%), resets day %d",
						humanizeBytes(total), humanizeBytes(report.PlanTotal), pct, report.ResetDay)
				}
				fmt.Fprintln(app.Out, summary)
			}
			return app.output(
				[]string{"wan", "up", "down", "total"},
				rows, report,
			)
		},
	}
	return cmd
}
