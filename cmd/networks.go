package cmd

import (
	"strconv"

	"github.com/spf13/cobra"
)

func newNetworksCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "networks",
		Aliases: []string{"network", "net", "nets", "vlans", "vlan"},
		Short:   "List networks and VLANs configured on the Firewalla",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			nets, err := app.Client.ListNetworks(c.Context())
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(nets))
			for _, n := range nets {
				vlan := ""
				if n.VLANID > 0 {
					vlan = strconv.Itoa(n.VLANID)
				}
				rows = append(rows, []string{n.Name, n.Type, n.Interface, vlan, n.Subnet})
			}
			return app.output(
				[]string{"name", "type", "interface", "vlan", "subnet"},
				rows, nets,
			)
		},
	}
}
