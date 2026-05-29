package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// alarmPickLimit caps how many alarms the picker/completion fetch.
const alarmPickLimit = 50

// resolveOrPickAlarm turns args[0] into an alarm id, or—when no id is given and
// a terminal is available—opens the fuzzy picker over recent alarms. An
// explicit id is passed through verbatim. Returns "" on cancel.
func (app *App) resolveOrPickAlarm(ctx context.Context, args []string, prompt string) (string, error) {
	if len(args) >= 1 {
		return args[0], nil
	}
	if err := app.requireInteractive("alarm", "an id", "see `fire alarms`"); err != nil {
		return "", err
	}
	alarms, err := app.Client.ListAlarms(ctx, alarmPickLimit)
	if err != nil {
		return "", err
	}
	items := make([]string, len(alarms))
	for i, a := range alarms {
		items[i] = strings.TrimSpace(fmt.Sprintf("%s  %s  %s  %s", a.ID, a.Type, a.Device, a.Message))
	}
	i, err := app.selectIndex("alarm", prompt, items)
	if err != nil || i < 0 {
		return "", err
	}
	return alarms[i].ID, nil
}

// completeAlarm offers recent alarm ids annotated with their type/device.
func (app *App) completeAlarm(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	return app.completionFor(cmd, args, func(ctx context.Context) ([]string, error) {
		alarms, err := app.Client.ListAlarms(ctx, alarmPickLimit)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(alarms))
		for _, a := range alarms {
			desc := strings.TrimSpace(fmt.Sprintf("%s %s", a.Type, a.Device))
			out = append(out, fmt.Sprintf("%s\t%s", a.ID, desc))
		}
		return out, nil
	})
}
