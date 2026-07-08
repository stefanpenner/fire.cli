package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stefanpenner/fire.cli/internal/firewalla"
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
	cmd.AddCommand(newRuleAddCmd(app), newRuleRmCmd(app), newRuleToggleCmd(app, true), newRuleToggleCmd(app, false))
	return cmd
}

// newRuleAddCmd: fire rules add <block|allow> <type> <target>
func newRuleAddCmd(app *App) *cobra.Command {
	var (
		confirm   bool
		direction string
		notes     string
	)
	cmd := &cobra.Command{
		Use:   "add <block|allow> <type> <target>",
		Short: "Create a rule (e.g. add block dns ads.example.com)",
		Long: "Create and enforce a rule. <type> is one of dns, ip, net, mac,\n" +
			"category, country, … and <target> is the value to match.",
		Args: cobra.ExactArgs(3),
		RunE: func(c *cobra.Command, args []string) error {
			action, typ, target := args[0], args[1], args[2]
			if action != "block" && action != "allow" {
				return fmt.Errorf("action must be block or allow, got %q", action)
			}
			res := &mutationResult{Action: "rule.add", Target: target}
			if !app.beginMutation(confirm, fmt.Sprintf("%s %s %s", action, typ, target), res) {
				return nil
			}
			pid, err := app.Client.CreateRule(c.Context(), firewalla.RuleSpec{
				Action: action, Type: typ, Target: target, Direction: direction, Notes: notes,
			})
			if err != nil {
				return err
			}
			res.Rule = pid
			return app.reportMutation(fmt.Sprintf("created rule %s", pid), res)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the change")
	cmd.Flags().StringVar(&direction, "direction", "bidirection", "bidirection | inbound | outbound")
	cmd.Flags().StringVar(&notes, "notes", "via fire cli", "note stored on the rule")
	return cmd
}

// newRuleRmCmd: fire rules rm <id>
func newRuleRmCmd(app *App) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:               "rm [id]",
		Aliases:           []string{"delete", "del"},
		Short:             "Delete a rule by id (omit to pick interactively)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: app.completeRule,
		RunE: func(c *cobra.Command, args []string) error {
			id, err := app.resolveOrPickRule(c.Context(), args, "Delete which rule?")
			if err != nil {
				return err
			}
			if id == "" {
				return nil // cancelled
			}
			res := &mutationResult{Action: "rule.rm", Target: id, Rule: id}
			if !app.beginMutation(confirm, fmt.Sprintf("delete rule %s", id), res) {
				return nil
			}
			if err := app.Client.DeleteRule(c.Context(), id); err != nil {
				return err
			}
			return app.reportMutation(fmt.Sprintf("deleted rule %s", id), res)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the change")
	return cmd
}

// newRuleToggleCmd builds the enable or disable subcommand.
func newRuleToggleCmd(app *App, disable bool) *cobra.Command {
	verb := "enable"
	if disable {
		verb = "disable"
	}
	var confirm bool
	cmd := &cobra.Command{
		Use:               verb + " [id]",
		Short:             verb + " a rule by id (omit to pick interactively)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: app.completeRule,
		RunE: func(c *cobra.Command, args []string) error {
			id, err := app.resolveOrPickRule(c.Context(), args, "Which rule to "+verb+"?")
			if err != nil {
				return err
			}
			if id == "" {
				return nil // cancelled
			}
			res := &mutationResult{Action: "rule." + verb, Target: id, Rule: id}
			if !app.beginMutation(confirm, fmt.Sprintf("%s rule %s", verb, id), res) {
				return nil
			}
			if err := app.Client.SetRuleDisabled(c.Context(), id, disable); err != nil {
				return err
			}
			return app.reportMutation(fmt.Sprintf("%sd rule %s", verb, id), res)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the change")
	return cmd
}
