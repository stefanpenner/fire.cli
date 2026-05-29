package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stefanpenner/fire.cli/internal/picker"
)

// Shared scaffolding for the interactive "pick one of these" commands
// (rules, features, alarms, devices). Each command supplies the type-specific
// loader, display, and arg-resolution; these helpers carry the common UX so it
// stays identical across them.

// requireInteractive returns the standard "<noun> required" error when no
// argument was given and the output is not a terminal. passHint describes the
// accepted argument (e.g. "an id"); discover is a pointer to the listing
// command (e.g. "see `fire rules`").
func (app *App) requireInteractive(noun, passHint, discover string) error {
	if picker.Interactive(app.Err) {
		return nil
	}
	return fmt.Errorf("%s required: pass %s, or run in a terminal to pick one (%s)", noun, passHint, discover)
}

// selectIndex runs the fuzzy picker over items and returns the chosen index, or
// -1 when the user cancels. noun is used for the empty-list error. Callers must
// have already passed requireInteractive.
func (app *App) selectIndex(noun, prompt string, items []string) (int, error) {
	if len(items) == 0 {
		return -1, fmt.Errorf("no %ss to choose from", noun)
	}
	i, err := picker.Select(app.Err, prompt, items, 12)
	if errors.Is(err, picker.ErrCancelled) {
		return -1, nil
	}
	return i, err
}

// completionFor wires the client (shell completion skips PersistentPreRunE) and
// runs list to produce "value\tdescription" suggestions, swallowing errors so a
// box hiccup never breaks completion. It returns no suggestions once a
// positional argument is already present.
func (app *App) completionFor(cmd *cobra.Command, args []string, list func(context.Context) ([]string, error)) ([]string, cobra.ShellCompDirective) {
	if len(args) >= 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	_ = app.connect(cmd, nil)
	if app.Client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out, err := list(cmd.Context())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}
