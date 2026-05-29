// Package cmd wires the cobra command tree. Commands depend only on the
// Client interface and an injected App (writers + clock), so every command is
// unit-testable against a fake client with no SSH or network.
package cmd

import (
	"context"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/stefanpenner/fire.cli/internal/firewalla"
	"github.com/stefanpenner/fire.cli/internal/picker"
	"github.com/stefanpenner/fire.cli/internal/render"
	"github.com/stefanpenner/fire.cli/internal/transport"
)

// Client is the firewalla surface the commands need. Defined consumer-side so
// tests can supply a fake; *firewalla.Client satisfies it.
type Client interface {
	Host() string
	ListDevices(ctx context.Context) ([]firewalla.Device, error)
	DNSByDevice(ctx context.Context, mac string, limit int) ([]firewalla.DNSQuery, error)
	WhoResolved(ctx context.Context, domain string) ([]firewalla.Resolver, error)
	ListNetworks(ctx context.Context) ([]firewalla.Network, error)
	ListRules(ctx context.Context) ([]firewalla.Rule, error)
	ListWANs(ctx context.Context) ([]firewalla.WAN, error)
	DataUsage(ctx context.Context) (firewalla.DataUsageReport, error)
	Traffic(ctx context.Context, mac string) ([]firewalla.Peer, error)
	ListAlarms(ctx context.Context, limit int) ([]firewalla.Alarm, error)
	ListFeatures(ctx context.Context) ([]firewalla.Feature, error)
	CreateRule(ctx context.Context, spec firewalla.RuleSpec) (string, error)
	DeleteMatching(ctx context.Context, spec firewalla.RuleSpec) (int, error)
	DeleteRule(ctx context.Context, id string) error
	SetRuleDisabled(ctx context.Context, id string, disabled bool) error
	SetFeature(ctx context.Context, key string, enabled bool) error
	ArchiveAlarm(ctx context.Context, id string) error
	DeleteAlarm(ctx context.Context, id string) error
	Raw(ctx context.Context, args string) (string, error)
}

// App carries the injected dependencies shared by every command.
type App struct {
	Out, Err io.Writer
	Client   Client
	Now      func() time.Time

	// persistent flags
	Host    string
	JSON    bool
	NoColor bool
}

func (a *App) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// output prints either JSON (jsonVal) or a human table (headers/rows),
// honoring --json and color gating.
func (a *App) output(headers []string, rows [][]string, jsonVal any) error {
	if a.JSON {
		return render.JSON(a.Out, jsonVal)
	}
	return render.Table(a.Out, headers, rows, render.ColorEnabled(a.Out, a.NoColor))
}

// NewRootCmd builds the command tree for the given App.
func NewRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:   "fire",
		Short: "fire — a fast CLI for your Firewalla box",
		Long: "fire is a command-line interface for a Firewalla box.\n" +
			"It runs over SSH and exposes devices, DNS activity, flows and more.",
		SilenceUsage:      true,
		SilenceErrors:     true,
		Version:           Version,
		PersistentPreRunE: app.connect,
		// Bare `fire` in a terminal launches the interactive dashboard;
		// piped/redirected, it falls back to printing help.
		RunE: func(c *cobra.Command, _ []string) error {
			if picker.Interactive(app.Out) {
				return app.runTUI()
			}
			return c.Help()
		},
	}
	root.SetOut(app.Out)
	root.SetErr(app.Err)

	pf := root.PersistentFlags()
	pf.StringVar(&app.Host, "host", "pi@fire.walla", "ssh destination of the Firewalla box")
	pf.BoolVar(&app.JSON, "json", false, "output JSON instead of a table")
	pf.BoolVar(&app.NoColor, "no-color", false, "disable colored output")

	root.AddCommand(
		newVersionCmd(app),
		newDevicesCmd(app),
		newDNSCmd(app),
		newNetworksCmd(app),
		newRulesCmd(app),
		newWANCmd(app),
		newDataCmd(app),
		newTrafficCmd(app),
		newAlarmsCmd(app),
		newFeaturesCmd(app),
		newBlockCmd(app),
		newUnblockCmd(app),
		newPauseCmd(app),
		newResumeCmd(app),
		newStatusCmd(app),
		newRedisCmd(app),
		newTUICmd(app),
	)
	return root
}

// connect lazily builds a real SSH-backed client when one was not injected
// (tests inject a fake, so this is a no-op there).
func (a *App) connect(_ *cobra.Command, _ []string) error {
	if a.Client == nil {
		a.Client = firewalla.New(transport.NewSSH(a.Host))
	}
	return nil
}
