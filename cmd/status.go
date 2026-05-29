package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newStatusCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check that the Firewalla is reachable and summarize it",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			pong, err := app.Client.Raw(c.Context(), "ping")
			reachable := err == nil && strings.Contains(strings.ToUpper(pong), "PONG")

			deviceCount := -1
			if reachable {
				if devices, derr := app.Client.ListDevices(c.Context()); derr == nil {
					deviceCount = len(devices)
				}
			}

			if app.JSON {
				return app.output(nil, nil, map[string]any{
					"host":      app.Client.Host(),
					"reachable": reachable,
					"devices":   deviceCount,
				})
			}
			status := "unreachable"
			if reachable {
				status = "reachable"
			}
			fmt.Fprintf(app.Out, "host:      %s\n", app.Client.Host())
			fmt.Fprintf(app.Out, "status:    %s\n", status)
			if deviceCount >= 0 {
				fmt.Fprintf(app.Out, "devices:   %d\n", deviceCount)
			}
			if !reachable {
				return fmt.Errorf("firewalla %s is not reachable", app.Client.Host())
			}
			return nil
		},
	}
}
