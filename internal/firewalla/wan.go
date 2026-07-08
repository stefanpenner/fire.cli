package firewalla

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// WAN section markers in the ListWANs stream.
const (
	wanRouting = "@@ROUTING@@"
	wanConn    = "@@CONN@@"
	wanNet     = "@@NET@@"
)

// WAN is one internet uplink: its identity, its role in the routing policy, and
// its live health. Built from FireRouter's active config + connectivity probe,
// joined to the network descriptors in sys:network:info.
type WAN struct {
	Name      string `json:"name"`      // friendly description (ISP name)
	Interface string `json:"interface"` // eth0, eth3, pppoe0
	UUID      string `json:"uuid"`
	Mode      string `json:"mode"`    // routing mode: single | primary_standby | load_balance
	Role      string `json:"role"`    // primary | standby | balanced
	Active    bool   `json:"active"`  // currently carrying traffic
	Carrier   bool   `json:"carrier"` // physical link up
	Ping      bool   `json:"ping"`    // ping health check passing
	DNS       bool   `json:"dns"`     // dns health check passing
	Ready     bool   `json:"ready"`
}

// fireRouterBase is the local FireRouter API. Overridable in tests is
// unnecessary because ListWANs is parsed from a captured stream.
const fireRouterBase = "http://localhost:8837/v1/config"

// ListWANs returns the box's internet uplinks with routing role and live health.
func (c *Client) ListWANs(ctx context.Context) ([]WAN, error) {
	script := fmt.Sprintf(
		`echo %s; curl -s %s/active; `+
			`echo; echo %s; curl -s %s/wan/connectivity; `+
			`echo; echo %s; redis-cli hgetall sys:network:info`,
		wanRouting, fireRouterBase, wanConn, fireRouterBase, wanNet)
	res, err := c.t.Run(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("reading WAN config on %s: %w", c.t.Host(), err)
	}
	return parseWANs(res.Stdout)
}

// routingDefault is FireRouter's routing.global.default.
type routingDefault struct {
	ViaIntf  string `json:"viaIntf"`
	ViaIntf2 string `json:"viaIntf2"`
	Type     string `json:"type"`
	Failback bool   `json:"failback"`
}

// connWAN is one entry of FireRouter's /wan/connectivity wans map.
type connWAN struct {
	Active    bool `json:"active"`
	Carrier   bool `json:"carrier"`
	Ping      bool `json:"ping"`
	DNS       bool `json:"dns"`
	ConnState struct {
		Ready  bool `json:"ready"`
		Active bool `json:"active"`
	} `json:"wanConnState"`
}

func parseWANs(s string) ([]WAN, error) {
	routingJSON := between(s, wanRouting, wanConn)
	connJSON := between(s, wanConn, wanNet)
	netStream := between(s, wanNet, "")

	var active struct {
		Routing struct {
			Global struct {
				Default routingDefault `json:"default"`
			} `json:"global"`
		} `json:"routing"`
	}
	_ = json.Unmarshal([]byte(strings.TrimSpace(routingJSON)), &active)
	def := active.Routing.Global.Default

	var conn struct {
		WANs map[string]connWAN `json:"wans"`
	}
	_ = json.Unmarshal([]byte(strings.TrimSpace(connJSON)), &conn)

	// Map interface → {desc, uuid} from sys:network:info, WAN entries only.
	type netMeta struct{ name, uuid string }
	meta := map[string]netMeta{}
	for intf, val := range parseRedisHash(strings.TrimSpace(netStream)) {
		val = strings.TrimSpace(val)
		if !strings.HasPrefix(val, "{") {
			continue
		}
		var raw rawNetwork
		if json.Unmarshal([]byte(val), &raw) != nil || raw.Type != "wan" {
			continue
		}
		meta[intf] = netMeta{name: firstNonEmpty(raw.Desc, raw.Name, intf), uuid: raw.UUID}
	}

	role := func(intf string) string {
		switch {
		case def.Type == "load_balance":
			return "balanced"
		case intf == def.ViaIntf:
			return "primary"
		case intf == def.ViaIntf2:
			return "standby"
		default:
			return ""
		}
	}

	var wans []WAN
	for intf, cw := range conn.WANs {
		m := meta[intf]
		wans = append(wans, WAN{
			Name:      firstNonEmpty(m.name, intf),
			Interface: intf,
			UUID:      m.uuid,
			Mode:      firstNonEmpty(def.Type, "single"),
			Role:      role(intf),
			Active:    cw.ConnState.Active, // the route currently in use
			Carrier:   cw.Carrier,
			Ping:      cw.Ping,
			DNS:       cw.DNS,
			Ready:     cw.ConnState.Ready,
		})
	}

	// primary, then standby, then others; stable by interface within a role.
	rank := map[string]int{"primary": 0, "standby": 1, "balanced": 0}
	sort.Slice(wans, func(i, j int) bool {
		ri, rj := rank[wans[i].Role], rank[wans[j].Role]
		if ri != rj {
			return ri < rj
		}
		return wans[i].Interface < wans[j].Interface
	})
	return wans, nil
}
