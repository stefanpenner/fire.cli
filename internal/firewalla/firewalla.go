// Package firewalla is the typed client layer. It turns the raw strings
// returned by the transport (redis-cli output, Zeek logs) into Go structs.
// All parsing lives in pure functions that are unit-tested against fixtures
// captured from a real box, so the risky code is the cheapest to test.
package firewalla

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/stefanpenner/fire.cli/internal/transport"
)

// deviceMarker delimits per-device hgetall blocks in the ListDevices stream.
const deviceMarker = "@@DEVICE@@"

// Device is a host known to the Firewalla, distilled from a host:mac:* hash.
type Device struct {
	MAC        string    `json:"mac"`
	Name       string    `json:"name"`
	IP         string    `json:"ip"`
	Vendor     string    `json:"vendor"`
	Type       string    `json:"type"`
	LastActive time.Time `json:"lastActive"`
	FirstSeen  time.Time `json:"firstSeen"`
}

// SeenWithin reports whether the device was active within window before now.
// A zero LastActive (never seen) always returns false.
func (d Device) SeenWithin(window time.Duration, now time.Time) bool {
	if d.LastActive.IsZero() {
		return false
	}
	return now.Sub(d.LastActive) <= window
}

// DNSQuery is one DNS lookup recorded in a flow:dns:<mac> zset.
type DNSQuery struct {
	Time    time.Time `json:"time"`
	Domain  string    `json:"domain"`
	Client  string    `json:"client"`
	Answers []string  `json:"answers"`
	Count   int       `json:"count"`
}

// Resolver is a client and how many times it resolved a given domain.
type Resolver struct {
	Client string `json:"client"`
	Count  int    `json:"count"`
}

// Client issues typed queries against a Firewalla box via a Transport.
type Client struct {
	t transport.Transport
}

// New returns a Client backed by the given transport.
func New(t transport.Transport) *Client { return &Client{t: t} }

// Host returns the underlying transport host.
func (c *Client) Host() string { return c.t.Host() }

// ListDevices returns every device the Firewalla knows about. It emits a
// single marker-delimited stream of hgetall blocks so the whole inventory
// comes back in one round trip.
func (c *Client) ListDevices(ctx context.Context) ([]Device, error) {
	const cmd = `for k in $(redis-cli --scan --pattern 'host:mac:*'); do ` +
		`echo "` + deviceMarker + ` $k"; redis-cli hgetall "$k"; done`
	res, err := c.t.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("listing devices on %s: %w", c.t.Host(), err)
	}
	return parseDevices(res.Stdout), nil
}

// DNSByDevice returns the most recent DNS lookups made by a device.
func (c *Client) DNSByDevice(ctx context.Context, mac string, limit int) ([]DNSQuery, error) {
	if limit <= 0 {
		limit = 100
	}
	cmd := fmt.Sprintf("redis-cli zrevrange flow:dns:%s 0 %d", strings.ToUpper(mac), limit-1)
	res, err := c.t.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("reading dns flows for %s: %w", mac, err)
	}
	return parseDNSFlows(res.Stdout)
}

// WhoResolved reports which clients resolved a domain today, using the
// Firewalla's per-3-minute Zeek dns logs. This is the "who queried this
// hostname" question.
func (c *Client) WhoResolved(ctx context.Context, domain string) ([]Resolver, error) {
	// grep -F: treat the domain literally (dots are not regex metachars here).
	// `|| true` swallows grep's exit-1-on-no-match so an empty result is not an
	// error; a genuine ssh/connection failure still propagates as a non-zero exit.
	cmd := fmt.Sprintf(`zcat /log/blog/$(date +%%F)/dns.*.log.gz 2>/dev/null | grep -iF %s || true`,
		shellQuote(domain))
	res, err := c.t.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("searching dns logs for %s: %w", domain, err)
	}
	return parseResolvers(res.Stdout, domain), nil
}

// Raw runs an arbitrary redis-cli invocation and returns stdout. It is the
// escape hatch for the long tail of the Firewalla surface not yet modelled.
func (c *Client) Raw(ctx context.Context, args string) (string, error) {
	res, err := c.t.Run(ctx, "redis-cli "+args)
	if err != nil {
		return res.Stdout, fmt.Errorf("redis-cli %s: %w", args, err)
	}
	return res.Stdout, nil
}

// ---- pure parsers (no I/O) ----

// parseRedisHash converts redis-cli hgetall output (alternating key/value
// lines) into a map. A trailing key with no value is ignored.
func parseRedisHash(s string) map[string]string {
	lines := splitLines(s)
	m := make(map[string]string, len(lines)/2)
	for i := 0; i+1 < len(lines); i += 2 {
		m[lines[i]] = lines[i+1]
	}
	return m
}

// parseDevices splits a marker-delimited stream of hgetall blocks into Devices.
func parseDevices(s string) []Device {
	var devices []Device
	blocks := strings.Split(s, deviceMarker)
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		// Drop the marker's trailing "host:mac:..." key line; the hash that
		// follows carries the canonical mac field anyway.
		if nl := strings.IndexByte(block, '\n'); nl >= 0 {
			block = block[nl+1:]
		} else {
			continue
		}
		devices = append(devices, deviceFromHash(parseRedisHash(block)))
	}
	return devices
}

// deviceFromHash maps a host:mac hash to a Device, applying name precedence
// and pulling the device type out of the embedded detect JSON.
func deviceFromHash(m map[string]string) Device {
	d := Device{
		MAC:        m["mac"],
		Name:       firstNonEmpty(m["name"], m["bname"], m["bonjourName"], m["dnsmasq.dhcp.leaseName"], m["dhcpName"]),
		IP:         firstNonEmpty(m["ipv4Addr"], m["ipv4"]),
		Vendor:     m["macVendor"],
		LastActive: parseUnixFloat(m["lastActiveTimestamp"]),
		FirstSeen:  parseUnixFloat(m["firstFoundTimestamp"]),
	}
	if detect := m["detect"]; detect != "" {
		var dd struct {
			Type string `json:"type"`
		}
		if json.Unmarshal([]byte(detect), &dd) == nil {
			d.Type = dd.Type
		}
	}
	return d
}

// parseDNSFlows decodes flow:dns zset members (one JSON object per line).
func parseDNSFlows(s string) ([]DNSQuery, error) {
	var out []DNSQuery
	for _, line := range splitLines(s) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var raw struct {
			Ts float64  `json:"ts"`
			DN string   `json:"dn"`
			SH string   `json:"sh"`
			AS []string `json:"as"`
			CT int      `json:"ct"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return nil, fmt.Errorf("parsing dns flow %q: %w", line, err)
		}
		out = append(out, DNSQuery{
			Time:    floatToTime(raw.Ts),
			Domain:  raw.DN,
			Client:  raw.SH,
			Answers: raw.AS,
			Count:   raw.CT,
		})
	}
	return out, nil
}

// parseResolvers aggregates Zeek dns.log JSON lines into per-client counts for
// queries matching domain, sorted by count desc then client for stability.
func parseResolvers(s, domain string) []Resolver {
	counts := map[string]int{}
	for _, line := range splitLines(s) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var raw struct {
			OrigH string `json:"id.orig_h"`
			Query string `json:"query"`
		}
		if json.Unmarshal([]byte(line), &raw) != nil {
			continue
		}
		if raw.Query == "" || !strings.Contains(raw.Query, domain) {
			continue
		}
		counts[raw.OrigH]++
	}
	out := make([]Resolver, 0, len(counts))
	for client, n := range counts {
		out = append(out, Resolver{Client: client, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Client < out[j].Client
	})
	return out
}

// ---- small helpers ----

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseUnixFloat(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return time.Time{}
	}
	return floatToTime(f)
}

func floatToTime(f float64) time.Time {
	if f == 0 {
		return time.Time{}
	}
	sec := int64(f)
	nsec := int64((f - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).UTC()
}

// shellQuote single-quotes a string for safe inclusion in a remote command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
