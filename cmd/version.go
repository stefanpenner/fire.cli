package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stefanpenner/fire.cli/internal/render"
)

// Version is the CLI version, overridden at build time via:
//
//	-ldflags "-X github.com/stefanpenner/fire.cli/cmd.Version=v1.2.3"
var Version = "dev"

func newVersionCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the fire version",
		Args:  cobra.NoArgs,
		// version must work without connecting to a box.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(*cobra.Command, []string) error {
			if app.JSON {
				return render.JSON(app.Out, map[string]string{"version": Version})
			}
			fmt.Fprintln(app.Out, Version)
			return nil
		},
	}
}
