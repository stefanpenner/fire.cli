package cmd

import (
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
	return cmd
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
