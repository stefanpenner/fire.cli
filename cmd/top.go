package cmd

import (
	"github.com/spf13/cobra"
)

// newTopCmd: fire top — rank devices by bandwidth used (top talkers).
func newTopCmd(app *App) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "top",
		Aliases: []string{"talkers", "bandwidth"},
		Short:   "Rank devices by bandwidth used (top talkers)",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			talkers, err := app.Client.TopTalkers(c.Context())
			if err != nil {
				return err
			}
			if limit > 0 && len(talkers) > limit {
				talkers = talkers[:limit]
			}
			idx := loadDevices(c.Context(), app)
			rows := make([][]string, 0, len(talkers))
			for _, t := range talkers {
				rows = append(rows, []string{
					idx.name(t.MAC), t.MAC,
					humanizeBytes(t.Download), humanizeBytes(t.Upload), humanizeBytes(t.Bytes()),
				})
			}
			return app.output([]string{"device", "mac", "down", "up", "total"}, rows, talkers)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum devices to show (0 = all)")
	return cmd
}
