package firewalla

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAlarms(t *testing.T) {
	alarms := parseAlarms(fixture(t, "alarms.txt"))
	require.Len(t, alarms, 2)

	assert.Equal(t, "2297", alarms[0].ID)
	assert.Equal(t, "Port Scan", alarms[0].Type) // from p.noticeType Scan::Port_Scan
	assert.Equal(t, "Example Laptop", alarms[0].Device)
	assert.Contains(t, alarms[0].Message, "scanning ports")

	assert.Equal(t, "new device", alarms[1].Type) // from ALARM_NEW_DEVICE
	assert.Equal(t, "Example Phone", alarms[1].Device)
}

func TestParseFeatures(t *testing.T) {
	feats := parseFeatures(fixture(t, "features.txt"))
	// curated + present only; newDeviceTag is not curated and is skipped.
	byKey := map[string]bool{}
	for _, f := range feats {
		byKey[f.Key] = f.Enabled
	}
	assert.False(t, byKey["adblock"])
	assert.True(t, byKey["family"])
	assert.False(t, byKey["doh"])     // {"state":false}
	assert.True(t, byKey["vpn"])      // {"state":true}
	assert.False(t, byKey["qos"])     // object with no true bool
	assert.True(t, byKey["monitor"])  // plain "true"
	_, present := byKey["safeSearch"] // absent in fixture
	assert.False(t, present)
	_, present = byKey["newDeviceTag"] // not curated
	assert.False(t, present)
}

func TestFeatureEnabled(t *testing.T) {
	assert.True(t, featureEnabled("true"))
	assert.False(t, featureEnabled("false"))
	assert.True(t, featureEnabled(`{"state":true}`))
	assert.False(t, featureEnabled(`{"state":false}`))
	assert.True(t, featureEnabled(`{"upload":true}`)) // any-true fallback
	assert.False(t, featureEnabled(`{"upload":false}`))
	assert.False(t, featureEnabled(""))
}
