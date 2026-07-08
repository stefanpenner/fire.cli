package firewalla

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Data-usage stream markers.
const (
	planMarker = "@@PLAN@@"
	wanUMarker = "@@WANU@@"
)

// WANUsage is one WAN's data consumed in the current plan period.
type WANUsage struct {
	UUID     string `json:"uuid"`
	Name     string `json:"name"` // resolved by the command via networks
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

// Bytes returns total bytes for the WAN.
func (u WANUsage) Bytes() int64 { return u.Upload + u.Download }

// DataUsageReport is the box's data-plan status: the plan limit, its monthly
// reset day, and per-WAN consumption this period.
type DataUsageReport struct {
	PlanTotal int64      `json:"planTotal"` // bytes; 0 = no plan set
	ResetDay  int        `json:"resetDay"`  // day of month the plan resets
	WANs      []WANUsage `json:"wans"`
}

// Total returns total bytes used across all WANs.
func (r DataUsageReport) Total() int64 {
	var n int64
	for _, w := range r.WANs {
		n += w.Bytes()
	}
	return n
}

// DataUsage returns data-plan status and per-WAN usage for the current period.
//
// Schema: sys:data:plan is JSON {total, date}. Per-WAN monthly usage lives in
// monthly:wan:data:usage:<uuid>:<periodStart> (JSON with daily download/upload
// arrays); the latest period is pointed to by …:<uuid>:lastTs.
func (c *Client) DataUsage(ctx context.Context) (DataUsageReport, error) {
	script := `echo ` + planMarker + `; redis-cli get sys:data:plan; ` +
		`for k in $(redis-cli --scan --pattern 'monthly:wan:data:usage:*:lastTs'); do ` +
		`u=${k#monthly:wan:data:usage:}; u=${u%:lastTs}; ts=$(redis-cli get "$k"); ` +
		`echo "` + wanUMarker + ` $u"; redis-cli get "monthly:wan:data:usage:$u:$ts"; done`
	res, err := c.t.Run(ctx, script)
	if err != nil {
		return DataUsageReport{}, fmt.Errorf("reading data usage on %s: %w", c.t.Host(), err)
	}
	return parseDataUsage(res.Stdout), nil
}

func parseDataUsage(s string) DataUsageReport {
	var report DataUsageReport

	// Plan section (between @@PLAN@@ and the first @@WANU@@).
	planJSON := strings.TrimSpace(between(s, planMarker, wanUMarker))
	var plan struct {
		Total int64 `json:"total"`
		Date  int   `json:"date"`
	}
	if json.Unmarshal([]byte(planJSON), &plan) == nil {
		report.PlanTotal = plan.Total
		report.ResetDay = plan.Date
	}

	// Each WAN section starts with "@@WANU@@ <uuid>".
	parts := strings.Split(s, wanUMarker)
	for _, block := range parts[1:] {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		nl := strings.IndexByte(block, '\n')
		uuid, body := block, ""
		if nl >= 0 {
			uuid, body = strings.TrimSpace(block[:nl]), block[nl+1:]
		}
		var usage struct {
			Stats struct {
				Download [][]float64 `json:"download"`
				Upload   [][]float64 `json:"upload"`
			} `json:"stats"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(body)), &usage) != nil {
			continue
		}
		report.WANs = append(report.WANs, WANUsage{
			UUID:     uuid,
			Download: sumSeries(usage.Stats.Download),
			Upload:   sumSeries(usage.Stats.Upload),
		})
	}
	return report
}

// sumSeries sums the value (index 1) of each [timestamp, value] pair.
func sumSeries(series [][]float64) int64 {
	var total int64
	for _, point := range series {
		if len(point) >= 2 {
			total += int64(point[1])
		}
	}
	return total
}
