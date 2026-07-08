package cmd

import (
	"fmt"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/stefanpenner/fire.cli/internal/firewalla"
)

// blockVerb carries the wording for a block-like command (block/pause), so the
// two share one implementation while reading naturally in prompts and output.
type blockVerb struct {
	verb string // imperative shown before confirm ("block", "pause")
	past string // result wording ("blocked", "paused")
}

// runBlock resolves the device (arg or interactive picker) and creates a MAC
// block rule, optionally auto-expiring after forDur. Shared by block and pause.
func (app *App) runBlock(c *cobra.Command, args []string, v blockVerb, confirm bool, forDur time.Duration) error {
	idx := loadDevices(c.Context(), app)
	mac, err := resolveOrPick(app, idx, args, capitalize(v.verb)+" which device?")
	if err != nil {
		return err
	}
	if mac == "" {
		return nil // cancelled
	}
	label := idx.name(mac)
	action := fmt.Sprintf("%s %s (%s)", v.verb, label, mac)
	if forDur > 0 {
		action += fmt.Sprintf(" for %s", forDur)
	}
	res := &mutationResult{Action: v.verb, Target: label, MAC: mac}
	if !app.beginMutation(confirm, action, res) {
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
	res.Rule = pid
	return app.reportMutation(fmt.Sprintf("%s %s (rule %s)", v.past, label, pid), res)
}

// runUnblock removes a device's block rule(s). Shared by unblock and resume.
func (app *App) runUnblock(c *cobra.Command, args []string, v blockVerb, confirm bool) error {
	idx := loadDevices(c.Context(), app)
	mac, err := resolveOrPick(app, idx, args, capitalize(v.verb)+" which device?")
	if err != nil {
		return err
	}
	if mac == "" {
		return nil // cancelled
	}
	label := idx.name(mac)
	res := &mutationResult{Action: v.verb, Target: label, MAC: mac}
	if !app.beginMutation(confirm, fmt.Sprintf("%s %s (%s)", v.verb, label, mac), res) {
		return nil
	}
	n, err := app.Client.DeleteMatching(c.Context(), firewalla.RuleSpec{
		Action: "block", Type: "mac", Target: mac,
	})
	if err != nil {
		return err
	}
	res.Count = &n
	return app.reportMutation(fmt.Sprintf("%s %s (removed %d rule(s))", v.past, label, n), res)
}

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
			return app.runBlock(c, args, blockVerb{"block", "blocked"}, confirm, forDur)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the change (without it, only prints what would happen)")
	cmd.Flags().DurationVar(&forDur, "for", 0, "auto-expire the block after this duration (e.g. 1h)")
	return cmd
}

// newPauseCmd is the iOS-app's "pause internet": a device block, usually timed.
func newPauseCmd(app *App) *cobra.Command {
	var (
		confirm bool
		forDur  time.Duration
	)
	cmd := &cobra.Command{
		Use:               "pause [device]",
		Short:             "Pause a device's internet (a block; use --for to auto-resume)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: app.completeDevice,
		RunE: func(c *cobra.Command, args []string) error {
			return app.runBlock(c, args, blockVerb{"pause", "paused"}, confirm, forDur)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the change (without it, only prints what would happen)")
	cmd.Flags().DurationVar(&forDur, "for", 0, "auto-resume after this duration (e.g. 1h)")
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
			return app.runUnblock(c, args, blockVerb{"unblock", "unblocked"}, confirm)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the change (without it, only prints what would happen)")
	return cmd
}

// newResumeCmd resumes a paused device (removes its block).
func newResumeCmd(app *App) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:               "resume [device]",
		Short:             "Resume a paused device's internet (removes the block)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: app.completeDevice,
		RunE: func(c *cobra.Command, args []string) error {
			return app.runUnblock(c, args, blockVerb{"resume", "resumed"}, confirm)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the change (without it, only prints what would happen)")
	return cmd
}

// capitalize upper-cases the first rune (UTF-8 safe; never byte-underflows).
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
