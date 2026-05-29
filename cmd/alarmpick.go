package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stefanpenner/fire.cli/internal/picker"
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
	if !picker.Interactive(app.Err) {
		return "", errors.New("alarm required: pass an id, or run in a terminal to pick one (see `fire alarms`)")
	}
	alarms, err := app.Client.ListAlarms(ctx, alarmPickLimit)
	if err != nil {
		return "", err
	}
	if len(alarms) == 0 {
		return "", errors.New("no alarms to choose from")
	}
	items := make([]string, len(alarms))
	for i, a := range alarms {
		items[i] = strings.TrimSpace(fmt.Sprintf("%s  %s  %s  %s", a.ID, a.Type, a.Device, a.Message))
	}
	i, err := picker.Select(app.Err, prompt, items, 12)
	if errors.Is(err, picker.ErrCancelled) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return alarms[i].ID, nil
}

// completeAlarm offers recent alarm ids annotated with their type/device.
func (app *App) completeAlarm(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) >= 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	_ = app.connect(cmd, nil)
	if app.Client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	alarms, err := app.Client.ListAlarms(cmd.Context(), alarmPickLimit)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := make([]string, 0, len(alarms))
	for _, a := range alarms {
		desc := strings.TrimSpace(fmt.Sprintf("%s %s", a.Type, a.Device))
		out = append(out, fmt.Sprintf("%s\t%s", a.ID, desc))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}
