package firewalla

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// TopTalker is a device ranked by the bandwidth it used, aggregated from the
// box's most-recent internet `sumflow` rollups (device↔device "local" flows are
// excluded so this reflects internet usage).
type TopTalker struct {
	MAC      string `json:"mac"`
	Download int64  `json:"download"`
	Upload   int64  `json:"upload"`
}

// Bytes returns total bytes exchanged.
func (t TopTalker) Bytes() int64 { return t.Download + t.Upload }

// TopTalkers ranks every device by total bytes over its most-recent sumflow
// window, highest first. One round trip: the box sums each sumflow zset's
// scores server-side and emits "<key> <bytes>" lines we aggregate per MAC.
func (c *Client) TopTalkers(ctx context.Context) ([]TopTalker, error) {
	const script = `for k in $(redis-cli --scan --pattern 'sumflow:*'); do ` +
		`case "$k" in *:local:*) continue ;; *:download:*|*:upload:*) ` +
		`s=$(redis-cli zrange "$k" 0 -1 withscores | awk 'NR%2==0{sum+=$1} END{printf "%.0f", sum+0}'); ` +
		`echo "$k $s" ;; esac; done`
	res, err := c.t.Run(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("reading top talkers on %s: %w", c.t.Host(), err)
	}
	return parseTopTalkers(res.Stdout), nil
}

// sumflowKeyRE matches an internet sumflow key: sumflow:<MAC>:<dir>:<begin>:<end>.
var sumflowKeyRE = regexp.MustCompile(`^sumflow:([0-9A-Fa-f]{2}(?::[0-9A-Fa-f]{2}){5}):(download|upload):(\d+):(\d+)$`)

// parseTopTalkers aggregates "<sumflow-key> <bytes>" lines into per-device
// totals, keeping the most-recent window per direction (by the trailing :end
// timestamp), sorted by total bytes desc then MAC for stability.
func parseTopTalkers(s string) []TopTalker {
	type acc struct {
		down, up       int64
		downEnd, upEnd int64
	}
	m := map[string]*acc{}
	for _, line := range splitLines(s) {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		key := sumflowKeyRE.FindStringSubmatch(fields[0])
		if key == nil {
			continue
		}
		mac, dir := strings.ToUpper(key[1]), key[2]
		end, err := strconv.ParseInt(key[4], 10, 64)
		if err != nil {
			continue
		}
		bytes, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		a := m[mac]
		if a == nil {
			a = &acc{}
			m[mac] = a
		}
		if dir == "download" {
			if end >= a.downEnd {
				a.downEnd, a.down = end, int64(bytes)
			}
		} else {
			if end >= a.upEnd {
				a.upEnd, a.up = end, int64(bytes)
			}
		}
	}
	out := make([]TopTalker, 0, len(m))
	for mac, a := range m {
		out = append(out, TopTalker{MAC: mac, Download: a.down, Upload: a.up})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Bytes() != out[j].Bytes() {
			return out[i].Bytes() > out[j].Bytes()
		}
		return out[i].MAC < out[j].MAC
	})
	return out
}
