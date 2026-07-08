package firewalla

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ruleMarker delimits per-rule hgetall blocks in the ListRules stream.
const ruleMarker = "@@RULE@@"

// Rule is a single firewall policy: block/allow (and friends) applied to a
// target (domain, ip, category, mac, …) in a direction. Distilled from a
// `policy:<id>` hash.
type Rule struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`    // block | allow | qos | route
	Target    string    `json:"target"`    // the thing the rule matches
	Type      string    `json:"type"`      // dns | ip | category | mac | net | …
	Direction string    `json:"direction"` // bidirection | inbound | outbound
	Scope     string    `json:"scope"`     // device/group/network the rule applies to
	Notes     string    `json:"notes"`
	Disabled  bool      `json:"disabled"`
	HitCount  int64     `json:"hitCount"` // times the rule has matched
	LastHit   time.Time `json:"lastHit"`
	Created   time.Time `json:"created"`
}

// ListRules returns every firewall rule known to the box.
//
// Schema assumption: rules live in `policy:<numeric-id>` hashes (the
// `policy:*` keyspace also holds non-rule keys like `policy:network:*`, which
// the numeric-suffix filter drops). Fields: action, target, type, direction,
// scope, notes, disabled, ts. Capture a real (anonymized) sample with
// scripts/capture-fixtures.sh to verify field names on your firmware.
func (c *Client) ListRules(ctx context.Context) ([]Rule, error) {
	// Stream all rule hashes in one round trip, marker-delimited. The case glob
	// keeps only policy:<digit…> keys, skipping policy:network:* and similar.
	const cmd = `for k in $(redis-cli --scan --pattern 'policy:*'); do ` +
		`case "$k" in policy:[0-9]*) ` +
		`echo "` + ruleMarker + ` $k"; redis-cli hgetall "$k";; esac; done`
	res, err := c.t.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("listing rules on %s: %w", c.t.Host(), err)
	}
	return parseRules(res.Stdout), nil
}

// parseRules splits a marker-delimited stream of hgetall blocks into Rules,
// sorted newest-first.
func parseRules(s string) []Rule {
	var rules []Rule
	for _, block := range strings.Split(s, ruleMarker) {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		// First line is the marker's "policy:<id>" key; the hash follows.
		nl := strings.IndexByte(block, '\n')
		if nl < 0 {
			continue
		}
		id := strings.TrimPrefix(strings.TrimSpace(block[:nl]), "policy:")
		rules = append(rules, ruleFromHash(id, parseRedisHash(block[nl+1:])))
	}
	sort.SliceStable(rules, func(i, j int) bool {
		return rules[i].Created.After(rules[j].Created)
	})
	return rules
}

func ruleFromHash(id string, m map[string]string) Rule {
	if pid := m["pid"]; pid != "" {
		id = pid
	}
	hits, _ := strconv.ParseInt(strings.TrimSpace(m["hitCount"]), 10, 64)
	return Rule{
		ID:        id,
		Action:    firstNonEmpty(m["action"], "block"), // Firewalla omits action on plain blocks
		Target:    m["target"],
		Type:      m["type"],
		Direction: firstNonEmpty(m["direction"], "bidirection"),
		Scope:     scopeFromTag(firstNonEmpty(m["scope"], m["tag"])),
		Notes:     m["notes"],
		Disabled:  isTruthy(m["disabled"]),
		HitCount:  hits,
		LastHit:   parseUnixFloat(m["lastHitTs"]),
		Created:   parseUnixFloat(firstNonEmpty(m["timestamp"], m["ts"])),
	}
}

// scopeFromTag tidies the scope/tag field, which is often a JSON array like
// ["intf:<uuid>"] or ["mac:AA:..."], into a compact human label.
func scopeFromTag(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") {
		return s
	}
	var arr []string
	if json.Unmarshal([]byte(s), &arr) != nil || len(arr) == 0 {
		return s
	}
	return strings.Join(arr, ",")
}

// isTruthy treats "1"/"true" (any case) as true; everything else false.
func isTruthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true":
		return true
	default:
		return false
	}
}
