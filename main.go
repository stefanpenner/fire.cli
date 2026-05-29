// Command fire is a CLI for a Firewalla box. It runs over SSH and exposes
// devices, DNS activity, flows, and a redis-cli escape hatch.
package main

import (
	"fmt"
	"os"

	"github.com/stefanpenner/fire.cli/cmd"
)

func main() {
	app := &cmd.App{Out: os.Stdout, Err: os.Stderr}
	if err := cmd.NewRootCmd(app).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
