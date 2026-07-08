package tui

import (
	"fmt"
	"sort"
	"time"

	"github.com/stefanpenner/fire.cli/internal/firewalla"
)

// topPeers returns the n peers with the most total bytes, descending. It does
// not mutate the input slice.
func topPeers(peers []firewalla.Peer, n int) []firewalla.Peer {
	out := make([]firewalla.Peer, len(peers))
	copy(out, peers)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Bytes() > out[j].Bytes() })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// humanizeBytes renders a byte count compactly: 0 B, 4.0 KB, 1.5 MB, 12.3 GB.
// 1024-based; values below 1 KB show as whole bytes.
func humanizeBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for v := b / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// lastSeen renders a last-active time relative to now ("12m ago", "never").
func lastSeen(t, now time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := now.Sub(t)
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
