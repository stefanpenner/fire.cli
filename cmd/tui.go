package cmd

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/stefanpenner/fire.cli/internal/picker"
	"github.com/stefanpenner/fire.cli/internal/render"
	"github.com/stefanpenner/fire.cli/internal/tui"
)

// tuiSource adapts the command-side Client to the tui.DataSource the dashboard
// needs.
type tuiSource struct{ Client }

func newTUICmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "tui",
		Aliases: []string{"dashboard", "ui"},
		Short:   "Launch the interactive dashboard (searchable devices, block/unblock)",
		Args:    cobra.NoArgs,
		RunE:    func(c *cobra.Command, _ []string) error { return app.runTUI() },
	}
}

// runTUI starts the Bubble Tea program. It requires a real terminal.
func (app *App) runTUI() error {
	if !picker.Interactive(app.Out) {
		return errors.New("the dashboard needs an interactive terminal; use the subcommands (e.g. `fire devices`) when piping")
	}
	m := tui.NewModel(tuiSource{app.Client}, app.now).
		WithColor(render.ColorEnabled(app.Out, app.NoColor))
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Ensure the adapter satisfies the interface at compile time (Host/ListDevices/
// CreateRule/DeleteMatching are promoted from the embedded Client).
var _ tui.DataSource = tuiSource{}
