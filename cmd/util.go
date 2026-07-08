package cmd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/stefanpenner/fire.cli/internal/picker"
	"github.com/stefanpenner/fire.cli/internal/render"
)

// mutationResult is the machine-readable outcome of a mutating command, emitted
// as one JSON object when --json is set. `applied` is the field agents key on:
// false + dryRun means nothing changed (no --confirm); true means it did.
type mutationResult struct {
	Action  string `json:"action"`           // block | unblock | pause | resume | rule.add | feature.enable | alarm.archive | …
	Target  string `json:"target,omitempty"` // device label, rule id, feature name, alarm id
	MAC     string `json:"mac,omitempty"`
	Rule    string `json:"rule,omitempty"`  // policy id created / affected
	Count   *int   `json:"count,omitempty"` // rules removed
	State   string `json:"state,omitempty"` // on | off (feature toggles)
	Applied bool   `json:"applied"`
	DryRun  bool   `json:"dryRun,omitempty"`
	Message string `json:"message,omitempty"` // the human sentence, for convenience
}

// beginMutation gates a mutating action, returning true to proceed. Human mode
// is unchanged: with --confirm it prints "<action>…" to stderr; without it, it
// prints "would <action>" and returns false. In --json mode it prints nothing
// on the confirmed path (the result follows from reportMutation) and emits a
// dryRun object on the unconfirmed path.
func (a *App) beginMutation(confirm bool, action string, res *mutationResult) bool {
	res.Message = action
	if confirm {
		if !a.JSON {
			fmt.Fprintf(a.Err, "%s…\n", action)
		}
		return true
	}
	res.DryRun = true
	if a.JSON {
		_ = render.JSON(a.Out, res)
	} else {
		fmt.Fprintf(a.Err, "would %s\nre-run with --confirm to apply\n", action)
	}
	return false
}

// reportMutation reports a completed mutation: JSON with applied=true when
// --json, else the human sentence to stdout.
func (a *App) reportMutation(msg string, res *mutationResult) error {
	res.Applied, res.Message = true, msg
	if a.JSON {
		return render.JSON(a.Out, res)
	}
	fmt.Fprintln(a.Out, msg)
	return nil
}

// reportNoop reports a mutation that was unnecessary (e.g. a feature already in
// the requested state): human sentence to stdout, or JSON with applied=false.
func (a *App) reportNoop(msg string, res *mutationResult) error {
	res.Message = msg
	if a.JSON {
		return render.JSON(a.Out, res)
	}
	fmt.Fprintln(a.Out, msg)
	return nil
}

// resolveOrPick turns args[0] into a MAC, or—when no device arg is given and a
// terminal is available—opens the fuzzy picker. Returns "" with a nil error
// when the user cancels the picker, so callers can exit cleanly.
func resolveOrPick(app *App, idx *deviceIndex, args []string, prompt string) (string, error) {
	if len(args) >= 1 {
		mac := idx.resolveMAC(args[0])
		if mac == "" {
			return "", fmt.Errorf("no device matches %q; run `fire devices` to list devices (name, IP, or MAC all work)", args[0])
		}
		return mac, nil
	}
	if !picker.Interactive(app.Err) {
		return "", errors.New("device required: pass a name/IP/MAC, or run in a terminal to pick one interactively")
	}
	mac, err := app.pickDevice(idx, prompt)
	if errors.Is(err, picker.ErrCancelled) {
		return "", nil
	}
	return mac, err
}

// completeDevice is a cobra completion function offering device names and IPs
// for arguments that take a device (traffic, block, unblock). It queries the
// box, so tab-completion doubles as discovery of valid values.
func (app *App) completeDevice(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	// nil args: a device may be the first of several positionals (traffic), so
	// always offer suggestions regardless of how many are already present.
	return app.completionFor(cmd, nil, func(ctx context.Context) ([]string, error) {
		devices, err := app.Client.ListDevices(ctx)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, d := range devices {
			if d.Name != "" {
				out = append(out, d.Name)
			}
			if d.IP != "" {
				out = append(out, d.IP)
			}
		}
		return out, nil
	})
}

var macRE = regexp.MustCompile(`^[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}$`)

// looksLikeMAC reports whether s is a colon-separated MAC address.
func looksLikeMAC(s string) bool { return macRE.MatchString(s) }

// deviceIndex resolves user-supplied device identifiers and renders names.
type deviceIndex struct {
	nameByMAC map[string]string // upper MAC → friendly name
	macByIP   map[string]string
	macByName map[string]string // lower name → upper MAC
	picks     []pickItem        // ordered list for the interactive picker
}

// pickItem is one selectable device: a display string and the MAC it resolves to.
type pickItem struct {
	mac     string
	display string
}

// loadDevices fetches the device list once for resolution + display. On error
// it returns an empty index so callers still function with raw MACs.
func loadDevices(ctx context.Context, app *App) *deviceIndex {
	idx := &deviceIndex{
		nameByMAC: map[string]string{},
		macByIP:   map[string]string{},
		macByName: map[string]string{},
	}
	devices, err := app.Client.ListDevices(ctx)
	if err != nil {
		return idx
	}
	for _, d := range devices {
		mac := strings.ToUpper(d.MAC)
		if d.Name != "" {
			idx.nameByMAC[mac] = d.Name
			idx.macByName[strings.ToLower(d.Name)] = mac
		}
		if d.IP != "" {
			idx.macByIP[d.IP] = mac
		}
		// Picker display includes name, IP and MAC so any of them is fuzzy-searchable.
		display := strings.TrimSpace(strings.Join(nonEmpty(d.Name, d.IP, mac), "  "))
		idx.picks = append(idx.picks, pickItem{mac: mac, display: display})
	}
	return idx
}

// nonEmpty returns the non-empty arguments, in order.
func nonEmpty(vals ...string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// pickDevice runs the interactive fuzzy finder over the known devices and
// returns the chosen MAC. picker.ErrCancelled is propagated for the caller to
// treat as a clean abort.
func (a *App) pickDevice(idx *deviceIndex, prompt string) (string, error) {
	if len(idx.picks) == 0 {
		return "", fmt.Errorf("no devices to choose from")
	}
	items := make([]string, len(idx.picks))
	for i, p := range idx.picks {
		items[i] = p.display
	}
	i, err := picker.Select(a.Err, prompt, items, 12)
	if err != nil {
		return "", err
	}
	return idx.picks[i].mac, nil
}

// resolveMAC maps an identifier (MAC, IP, or name substring) to an upper MAC.
// Returns "" when nothing matches and the input isn't already a MAC.
func (idx *deviceIndex) resolveMAC(id string) string {
	if looksLikeMAC(id) {
		return strings.ToUpper(id)
	}
	if mac, ok := idx.macByIP[id]; ok {
		return mac
	}
	if mac, ok := idx.macByName[strings.ToLower(id)]; ok {
		return mac
	}
	low := strings.ToLower(id)
	for name, mac := range idx.macByName {
		if strings.Contains(name, low) {
			return mac
		}
	}
	return ""
}

// name renders a MAC as its friendly name, falling back to the MAC itself.
func (idx *deviceIndex) name(mac string) string {
	if n, ok := idx.nameByMAC[strings.ToUpper(mac)]; ok {
		return n
	}
	return mac
}

// humanizeBytes renders a byte count compactly: 0 B, 4.0 KB, 1.5 MB, 12.3 GB.
// Uses 1024-based units; values below 1 KB show as whole bytes.
func humanizeBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// humanizeDuration renders a duration compactly: 45s, 12m, 3h, 5d.
func humanizeDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
