package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/stefanpenner/fire.cli/internal/firewalla"
)

func newBlockCmd(app *App) *cobra.Command {
	var (
		confirm bool
		forDur  time.Duration
	)
	cmd := &cobra.Command{
		Use:               "block [device]",
		Short:             "Block a device's internet access (by name, MAC, or IP)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: app.completeDevice,
		RunE: func(c *cobra.Command, args []string) error {
			idx := loadDevices(c.Context(), app)
			mac, err := resolveOrPick(app, idx, args, "Block which device?")
			if err != nil {
				return err
			}
			if mac == "" {
				return nil // cancelled
			}
			label := idx.name(mac)
			action := fmt.Sprintf("block %s (%s)", label, mac)
			if forDur > 0 {
				action += fmt.Sprintf(" for %s", forDur)
			}
			if !app.confirmed(confirm, action) {
				return nil
			}
			pid, err := app.Client.CreateRule(c.Context(), firewalla.RuleSpec{
				Action: "block", Type: "mac", Target: mac,
				Notes:     "via fire cli",
				ExpireSec: int(forDur.Seconds()),
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(app.Out, "blocked %s (rule %s)\n", label, pid)
			return nil
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the change (without it, only prints what would happen)")
	cmd.Flags().DurationVar(&forDur, "for", 0, "auto-expire the block after this duration (e.g. 1h)")
	return cmd
}

func newUnblockCmd(app *App) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:               "unblock [device]",
		Short:             "Remove a device's block (by name, MAC, or IP)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: app.completeDevice,
		RunE: func(c *cobra.Command, args []string) error {
			idx := loadDevices(c.Context(), app)
			mac, err := resolveOrPick(app, idx, args, "Unblock which device?")
			if err != nil {
				return err
			}
			if mac == "" {
				return nil // cancelled
			}
			label := idx.name(mac)
			if !app.confirmed(confirm, fmt.Sprintf("unblock %s (%s)", label, mac)) {
				return nil
			}
			n, err := app.Client.DeleteMatching(c.Context(), firewalla.RuleSpec{
				Action: "block", Type: "mac", Target: mac,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(app.Out, "unblocked %s (removed %d rule(s))\n", label, n)
			return nil
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the change (without it, only prints what would happen)")
	return cmd
}
