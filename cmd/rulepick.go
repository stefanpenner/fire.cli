package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// rulePick is one selectable rule: a display string and the id it resolves to.
type rulePick struct {
	id      string
	display string
}

// loadRulePicks fetches the current rules and renders each as a fuzzy-search
// line ("<id>  <action> <type> <target>  (state)"). Disabled rules are
// included so they can be re-enabled or removed interactively.
func loadRulePicks(ctx context.Context, app *App) ([]rulePick, error) {
	rules, err := app.Client.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	picks := make([]rulePick, 0, len(rules))
	for _, r := range rules {
		state := "enabled"
		if r.Disabled {
			state = "disabled"
		}
		display := strings.TrimSpace(fmt.Sprintf("%s  %s %s %s  (%s)",
			r.ID, r.Action, r.Type, r.Target, state))
		picks = append(picks, rulePick{id: r.ID, display: display})
	}
	return picks, nil
}

// resolveOrPickRule turns args[0] into a rule id, or—when no id is given and a
// terminal is available—opens the fuzzy picker. An explicit id is passed
// through verbatim (even if not in the local list) so the box can validate it.
// Returns "" with a nil error when the user cancels the picker.
func (app *App) resolveOrPickRule(ctx context.Context, args []string, prompt string) (string, error) {
	if len(args) >= 1 {
		return args[0], nil
	}
	if err := app.requireInteractive("rule", "an id", "see `fire rules`"); err != nil {
		return "", err
	}
	picks, err := loadRulePicks(ctx, app)
	if err != nil {
		return "", err
	}
	items := make([]string, len(picks))
	for i, p := range picks {
		items[i] = p.display
	}
	i, err := app.selectIndex("rule", prompt, items)
	if err != nil || i < 0 {
		return "", err
	}
	return picks[i].id, nil
}

// completeRule is a cobra completion function offering rule ids annotated with
// their action/type/target, so tab-completion doubles as discovery.
func (app *App) completeRule(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	return app.completionFor(cmd, args, func(ctx context.Context) ([]string, error) {
		rules, err := app.Client.ListRules(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(rules))
		for _, r := range rules {
			desc := strings.TrimSpace(fmt.Sprintf("%s %s %s", r.Action, r.Type, r.Target))
			out = append(out, fmt.Sprintf("%s\t%s", r.ID, desc))
		}
		return out, nil
	})
}
