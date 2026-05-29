package cmd

import (
	"github.com/spf13/cobra"
)

func newRulesCmd(app *App) *cobra.Command {
	var includeDisabled bool
	cmd := &cobra.Command{
		Use:     "rules",
		Aliases: []string{"rule", "policy", "policies"},
		Short:   "List firewall rules (block/allow policies) on the Firewalla",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			rules, err := app.Client.ListRules(c.Context())
			if err != nil {
				return err
			}
			now := app.now()
			rows := make([][]string, 0, len(rules))
			out := rules[:0:0] // copy slice header so JSON reflects the filter too
			for _, r := range rules {
				if r.Disabled && !includeDisabled {
					continue
				}
				out = append(out, r)
				state := "enabled"
				if r.Disabled {
					state = "disabled"
				}
				rows = append(rows, []string{
					r.ID, r.Action, r.Type, r.Target, r.Direction,
					r.Scope, lastSeen(r.Created, now), state,
				})
			}
			return app.output(
				[]string{"id", "action", "type", "target", "direction", "scope", "created", "state"},
				rows, out,
			)
		},
	}
	cmd.Flags().BoolVar(&includeDisabled, "all", false, "include disabled rules")
	return cmd
}
