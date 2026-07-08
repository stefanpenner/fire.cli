package firewalla

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Feature is a box-wide capability toggle (ad block, family mode, VPN server,
// …) read from the `policy:system` hash.
type Feature struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// curatedFeatures is the ordered set of policy:system toggles we surface, with
// friendly names. Other keys in the hash are internal/transient.
var curatedFeatures = []struct{ key, name string }{
	{"adblock", "Ad Block"},
	{"family", "Family Mode"},
	{"safeSearch", "Safe Search"},
	{"doh", "DNS over HTTPS"},
	{"vpn", "VPN Server"},
	{"vpnClient", "VPN Client"},
	{"qos", "Smart Queue (QoS)"},
	{"monitor", "Monitoring"},
	{"ntp_redirect", "NTP Redirect"},
	{"externalAccess", "External Access"},
}

// ListFeatures returns the curated box features and whether each is enabled.
func (c *Client) ListFeatures(ctx context.Context) ([]Feature, error) {
	res, err := c.t.Run(ctx, "redis-cli hgetall policy:system")
	if err != nil {
		return nil, fmt.Errorf("reading features on %s: %w", c.t.Host(), err)
	}
	return parseFeatures(res.Stdout), nil
}

func parseFeatures(s string) []Feature {
	hash := parseRedisHash(s)
	var out []Feature
	for _, f := range curatedFeatures {
		raw, ok := hash[f.key]
		if !ok {
			continue
		}
		out = append(out, Feature{Key: f.key, Name: f.name, Enabled: featureEnabled(raw)})
	}
	return out
}

// featureEnabled interprets a policy:system value, which may be a plain bool
// ("true"/"false") or a JSON object carrying a "state" flag (and sometimes
// other booleans, e.g. qos.upload). Returns true if the feature is on.
func featureEnabled(raw string) bool {
	raw = strings.TrimSpace(raw)
	switch strings.ToLower(raw) {
	case "true", "1":
		return true
	case "false", "0", "":
		return false
	}
	if !strings.HasPrefix(raw, "{") {
		return false
	}
	// Prefer an explicit "state"; otherwise treat any true boolean as enabled.
	var withState struct {
		State *bool `json:"state"`
	}
	if json.Unmarshal([]byte(raw), &withState) == nil && withState.State != nil {
		return *withState.State
	}
	var generic map[string]any
	if json.Unmarshal([]byte(raw), &generic) == nil {
		return anyTrue(generic)
	}
	return false
}

// anyTrue reports whether any boolean in the (possibly nested) value is true.
func anyTrue(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case map[string]any:
		for _, child := range t {
			if anyTrue(child) {
				return true
			}
		}
	}
	return false
}
