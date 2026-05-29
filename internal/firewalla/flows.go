package firewalla

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Traffic section markers in the Traffic stream.
const (
	tDownload      = "@@DL@@"
	tUpload        = "@@UL@@"
	tLocalDownload = "@@LDL@@"
	tLocalUpload   = "@@LUL@@"
)

// Peer is one endpoint a device exchanged traffic with, aggregated over
// Firewalla's most recent rollup window. Internet peers carry a Domain/IP;
// local peers carry the peer device's MAC.
type Peer struct {
	Label    string `json:"label"`    // domain or destIP (internet); peer MAC (local)
	PeerMAC  string `json:"peerMac"`  // set for device↔device peers
	Kind     string `json:"kind"`     // "internet" | "device"
	Upload   int64  `json:"upload"`   // bytes sent by the device
	Download int64  `json:"download"` // bytes received by the device
}

// Bytes returns total bytes exchanged with the peer.
func (p Peer) Bytes() int64 { return p.Upload + p.Download }

// Traffic returns the peers a device exchanged traffic with — internet
// destinations (by domain/IP) and other LAN devices (by MAC) — aggregated from
// Firewalla's own `sumflow` rollups (the most recent window per metric). This
// is the "what is this device talking to?" view; pass it through a peer filter
// for "is A talking to B?".
//
// Schema: sumflow:<MAC>:{download,upload}:<begin>:<end> (internet) and
// sumflow:<MAC>:local:{download,upload}:<begin>:<end> (device↔device) are
// zsets whose members are JSON {domain|destIP|dstMac, port, fd} scored by
// bytes. We pick the window with the greatest <end> (most recent 24h).
func (c *Client) Traffic(ctx context.Context, mac string) ([]Peer, error) {
	m := strings.ToUpper(mac)
	// sel <pattern> → the matching key with the greatest trailing :<end> ts.
	// Internet patterns exclude the "local:" rollups (distinct key prefix).
	script := `sel() { redis-cli --scan --pattern "$1" | awk -F: '{print $NF, $0}' | sort -rn | head -1 | cut -d" " -f2-; }; ` +
		`emit() { k=$(sel "$2"); [ -n "$k" ] && { echo "$1"; redis-cli zrevrange "$k" 0 -1 withscores; }; }; ` +
		fmt.Sprintf(`emit %s 'sumflow:%s:download:*'; emit %s 'sumflow:%s:upload:*'; `,
			tDownload, m, tUpload, m) +
		fmt.Sprintf(`emit %s 'sumflow:%s:local:download:*'; emit %s 'sumflow:%s:local:upload:*'`,
			tLocalDownload, m, tLocalUpload, m)
	res, err := c.t.Run(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("reading traffic for %s: %w", mac, err)
	}
	return parseTraffic(res.Stdout), nil
}

// rawPeer mirrors a sumflow member.
type rawPeer struct {
	Domain string `json:"domain"`
	DestIP string `json:"destIP"`
	DstMac string `json:"dstMac"`
}

// parseTraffic merges the four sumflow sections into per-peer up/down totals.
func parseTraffic(s string) []Peer {
	type acc struct {
		up, down int64
		mac, lab string
		kind     string
	}
	peers := map[string]*acc{}
	get := func(key, label, mac, kind string) *acc {
		a := peers[key]
		if a == nil {
			a = &acc{lab: label, mac: mac, kind: kind}
			peers[key] = a
		}
		return a
	}

	// Sections in stream order; `end` marks where each section's body stops.
	sections := []struct {
		marker, end     string
		download, local bool
	}{
		{tDownload, tUpload, true, false},
		{tUpload, tLocalDownload, false, false},
		{tLocalDownload, tLocalUpload, true, true},
		{tLocalUpload, "", false, true},
	}
	for _, sec := range sections {
		body := between(s, sec.marker, sec.end)
		for _, e := range parseScoredMembers(body) {
			var rp rawPeer
			if json.Unmarshal([]byte(e.member), &rp) != nil {
				continue
			}
			var key, label, mac, kind string
			if sec.local {
				mac = strings.ToUpper(rp.DstMac)
				if mac == "" {
					continue
				}
				key, label, kind = "dev:"+mac, mac, "device"
			} else {
				label = firstNonEmpty(rp.Domain, rp.DestIP)
				if label == "" {
					continue
				}
				key, kind = "net:"+label, "internet"
			}
			a := get(key, label, mac, kind)
			if sec.download {
				a.down += e.score
			} else {
				a.up += e.score
			}
		}
	}

	out := make([]Peer, 0, len(peers))
	for _, a := range peers {
		out = append(out, Peer{Label: a.lab, PeerMAC: a.mac, Kind: a.kind, Upload: a.up, Download: a.down})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Bytes() > out[j].Bytes() })
	return out
}

// between returns the slice of s after marker `start` up to `end` (or end of s
// when end is ""). Returns "" if start is absent.
func between(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	i += len(start)
	if end == "" {
		return s[i:]
	}
	if j := strings.Index(s[i:], end); j >= 0 {
		return s[i : i+j]
	}
	return s[i:]
}

type scored struct {
	member string
	score  int64
}

// parseScoredMembers reads `zrange … withscores` output (alternating member /
// score lines), skipping blanks and unparsable scores.
func parseScoredMembers(s string) []scored {
	lines := splitLines(s)
	var out []scored
	for i := 0; i+1 < len(lines); i += 2 {
		member := strings.TrimSpace(lines[i])
		if member == "" {
			i-- // resync on a stray blank line
			continue
		}
		score, err := strconv.ParseFloat(strings.TrimSpace(lines[i+1]), 64)
		if err != nil {
			continue
		}
		out = append(out, scored{member: member, score: int64(score)})
	}
	return out
}
