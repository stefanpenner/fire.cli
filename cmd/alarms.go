package cmd

import (
	"github.com/spf13/cobra"
)

func newAlarmsCmd(app *App) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "alarms",
		Aliases: []string{"alarm", "alerts"},
		Short:   "Show recent security alarms (scans, new devices, abnormal activity)",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			alarms, err := app.Client.ListAlarms(c.Context(), limit)
			if err != nil {
				return err
			}
			now := app.now()
			rows := make([][]string, 0, len(alarms))
			for _, a := range alarms {
				rows = append(rows, []string{
					lastSeen(a.Time, now), a.Type, a.Device, a.Message,
				})
			}
			return app.output([]string{"when", "type", "device", "message"}, rows, alarms)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum number of alarms to return")
	return cmd
}
