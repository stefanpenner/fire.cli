package cmd

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var macRE = regexp.MustCompile(`^[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}$`)

// confirmed gates a mutating action. It always prints what will change; when
// confirm is false it tells the user how to apply and returns false so the
// caller can stop without performing the mutation.
func (a *App) confirmed(confirm bool, action string) bool {
	if confirm {
		fmt.Fprintf(a.Err, "%s…\n", action)
		return true
	}
	fmt.Fprintf(a.Err, "would %s\nre-run with --confirm to apply\n", action)
	return false
}

// looksLikeMAC reports whether s is a colon-separated MAC address.
func looksLikeMAC(s string) bool { return macRE.MatchString(s) }

// deviceIndex resolves user-supplied device identifiers and renders names.
type deviceIndex struct {
	nameByMAC map[string]string // upper MAC → friendly name
	macByIP   map[string]string
	macByName map[string]string // lower name → upper MAC
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
	}
	return idx
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
