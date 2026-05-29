package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newRedisCmd is the escape hatch: a thin passthrough to redis-cli on the box
// for the long tail of the Firewalla surface not yet modelled as a command.
func newRedisCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "redis <args>...",
		Short: "Run a redis-cli command on the Firewalla",
		Long: "Pass arguments straight through to redis-cli on the box.\n\n" +
			"Examples:\n" +
			"  fire redis ping\n" +
			"  fire redis keys 'policy:*'\n" +
			"  fire redis hgetall host:mac:AA:BB:CC:DD:EE:FF",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		RunE: func(c *cobra.Command, args []string) error {
			out, err := app.Client.Raw(c.Context(), strings.Join(args, " "))
			if out != "" {
				fmt.Fprint(app.Out, out)
				if !strings.HasSuffix(out, "\n") {
					fmt.Fprintln(app.Out)
				}
			}
			return err
		},
	}
}
