package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newFeaturesCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "features",
		Aliases: []string{"feature"},
		Short:   "Show box features and whether each is on (ad block, VPN, DoH, …)",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			feats, err := app.Client.ListFeatures(c.Context())
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(feats))
			for _, f := range feats {
				rows = append(rows, []string{f.Name, onOff(f.Enabled)})
			}
			return app.output([]string{"feature", "state"}, rows, feats)
		},
	}
	cmd.AddCommand(newFeatureToggleCmd(app, true), newFeatureToggleCmd(app, false))
	return cmd
}

// newFeatureToggleCmd builds the enable or disable subcommand for a feature.
func newFeatureToggleCmd(app *App, enable bool) *cobra.Command {
	verb := "disable"
	if enable {
		verb = "enable"
	}
	var confirm bool
	cmd := &cobra.Command{
		Use:               verb + " [feature]",
		Short:             verb + " a box feature (omit to pick interactively)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: app.completeFeature,
		RunE: func(c *cobra.Command, args []string) error {
			f, ok, err := app.resolveOrPickFeature(c.Context(), args, "Which feature to "+verb+"?")
			if err != nil {
				return err
			}
			if !ok {
				return nil // cancelled
			}
			if f.Enabled == enable {
				fmt.Fprintf(app.Out, "%s is already %s\n", f.Name, onOff(enable))
				return nil
			}
			if !app.confirmed(confirm, fmt.Sprintf("%s %s", verb, f.Name)) {
				return nil
			}
			if err := app.Client.SetFeature(c.Context(), f.Key, enable); err != nil {
				return err
			}
			fmt.Fprintf(app.Out, "%sd %s\n", verb, f.Name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the change (without it, only prints what would happen)")
	return cmd
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
