package cmd

import (
	"fmt"

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
	cmd.AddCommand(newAlarmArchiveCmd(app), newAlarmRmCmd(app))
	return cmd
}

// newAlarmArchiveCmd: fire alarms archive [id] — acknowledge/dismiss an alarm.
func newAlarmArchiveCmd(app *App) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:               "archive [id]",
		Aliases:           []string{"ack", "dismiss", "ignore"},
		Short:             "Acknowledge an alarm, moving it to the archive (omit id to pick)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: app.completeAlarm,
		RunE: func(c *cobra.Command, args []string) error {
			id, err := app.resolveOrPickAlarm(c.Context(), args, "Archive which alarm?")
			if err != nil {
				return err
			}
			if id == "" {
				return nil // cancelled
			}
			if !app.confirmed(confirm, fmt.Sprintf("archive alarm %s", id)) {
				return nil
			}
			if err := app.Client.ArchiveAlarm(c.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(app.Out, "archived alarm %s\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the change (without it, only prints what would happen)")
	return cmd
}

// newAlarmRmCmd: fire alarms rm [id] — delete an alarm entirely.
func newAlarmRmCmd(app *App) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:               "rm [id]",
		Aliases:           []string{"delete", "del"},
		Short:             "Delete an alarm entirely (omit id to pick)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: app.completeAlarm,
		RunE: func(c *cobra.Command, args []string) error {
			id, err := app.resolveOrPickAlarm(c.Context(), args, "Delete which alarm?")
			if err != nil {
				return err
			}
			if id == "" {
				return nil // cancelled
			}
			if !app.confirmed(confirm, fmt.Sprintf("delete alarm %s", id)) {
				return nil
			}
			if err := app.Client.DeleteAlarm(c.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(app.Out, "deleted alarm %s\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the change (without it, only prints what would happen)")
	return cmd
}
