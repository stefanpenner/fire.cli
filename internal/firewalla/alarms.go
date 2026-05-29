package firewalla

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// alarmMarker delimits per-alarm hgetall blocks in the ListAlarms stream.
const alarmMarker = "@@ALARM@@"

// Alarm is one security/activity event the Firewalla raised: a port scan, a
// new device, abnormal upload, risky site, gaming/video activity, etc.
// Distilled from an `_alarm:<id>` hash.
type Alarm struct {
	ID      string    `json:"id"`
	Type    string    `json:"type"`    // friendly label (port scan, new device, …)
	Message string    `json:"message"` // human-readable description
	Device  string    `json:"device"`  // device name involved
	MAC     string    `json:"mac"`
	Time    time.Time `json:"time"`
	State   string    `json:"state"` // active | archived
}

// ListAlarms returns the most recent alarms, newest-first.
//
// Schema: alarm ids live in the `alarm_active` sorted set (by timestamp); each
// `_alarm:<id>` hash carries type, message, p.device.*, p.dest.*, state.
func (c *Client) ListAlarms(ctx context.Context, limit int) ([]Alarm, error) {
	if limit <= 0 {
		limit = 50
	}
	cmd := fmt.Sprintf(
		`for id in $(redis-cli zrevrange alarm_active 0 %d); do `+
			`echo "%s $id"; redis-cli hgetall "_alarm:$id"; done`, limit-1, alarmMarker)
	res, err := c.t.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("listing alarms on %s: %w", c.t.Host(), err)
	}
	return parseAlarms(res.Stdout), nil
}

func parseAlarms(s string) []Alarm {
	var alarms []Alarm
	for _, block := range strings.Split(s, alarmMarker) {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		nl := strings.IndexByte(block, '\n')
		if nl < 0 {
			continue
		}
		id := strings.TrimSpace(block[:nl])
		m := parseRedisHash(block[nl+1:])
		alarms = append(alarms, Alarm{
			ID:      firstNonEmpty(m["aid"], id),
			Type:    alarmType(m),
			Message: cleanMessage(firstNonEmpty(m["message"], m["p.message"])),
			Device:  firstNonEmpty(m["p.device.name"], m["device"]),
			MAC:     m["p.device.mac"],
			Time:    parseUnixFloat(firstNonEmpty(m["alarmTimestamp"], m["timestamp"])),
			State:   firstNonEmpty(m["state"], "active"),
		})
	}
	return alarms
}

// cleanMessage blanks "INFO_ALARM_*"-style constants, which Firewalla renders
// client-side rather than storing as prose; the type column conveys those.
func cleanMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	for _, r := range msg {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return msg // contains lowercase/spaces/punctuation → real prose
		}
	}
	return "" // all upper/underscore → a raw constant
}

// alarmType derives a short human label, preferring a specific notice subtype
// over the generic ALARM_* constant.
func alarmType(m map[string]string) string {
	if nt := m["p.noticeType"]; nt != "" {
		// e.g. "Scan::Port_Scan" → "Port Scan"
		if i := strings.LastIndex(nt, "::"); i >= 0 {
			nt = nt[i+2:]
		}
		return strings.ReplaceAll(nt, "_", " ")
	}
	t := strings.TrimPrefix(m["type"], "ALARM_")
	t = strings.ToLower(strings.ReplaceAll(t, "_", " "))
	return t
}
