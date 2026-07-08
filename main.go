// Command fire is a CLI for a Firewalla box. It runs over SSH and exposes
// devices, DNS activity, flows, and a redis-cli escape hatch.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/stefanpenner/fire.cli/cmd"
)

func main() {
	app := &cmd.App{Out: os.Stdout, Err: os.Stderr}
	os.Exit(run(os.Stdout, os.Stderr, cmd.NewRootCmd(app).Execute))
}

// run executes fn and maps its outcome to a process exit code, printing exactly
// once. It is the single containment boundary: a panic escaping fn (e.g. a
// parser that slipped past the fuzzers on some hostile box output) degrades to
// a clean non-zero exit instead of a crash. Split out so this is unit-tested.
func run(_ io.Writer, stderr io.Writer, fn func() error) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(stderr, "fatal: internal error:", r)
			code = 2
		}
	}()
	if err := fn(); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}
