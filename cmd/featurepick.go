package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stefanpenner/fire.cli/internal/picker"
)

// resolveFeature maps an identifier (key or friendly name, case-insensitive,
// name matched as a substring) to a feature. ok is false when nothing matches.
func resolveFeature(feats []firewalla.Feature, id string) (firewalla.Feature, bool) {
	low := strings.ToLower(strings.TrimSpace(id))
	for _, f := range feats {
		if strings.ToLower(f.Key) == low {
			return f, true
		}
	}
	for _, f := range feats {
		if strings.Contains(strings.ToLower(f.Name), low) {
			return f, true
		}
	}
	return firewalla.Feature{}, false
}

// resolveOrPickFeature turns args[0] into a feature, or—when no arg is given
// and a terminal is available—opens the fuzzy picker. Returns ok=false with a
// nil error when the user cancels the picker.
func (app *App) resolveOrPickFeature(ctx context.Context, args []string, prompt string) (firewalla.Feature, bool, error) {
	feats, err := app.Client.ListFeatures(ctx)
	if err != nil {
		return firewalla.Feature{}, false, err
	}
	if len(args) >= 1 {
		f, ok := resolveFeature(feats, args[0])
		if !ok {
			return firewalla.Feature{}, false, fmt.Errorf("no feature matches %q; run `fire features` to list them", args[0])
		}
		return f, true, nil
	}
	if !picker.Interactive(app.Err) {
		return firewalla.Feature{}, false, errors.New("feature required: pass a key/name, or run in a terminal to pick one (see `fire features`)")
	}
	if len(feats) == 0 {
		return firewalla.Feature{}, false, errors.New("no features to choose from")
	}
	items := make([]string, len(feats))
	for i, f := range feats {
		items[i] = fmt.Sprintf("%s  (%s)  [%s]", f.Name, onOff(f.Enabled), f.Key)
	}
	i, err := picker.Select(app.Err, prompt, items, 12)
	if errors.Is(err, picker.ErrCancelled) {
		return firewalla.Feature{}, false, nil
	}
	if err != nil {
		return firewalla.Feature{}, false, err
	}
	return feats[i], true, nil
}

// completeFeature offers feature keys annotated with their friendly name.
func (app *App) completeFeature(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) >= 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	_ = app.connect(cmd, nil)
	if app.Client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	feats, err := app.Client.ListFeatures(cmd.Context())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := make([]string, 0, len(feats))
	for _, f := range feats {
		out = append(out, fmt.Sprintf("%s\t%s (%s)", f.Key, f.Name, onOff(f.Enabled)))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}
