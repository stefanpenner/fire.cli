package firewalla

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Network is a single L2/L3 segment the Firewalla routes: a physical LAN/WAN,
// a VLAN, or a bridge. Distilled from the FireRouter `sys:network:info` hash,
// where each field is an interface name (eth0, br0, eth2.2001) and the value
// is a JSON descriptor.
type Network struct {
	Name      string   `json:"name"`      // friendly description (e.g. "IoT", ISP name for WANs)
	Interface string   `json:"interface"` // eth0, br0, eth2.2001
	Parent    string   `json:"parent"`    // underlying interface for a VLAN (eth2)
	VLANID    int      `json:"vlanId"`    // 0 when not a VLAN
	Type      string   `json:"type"`      // wan | lan
	ConnType  string   `json:"connType"`  // Wired | WiFi | …
	Subnet    string   `json:"subnet"`    // primary IPv4 CIDR
	Gateway   string   `json:"gateway"`
	DNS       []string `json:"dns"`
	UUID      string   `json:"uuid"`
}

// ListNetworks returns the configured networks, including VLANs.
func (c *Client) ListNetworks(ctx context.Context) ([]Network, error) {
	res, err := c.t.Run(ctx, "redis-cli hgetall sys:network:info")
	if err != nil {
		return nil, fmt.Errorf("reading network info on %s: %w", c.t.Host(), err)
	}
	return parseNetworks(res.Stdout), nil
}

// rawNetwork mirrors the subset of a sys:network:info JSON descriptor we surface.
type rawNetwork struct {
	Name      string   `json:"name"`
	Desc      string   `json:"desc"`
	UUID      string   `json:"uuid"`
	Subnet    string   `json:"subnet"`
	Gateway   string   `json:"gateway"`
	GatewayIP string   `json:"gateway_ip"`
	DNS       []string `json:"dns"`
	ConnType  string   `json:"conn_type"`
	Type      string   `json:"type"`
}

// parseNetworks turns the sys:network:info hash into a sorted slice of
// Networks. Non-interface fields (publicIp, ddns, …) and any value that isn't
// a network descriptor are skipped.
func parseNetworks(s string) []Network {
	hash := parseRedisHash(s)
	var nets []Network
	for intf, val := range hash {
		val = strings.TrimSpace(val)
		if !strings.HasPrefix(val, "{") {
			continue // publicIp/ddns/token fields are plain strings
		}
		var raw rawNetwork
		if json.Unmarshal([]byte(val), &raw) != nil {
			continue
		}
		if raw.Type != "wan" && raw.Type != "lan" {
			continue // only routed segments; skip tunnels/AP-only/etc.
		}
		parent, vlan := splitVLAN(intf)
		nets = append(nets, Network{
			Name:      firstNonEmpty(raw.Desc, raw.Name, intf),
			Interface: intf,
			Parent:    parent,
			VLANID:    vlan,
			Type:      raw.Type,
			ConnType:  raw.ConnType,
			Subnet:    raw.Subnet,
			Gateway:   firstNonEmpty(raw.Gateway, raw.GatewayIP),
			DNS:       raw.DNS,
			UUID:      raw.UUID,
		})
	}

	// Stable order: WANs first, then by VLAN id, then interface name.
	sort.Slice(nets, func(i, j int) bool {
		if nets[i].Type != nets[j].Type {
			return nets[i].Type == "wan" // wan before lan
		}
		if nets[i].VLANID != nets[j].VLANID {
			return nets[i].VLANID < nets[j].VLANID
		}
		return nets[i].Interface < nets[j].Interface
	})
	return nets
}

// splitVLAN parses an interface name like "eth2.2001" into its parent ("eth2")
// and VLAN id (2001). For a plain interface it returns ("", 0).
func splitVLAN(intf string) (parent string, vlan int) {
	dot := strings.LastIndexByte(intf, '.')
	if dot < 0 {
		return "", 0
	}
	id, err := strconv.Atoi(intf[dot+1:])
	if err != nil {
		return "", 0
	}
	return intf[:dot], id
}
